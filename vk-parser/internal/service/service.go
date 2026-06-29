// Package service — оркестрация пайплайна: сбор данных из ВК, классификация
// комментариев и сегментация аудитории (Loyal / Neutral / Disloyal).
package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/plombir1917/vk-loyal-users-parser/internal/classifier"
	"github.com/plombir1917/vk-loyal-users-parser/internal/model"
	"github.com/plombir1917/vk-loyal-users-parser/internal/vk"
)

// Config — параметры одного прогона сбора.
type Config struct {
	RegionName         string
	CollectSince       time.Time
	MaxPostsPerGroup   int
	MaxCommentsPerPost int
	MaxCommunities     int
	// SkipExistingCommunities — пропускать сообщество целиком, если оно уже в БД.
	SkipExistingCommunities bool
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
	CommunityExists(ctx context.Context, groupID string) (bool, error)
	UpsertPost(ctx context.Context, p model.Post) error
	UpsertUser(ctx context.Context, u model.User) error
	UpsertLike(ctx context.Context, l model.Like) error
	UpsertComment(ctx context.Context, c model.Comment) error
	ExistingCommentIDs(ctx context.Context, postID string) (map[string]struct{}, error)
	ExistingLikeUserIDs(ctx context.Context, postID string) (map[string]struct{}, error)
	SegmentUsers(ctx context.Context) (int64, error)
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

	collected := 0
	for _, city := range cities {
		if s.cfg.MaxCommunities != 0 && collected >= s.cfg.MaxCommunities {
			break
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		communities, err := s.vk.SearchCommunities(ctx, city, s.cfg.RegionName)
		if err != nil {
			s.log.Warn("поиск сообществ не удался", "city", city.Title, "err", err)
			continue
		}
		for _, comm := range communities {
			if s.cfg.MaxCommunities != 0 && collected >= s.cfg.MaxCommunities {
				break
			}
			if s.cfg.SkipExistingCommunities {
				exists, err := s.store.CommunityExists(ctx, comm.GroupID)
				if err != nil {
					return err
				}
				if exists {
					s.log.Info("сообщество пропущено (уже в БД)", "group_id", comm.GroupID)
					continue
				}
			}
			if err := s.processCommunity(ctx, comm); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				s.log.Warn("сообщество пропущено", "group_id", comm.GroupID, "err", err)
				continue
			}
			collected++
		}
	}

	updated, err := s.store.SegmentUsers(ctx)
	if err != nil {
		return err
	}
	s.log.Info("сегментация завершена", "communities", collected, "users_segmented", updated)
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
		if err := s.processLikes(ctx, p); err != nil {
			return err
		}
		if err := s.processComments(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) processLikes(ctx context.Context, p model.Post) error {
	seen, err := s.store.ExistingLikeUserIDs(ctx, p.PostID)
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
	classified, err := s.store.ExistingCommentIDs(ctx, p.PostID)
	if err != nil {
		return err
	}
	comments, err := s.vk.GetComments(ctx, p.OwnerID, p.VKID, p.PostID, s.cfg.MaxCommentsPerPost)
	if err != nil {
		return err
	}
	for _, cm := range comments {
		if _, ok := classified[cm.CommentID]; ok {
			continue // комментарий уже сохранён и классифицирован — не дёргаем LLM
		}
		if err := s.ensureUser(ctx, vk.NewUser(atoiSafe(cm.UserID))); err != nil {
			return err
		}
		sentiment, err := s.classifier.Classify(ctx, cm.Text)
		if err != nil {
			s.log.Warn("классификация не удалась", "comment_id", cm.CommentID, "err", err)
			sentiment = model.Neutral
		}
		cm.Sentiment = &sentiment
		if err := s.store.UpsertComment(ctx, cm); err != nil {
			return err
		}
	}
	return nil
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
