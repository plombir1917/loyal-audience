// Package service — оркестрация пайплайна: сбор данных из ВК, классификация
// комментариев и сегментация аудитории (Loyal / Neutral / Disloyal).
package service

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/plombir1917/vk-loyal-users-parser/internal/classifier"
	"github.com/plombir1917/vk-loyal-users-parser/internal/model"
	"github.com/plombir1917/vk-loyal-users-parser/internal/vk"
)

// Config — параметры одного прогона сбора.
type Config struct {
	RegionName         string
	SearchKeywords     []string
	CollectSince       time.Time
	MaxPostsPerGroup   int
	MaxCommentsPerPost int
	MaxCommunities     int
	// ClassifyConcurrency — число параллельных обращений к классификатору.
	ClassifyConcurrency int
	// Ядро аудитории: пороги и режим объединения (OR вместо AND).
	LikeThreshold    int
	CommentThreshold int
	CoreCombineOr    bool
	// SkipExistingCommunities — пропускать сообщество целиком, если оно уже в БД.
	SkipExistingCommunities bool
	// ReparseExisting — повторно обрабатывать уже сохранённые сущности,
	// игнорируя пропуски (перекрывает SkipExistingCommunities).
	ReparseExisting bool
}

// Service связывает VK-клиент, классификатор и хранилище.
type Service struct {
	vk         *vk.Client
	classifier classifier.Classifier
	store      Store
	cfg        Config
	log        *slog.Logger

	seenUsers map[string]struct{} // дедуп upsert'ов пользователей в рамках прогона
}

// Store — необходимый сервису контракт слоя хранения.
type Store interface {
	UpsertCommunity(ctx context.Context, c model.Community) error
	AllCommunityIDs(ctx context.Context) (map[string]struct{}, error)
	UpsertPost(ctx context.Context, p model.Post) error
	UpsertUser(ctx context.Context, u model.User) error
	UpsertLike(ctx context.Context, l model.Like) error
	UpsertComment(ctx context.Context, c model.Comment) error
	UpsertReaction(ctx context.Context, r model.Reaction) error
	UpdatePostSentiment(ctx context.Context, postID string, sentiment model.Sentiment) error
	ExistingCommentIDs(ctx context.Context, postID string) (map[string]struct{}, error)
	ExistingLikeUserIDs(ctx context.Context, postID string) (map[string]struct{}, error)
	ExistingPostSentiments(ctx context.Context, groupID string) (map[string]struct{}, error)
	RecomputeUsers(ctx context.Context, likeThr, commentThr int, combineOr bool) (int64, error)
	RecomputeStats(ctx context.Context, likeThr int) error
}

// New создаёт сервис.
func New(vkClient *vk.Client, cls classifier.Classifier, store Store, cfg Config, log *slog.Logger) *Service {
	return &Service{
		vk:         vkClient,
		classifier: cls,
		store:      store,
		cfg:        cfg,
		log:        log,
		seenUsers:  make(map[string]struct{}),
	}
}

// Run выполняет полный пайплайн сбора и сегментации.
func (s *Service) Run(ctx context.Context) error {
	cities, err := s.vk.ResolveRegionCities(ctx, s.cfg.RegionName)
	if err != nil {
		return err
	}
	s.log.Info("регион разрешён", "region", s.cfg.RegionName, "cities", len(cities))

	// При SKIP_EXISTING_COMMUNITIES заранее загружаем id всех обработанных
	// сообществ, чтобы отсеивать их на этапе поиска (до groups.getById) и внутри
	// прогона не заходить в них повторно. Ключевое при этом флаге — пройтись
	// только по новым, ещё не обработанным группам и их данным.
	skipExisting := s.cfg.SkipExistingCommunities && !s.cfg.ReparseExisting
	var known map[string]struct{}
	if skipExisting {
		known, err = s.store.AllCommunityIDs(ctx)
		if err != nil {
			return err
		}
		s.log.Info("пропуск существующих включён", "known_communities", len(known))
	}

	collected := 0
	for _, city := range cities {
		if s.cfg.MaxCommunities != 0 && collected >= s.cfg.MaxCommunities {
			break
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		// known передаётся как skip-множество: SearchCommunities не обогащает и не
		// возвращает уже обработанные группы (при выключенном флаге known == nil).
		communities, err := s.vk.SearchCommunities(ctx, city, s.cfg.RegionName, s.cfg.SearchKeywords, known)
		if err != nil {
			s.log.Warn("поиск сообществ не удался", "city", city.Title, "err", err)
			continue
		}
		for _, comm := range communities {
			if s.cfg.MaxCommunities != 0 && collected >= s.cfg.MaxCommunities {
				break
			}
			if err := s.processCommunity(ctx, comm); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				s.log.Warn("сообщество пропущено", "group_id", comm.GroupID, "err", err)
				continue
			}
			collected++
			// Помечаем обработанным, чтобы в других городах его снова не забирать.
			if skipExisting {
				known[comm.GroupID] = struct{}{}
			}
		}
	}

	updated, err := s.store.RecomputeUsers(ctx, s.cfg.LikeThreshold, s.cfg.CommentThreshold, s.cfg.CoreCombineOr)
	if err != nil {
		return err
	}
	s.log.Info("сегментация завершена", "communities", collected, "users_segmented", updated)

	if err := s.store.RecomputeStats(ctx, s.cfg.LikeThreshold); err != nil {
		return err
	}
	s.log.Info("статистика пересчитана",
		"like_threshold", s.cfg.LikeThreshold,
		"comment_threshold", s.cfg.CommentThreshold,
		"core_combine_or", s.cfg.CoreCombineOr)
	return nil
}

func (s *Service) processCommunity(ctx context.Context, comm model.Community) error {
	if err := s.store.UpsertCommunity(ctx, comm); err != nil {
		return err
	}
	posts, err := s.vk.GetPosts(ctx, comm, s.cfg.CollectSince, s.cfg.MaxPostsPerGroup)
	if err != nil {
		return err
	}
	s.log.Info("сообщество обработано", "group_id", comm.GroupID, "name", comm.Name, "posts", len(posts))

	for _, p := range posts {
		if err := s.store.UpsertPost(ctx, p); err != nil {
			return err
		}
		if err := s.processReactions(ctx, p); err != nil {
			return err
		}
		if err := s.processLikes(ctx, p); err != nil {
			return err
		}
		if err := s.processComments(ctx, p); err != nil {
			return err
		}
	}
	return s.classifyPosts(ctx, comm.GroupID, posts)
}

// existingSet возвращает уже сохранённое множество для пропуска — или пустое,
// если включён повторный парсинг (ReparseExisting), тогда всё переобрабатывается
// заново. Инкапсулирует единое правило skip для комментариев/лайков/постов.
func (s *Service) existingSet(ctx context.Context, reparse bool, load func(context.Context) (map[string]struct{}, error)) (map[string]struct{}, error) {
	if reparse {
		return map[string]struct{}{}, nil
	}
	return load(ctx)
}

// classifyPosts проставляет тональность содержания постов (задача 3). Уже
// классифицированные пропускаются, новые классифицируются конкурентно. При
// ReparseExisting классифицируются заново все посты.
func (s *Service) classifyPosts(ctx context.Context, groupID string, posts []model.Post) error {
	done, err := s.existingSet(ctx, s.cfg.ReparseExisting, func(ctx context.Context) (map[string]struct{}, error) {
		return s.store.ExistingPostSentiments(ctx, groupID)
	})
	if err != nil {
		return err
	}
	fresh := make([]model.Post, 0, len(posts))
	for _, p := range posts {
		if _, ok := done[p.PostID]; !ok {
			fresh = append(fresh, p)
		}
	}
	if len(fresh) == 0 {
		return nil
	}
	texts := make([]string, len(fresh))
	for i, p := range fresh {
		texts[i] = p.Text
	}
	sentiments := s.classifyTexts(ctx, texts)
	for i, p := range fresh {
		if err := s.store.UpdatePostSentiment(ctx, p.PostID, sentiments[i]); err != nil {
			return err
		}
	}
	return nil
}

// processReactions сохраняет агрегат реакций поста (приходит инлайн из wall.get).
func (s *Service) processReactions(ctx context.Context, p model.Post) error {
	for _, r := range p.Reactions {
		if err := s.store.UpsertReaction(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) processLikes(ctx context.Context, p model.Post) error {
	seen, err := s.existingSet(ctx, s.cfg.ReparseExisting, func(ctx context.Context) (map[string]struct{}, error) {
		return s.store.ExistingLikeUserIDs(ctx, p.PostID)
	})
	if err != nil {
		return err
	}
	likers, err := s.vk.GetLikers(ctx, p.OwnerID, p.VKID)
	if err != nil {
		return err
	}
	for _, vkID := range likers {
		u := vk.NewUser(vkID)
		if _, ok := seen[u.UserID]; ok {
			continue // лайк уже сохранён в прошлом прогоне
		}
		if err := s.ensureUser(ctx, u); err != nil {
			return err
		}
		like := model.Like{LikeID: p.PostID + "_" + u.UserID, PostID: p.PostID, UserID: u.UserID}
		if err := s.store.UpsertLike(ctx, like); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) processComments(ctx context.Context, p model.Post) error {
	classified, err := s.existingSet(ctx, s.cfg.ReparseExisting, func(ctx context.Context) (map[string]struct{}, error) {
		return s.store.ExistingCommentIDs(ctx, p.PostID)
	})
	if err != nil {
		return err
	}
	comments, err := s.vk.GetComments(ctx, p.OwnerID, p.VKID, p.PostID, s.cfg.MaxCommentsPerPost)
	if err != nil {
		return err
	}

	// Оставляем только новые комментарии — остальные уже классифицированы.
	fresh := make([]model.Comment, 0, len(comments))
	for _, cm := range comments {
		if _, ok := classified[cm.CommentID]; !ok {
			fresh = append(fresh, cm)
		}
	}
	if len(fresh) == 0 {
		return nil
	}

	// Классификация — самая медленная часть (сетевой вызов на комментарий),
	// поэтому считаем тональность конкурентно. Запись в БД и дедуп пользователей
	// остаются последовательными.
	texts := make([]string, len(fresh))
	for i, cm := range fresh {
		texts[i] = cm.Text
	}
	sentiments := s.classifyTexts(ctx, texts)
	for i, cm := range fresh {
		if err := s.ensureUser(ctx, vk.NewUser(atoiSafe(cm.UserID))); err != nil {
			return err
		}
		sentiment := sentiments[i]
		cm.Sentiment = &sentiment
		if err := s.store.UpsertComment(ctx, cm); err != nil {
			return err
		}
	}
	return nil
}

// classifyTexts классифицирует тексты конкурентно через пул воркеров
// (cfg.ClassifyConcurrency, минимум 1). Каждая горутина пишет в свой индекс,
// поэтому гонок нет; ошибка классификации не валит пайплайн — текст помечается
// neutral.
func (s *Service) classifyTexts(ctx context.Context, texts []string) []model.Sentiment {
	out := make([]model.Sentiment, len(texts))
	workers := max(s.cfg.ClassifyConcurrency, 1)

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i := range texts {
		wg.Add(1)
		sem <- struct{}{} // блокируется, когда в работе уже workers горутин
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			sentiment, err := s.classifier.Classify(ctx, texts[i])
			if err != nil {
				s.log.Warn("классификация не удалась", "err", err)
				sentiment = model.Neutral
			}
			out[i] = sentiment
		}(i)
	}
	wg.Wait()
	return out
}

// ensureUser сохраняет пользователя один раз за прогон.
func (s *Service) ensureUser(ctx context.Context, u model.User) error {
	if _, ok := s.seenUsers[u.UserID]; ok {
		return nil
	}
	if err := s.store.UpsertUser(ctx, u); err != nil {
		return err
	}
	s.seenUsers[u.UserID] = struct{}{}
	return nil
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
