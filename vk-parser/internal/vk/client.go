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
	return &Client{
		api: api.NewVK(token),
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
func (c *Client) SearchCommunities(ctx context.Context, city City, regionName string) ([]model.Community, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	resp, err := c.api.GroupsSearch(api.Params{
		"q":       city.Title,
		"city_id": city.ID,
		"count":   searchPageSize,
		"sort":    0,
	}.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("groups.search city=%d: %w", city.ID, err)
	}

	ids := make([]string, 0, len(resp.Items))
	for _, g := range resp.Items {
		if g.IsClosed != 0 { // только открытые сообщества
			continue
		}
		ids = append(ids, strconv.Itoa(g.ID))
	}
	if len(ids) == 0 {
		return nil, nil
	}

	var communities []model.Community
	for start := 0; start < len(ids); start += getByIDBatch {
		end := min(start+getByIDBatch, len(ids))
		if err := c.wait(ctx); err != nil {
			return nil, err
		}
		details, err := c.api.GroupsGetByID(api.Params{
			"group_ids": strings.Join(ids[start:end], ","),
			"fields":    "description,members_count,city",
		}.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("groups.getById: %w", err)
		}
		for _, g := range details {
			if g.IsClosed != 0 {
				continue
			}
			communities = append(communities, toCommunity(g, regionName, city.Title))
		}
	}
	return communities, nil
}

// GetPosts возвращает публикации сообщества не старше since (по убыванию даты).
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
		resp, err := c.api.WallGet(api.Params{
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
			posts = append(posts, toPost(p, ownerID, comm.GroupID, date))
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

func toCommunity(g object.GroupsGroup, region, city string) model.Community {
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

func toPost(p object.WallWallpost, ownerID int, groupID string, date time.Time) model.Post {
	return model.Post{
		PostID:  fmt.Sprintf("%d_%d", ownerID, p.ID),
		GroupID: groupID,
		Text:    p.Text,
		Date:    date,
		URL:     trunc(fmt.Sprintf("https://vk.com/wall%d_%d", ownerID, p.ID), 50),
		OwnerID: ownerID,
		VKID:    p.ID,
	}
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
