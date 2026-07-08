// Package vk — клиент API ВКонтакте: сбор сообществ, постов, лайков и
// комментариев. Обёртка над github.com/SevereCloud/vksdk с ограничением частоты
// запросов и маппингом ответов в доменные модели (internal/model).
package vk

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/object"
	"golang.org/x/time/rate"

	"github.com/plombir1917/vk-loyal-users-parser/internal/model"
)

const (
	vkPageSize     = 100  // максимум для wall.get / wall.getComments
	likesPageSize  = 1000 // максимум для likes.getList
	getByIDBatch   = 200  // сколько групп запрашиваем за один groups.getById
	searchPageSize = 1000 // максимум для groups.search
	// apiVersion — версия VK API. По умолчанию SDK шлёт 5.131, где wall.get не
	// возвращает поле reactions; для сбора реакций нужна версия поновее.
	apiVersion = "5.199"
)

// City — населённый пункт региона.
type City struct {
	ID    int
	Title string
}

// Client — клиент VK с лимитером частоты запросов.
type Client struct {
	api *api.VK
	lim *rate.Limiter
	log *slog.Logger
}

// New создаёт клиент. ratePerSec — допустимое число запросов в секунду.
func New(token string, ratePerSec int, log *slog.Logger) *Client {
	if ratePerSec <= 0 {
		ratePerSec = 3
	}
	vkAPI := api.NewVK(token)
	vkAPI.Version = apiVersion // иначе wall.get не отдаёт reactions
	return &Client{
		api: vkAPI,
		lim: rate.NewLimiter(rate.Limit(ratePerSec), 1),
		log: log,
	}
}

// call дожидается разрешения лимитера и прикрепляет контекст к параметрам.
func (c *Client) wait(ctx context.Context) error {
	return c.lim.Wait(ctx)
}

// ResolveRegionCities находит регион по названию и возвращает его города.
func (c *Client) ResolveRegionCities(ctx context.Context, regionName string) ([]City, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	regions, err := c.api.DatabaseGetRegions(api.Params{
		"country_id": 1,
		"q":          regionName,
		"count":      100,
	}.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("database.getRegions: %w", err)
	}

	regionID := 0
	for _, r := range regions.Items {
		if strings.Contains(strings.ToLower(r.Title), strings.ToLower(regionName)) {
			regionID = r.ID
			break
		}
	}
	if regionID == 0 && len(regions.Items) > 0 {
		regionID = regions.Items[0].ID // лучший доступный кандидат
	}
	if regionID == 0 {
		return nil, fmt.Errorf("регион %q не найден", regionName)
	}

	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	cities, err := c.api.DatabaseGetCities(api.Params{
		"country_id": 1,
		"region_id":  regionID,
		"need_all":   1,
		"count":      1000,
	}.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("database.getCities: %w", err)
	}

	out := make([]City, 0, len(cities.Items))
	for _, ci := range cities.Items {
		out = append(out, City{ID: ci.ID, Title: ci.Title})
	}
	return out, nil
}

// SearchCommunities ищет открытые сообщества, привязанные к городу, и обогащает
// их данными (описание, число подписчиков) через groups.getById.
func (c *Client) SearchCommunities(ctx context.Context, city City, regionName string, keywords []string) ([]model.Community, error) {
	// groups.search отдаёт максимум ~1000 групп на один q и матчит его по тексту.
	// Чтобы выбрать больше, гоняем несколько запросов на город (имя города +
	// темы-затравки) и дедуплицируем открытые сообщества по id. city_id при этом
	// удерживает выдачу в пределах города.
	queries := make([]string, 0, len(keywords)+1)
	queries = append(queries, city.Title)
	queries = append(queries, keywords...)

	seen := make(map[int]struct{})
	ids := make([]string, 0)
	var firstErr error
	for _, q := range queries {
		if err := c.wait(ctx); err != nil {
			return nil, err
		}
		resp, err := c.api.GroupsSearch(api.Params{
			"q":       q,
			"city_id": city.ID,
			"count":   searchPageSize,
			"sort":    0,
		}.WithContext(ctx))
		if err != nil {
			// Одна неудачная затравка не должна ронять весь город — запоминаем
			// первую ошибку и продолжаем перебор.
			if firstErr == nil {
				firstErr = fmt.Errorf("groups.search city=%d q=%q: %w", city.ID, q, err)
			}
			continue
		}
		for _, g := range resp.Items {
			if g.IsClosed != 0 { // только открытые сообщества
				continue
			}
			if _, ok := seen[g.ID]; ok {
				continue
			}
			seen[g.ID] = struct{}{}
			ids = append(ids, strconv.Itoa(g.ID))
		}
	}
	if len(ids) == 0 {
		return nil, firstErr
	}

	var communities []model.Community
	for start := 0; start < len(ids); start += getByIDBatch {
		end := min(start+getByIDBatch, len(ids))
		if err := c.wait(ctx); err != nil {
			return nil, err
		}
		// На версии API 5.199 (нужна для reactions) groups.getById возвращает
		// объект {groups, profiles}, а типизированный метод SDK ждёт массив —
		// поэтому разбираем ответ сами.
		var details groupsGetByIDResponse
		err := c.api.RequestUnmarshal("groups.getById", &details, api.Params{
			"group_ids": strings.Join(ids[start:end], ","),
			"fields":    "description,members_count,city",
		}.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("groups.getById: %w", err)
		}
		for _, g := range details.Groups {
			if g.IsClosed != 0 {
				continue
			}
			communities = append(communities, toCommunity(g, regionName, city.Title))
		}
	}
	return communities, nil
}

// wallGetResponse — часть ответа wall.get, которую разбираем сами: SDK не
// типизирует поле reactions, а оно нужно для тональности реакций.
type wallGetResponse struct {
	Count int        `json:"count"`
	Items []wallPost `json:"items"`
}

type wallPost struct {
	ID        int            `json:"id"`
	Date      int            `json:"date"`
	Text      string         `json:"text"`
	Reactions *wallReactions `json:"reactions"`
}

type wallReactions struct {
	Count int `json:"count"`
	Items []struct {
		ID    int `json:"id"`
		Count int `json:"count"`
	} `json:"items"`
}

// GetPosts возвращает публикации сообщества не старше since (по убыванию даты)
// вместе с агрегатом реакций (поле Reactions приходит инлайн в wall.get).
func (c *Client) GetPosts(ctx context.Context, comm model.Community, since time.Time, max int) ([]model.Post, error) {
	groupID, err := strconv.Atoi(comm.GroupID)
	if err != nil {
		return nil, fmt.Errorf("некорректный group_id %q: %w", comm.GroupID, err)
	}
	ownerID := -groupID

	var posts []model.Post
	for offset := 0; max == 0 || len(posts) < max; offset += vkPageSize {
		if err := c.wait(ctx); err != nil {
			return nil, err
		}
		var resp wallGetResponse
		err := c.api.RequestUnmarshal("wall.get", &resp, api.Params{
			"owner_id": ownerID,
			"count":    vkPageSize,
			"offset":   offset,
		}.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("wall.get owner=%d: %w", ownerID, err)
		}
		if len(resp.Items) == 0 {
			break
		}

		stop := false
		for _, p := range resp.Items {
			date := time.Unix(int64(p.Date), 0).UTC()
			if date.Before(since) {
				stop = true
				break
			}
			posts = append(posts, c.toPost(p, ownerID, comm.GroupID, date))
			if max != 0 && len(posts) >= max {
				break
			}
		}
		if stop || offset+vkPageSize >= resp.Count {
			break
		}
	}
	return posts, nil
}

// GetLikers возвращает VK-id пользователей, поставивших лайк публикации.
func (c *Client) GetLikers(ctx context.Context, ownerID, postVKID int) ([]int, error) {
	var users []int
	for offset := 0; ; offset += likesPageSize {
		if err := c.wait(ctx); err != nil {
			return nil, err
		}
		resp, err := c.api.LikesGetList(api.Params{
			"type":     "post",
			"owner_id": ownerID,
			"item_id":  postVKID,
			"count":    likesPageSize,
			"offset":   offset,
		}.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("likes.getList post=%d_%d: %w", ownerID, postVKID, err)
		}
		users = append(users, resp.Items...)
		if offset+likesPageSize >= resp.Count || len(resp.Items) == 0 {
			break
		}
	}
	return users, nil
}

// GetComments возвращает комментарии публикации (только от пользователей).
// postID — уже сформированный составной id поста для связи.
func (c *Client) GetComments(ctx context.Context, ownerID, postVKID int, postID string, max int) ([]model.Comment, error) {
	var comments []model.Comment
	for offset := 0; max == 0 || len(comments) < max; offset += vkPageSize {
		if err := c.wait(ctx); err != nil {
			return nil, err
		}
		resp, err := c.api.WallGetComments(api.Params{
			"owner_id": ownerID,
			"post_id":  postVKID,
			"count":    vkPageSize,
			"offset":   offset,
		}.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("wall.getComments post=%d_%d: %w", ownerID, postVKID, err)
		}
		if len(resp.Items) == 0 {
			break
		}
		for _, cm := range resp.Items {
			if cm.FromID <= 0 { // комментарии от сообществ пропускаем
				continue
			}
			if strings.TrimSpace(cm.Text) == "" {
				continue
			}
			comments = append(comments, toComment(cm, ownerID, postID))
			if max != 0 && len(comments) >= max {
				break
			}
		}
		if offset+vkPageSize >= resp.Count {
			break
		}
	}
	return comments, nil
}

// --- мапперы ---

// groupsGetByIDResponse — ответ groups.getById на версии API 5.199.
type groupsGetByIDResponse struct {
	Groups []groupInfo `json:"groups"`
}

type groupInfo struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	ScreenName   string `json:"screen_name"`
	Description  string `json:"description"`
	MembersCount int    `json:"members_count"`
	IsClosed     int    `json:"is_closed"`
}

func toCommunity(g groupInfo, region, city string) model.Community {
	url := "https://vk.com/" + g.ScreenName
	if g.ScreenName == "" {
		url = fmt.Sprintf("https://vk.com/club%d", g.ID)
	}
	return model.Community{
		GroupID:     strconv.Itoa(g.ID),
		Name:        trunc(g.Name, 50),
		URL:         trunc(url, 50),
		Description: trunc(g.Description, 100),
		Subscribers: g.MembersCount,
		Region:      region,
		City:        city,
	}
}

func (c *Client) toPost(p wallPost, ownerID int, groupID string, date time.Time) model.Post {
	post := model.Post{
		PostID:  fmt.Sprintf("%d_%d", ownerID, p.ID),
		GroupID: groupID,
		Text:    p.Text,
		Date:    date,
		URL:     trunc(fmt.Sprintf("https://vk.com/wall%d_%d", ownerID, p.ID), 50),
		OwnerID: ownerID,
		VKID:    p.ID,
	}
	if p.Reactions != nil {
		for _, r := range p.Reactions.Items {
			name, sentiment := reactionMeta(r.ID)
			if name == "" {
				c.log.Warn("vk: неизвестная реакция", "reaction_id", r.ID, "post_id", post.PostID)
			}
			post.Reactions = append(post.Reactions, model.Reaction{
				ReactionID:   fmt.Sprintf("%s_%d", post.PostID, r.ID),
				PostID:       post.PostID,
				VKReactionID: r.ID,
				Name:         name,
				Sentiment:    sentiment,
				Count:        r.Count,
			})
		}
	}
	return post
}

// reactionTable — маппинг id реакции VK (набор по умолчанию) в имя и тональность.
// 0 — обычный лайк (учитывается отдельно таблицей likes, поэтому neutral);
// 1..4 — позитивные эмодзи (сердце, восторг, смех, удивление); 5..6 — негативные
// (печаль, гнев). Точные имена — best-effort; для тональности важна лишь группа.
var reactionTable = map[int]struct {
	name      string
	sentiment model.Sentiment
}{
	0: {"like", model.Neutral},
	1: {"heart", model.Positive},
	2: {"fire", model.Positive},
	3: {"haha", model.Positive},
	4: {"wow", model.Positive},
	5: {"sad", model.Negative},
	6: {"angry", model.Negative},
}

// reactionMeta возвращает имя и тональность реакции. Неизвестный id (кастомные
// наборы реакций) → пустое имя и neutral, чтобы не искажать статистику.
func reactionMeta(id int) (string, *model.Sentiment) {
	if m, ok := reactionTable[id]; ok {
		s := m.sentiment
		return m.name, &s
	}
	s := model.Neutral
	return "", &s
}

func toComment(cm object.WallWallComment, ownerID int, postID string) model.Comment {
	return model.Comment{
		CommentID: fmt.Sprintf("%d_%d", ownerID, cm.ID),
		PostID:    postID,
		UserID:    strconv.Itoa(cm.FromID),
		Text:      cm.Text,
		Date:      time.Unix(int64(cm.Date), 0).UTC(),
	}
}

// NewUser строит доменного пользователя из VK-id.
func NewUser(vkID int) model.User {
	return model.User{
		UserID:     strconv.Itoa(vkID),
		VKID:       vkID,
		ProfileURL: fmt.Sprintf("https://vk.com/id%d", vkID),
	}
}

// trunc обрезает строку до n рун (поля схемы ограничены по длине).
func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
