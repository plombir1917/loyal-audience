// Package storage — слой хранения собранных данных (сообщества, посты,
// пользователи, лайки, комментарии, сегменты) в общем с admin-panel Postgres.
// Схему создаёт Prisma; этот слой только пишет данные идемпотентными upsert'ами.
package storage

import (
	"context"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plombir1917/vk-loyal-users-parser/internal/model"
)

// Storage — пул соединений к Postgres.
type Storage struct {
	pool *pgxpool.Pool
}

// New открывает пул соединений и проверяет связь.
func New(ctx context.Context, dsn string) (*Storage, error) {
	pool, err := pgxpool.New(ctx, normalizeDSN(dsn))
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Storage{pool: pool}, nil
}

// Close закрывает пул соединений.
func (s *Storage) Close() { s.pool.Close() }

// normalizeDSN приводит Prisma-совместимую строку подключения к понятной pgx:
// параметр schema (специфичный для Prisma) заменяется на search_path.
func normalizeDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	q := u.Query()
	if schema := q.Get("schema"); schema != "" {
		q.Del("schema")
		if q.Get("search_path") == "" {
			q.Set("search_path", schema)
		}
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// UpsertCommunity сохраняет сообщество (идемпотентно по group_id).
func (s *Storage) UpsertCommunity(ctx context.Context, c model.Community) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO community (group_id, group_name, group_url, group_description, group_subscribers, region, city)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (group_id) DO UPDATE SET
			group_name = EXCLUDED.group_name,
			group_url = EXCLUDED.group_url,
			group_description = EXCLUDED.group_description,
			group_subscribers = EXCLUDED.group_subscribers,
			region = EXCLUDED.region,
			city = EXCLUDED.city`,
		c.GroupID, c.Name, c.URL, c.Description, c.Subscribers, c.Region, c.City)
	if err != nil {
		return fmt.Errorf("upsert community %s: %w", c.GroupID, err)
	}
	return nil
}

// UpsertPost сохраняет публикацию (идемпотентно по post_id).
func (s *Storage) UpsertPost(ctx context.Context, p model.Post) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO post (post_id, group_id, post_text, post_date, post_url)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (post_id) DO UPDATE SET
			post_text = EXCLUDED.post_text,
			post_date = EXCLUDED.post_date,
			post_url = EXCLUDED.post_url`,
		p.PostID, p.GroupID, p.Text, p.Date, p.URL)
	if err != nil {
		return fmt.Errorf("upsert post %s: %w", p.PostID, err)
	}
	return nil
}

// UpsertUser сохраняет пользователя (идемпотентно по user_id).
func (s *Storage) UpsertUser(ctx context.Context, u model.User) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (user_id, user_vk_id, user_profile_url)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET
			user_profile_url = EXCLUDED.user_profile_url`,
		u.UserID, int64(u.VKID), u.ProfileURL)
	if err != nil {
		return fmt.Errorf("upsert user %s: %w", u.UserID, err)
	}
	return nil
}

// UpsertLike сохраняет лайк (идемпотентно по like_id и по паре post/user).
func (s *Storage) UpsertLike(ctx context.Context, l model.Like) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO likes (like_id, post_id, user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (post_id, user_id) DO NOTHING`,
		l.LikeID, l.PostID, l.UserID)
	if err != nil {
		return fmt.Errorf("upsert like %s: %w", l.LikeID, err)
	}
	return nil
}

// UpsertComment сохраняет комментарий с тональностью (идемпотентно по comment_id).
func (s *Storage) UpsertComment(ctx context.Context, c model.Comment) error {
	var sentiment *string
	if c.Sentiment != nil {
		v := string(*c.Sentiment)
		sentiment = &v
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO comment (comment_id, post_id, user_id, comment_text, comment_date, sentiment)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (comment_id) DO UPDATE SET
			comment_text = EXCLUDED.comment_text,
			comment_date = EXCLUDED.comment_date,
			sentiment = EXCLUDED.sentiment`,
		c.CommentID, c.PostID, c.UserID, c.Text, c.Date, sentiment)
	if err != nil {
		return fmt.Errorf("upsert comment %s: %w", c.CommentID, err)
	}
	return nil
}

// CommunityExists сообщает, сохранено ли уже сообщество с таким group_id.
func (s *Storage) CommunityExists(ctx context.Context, groupID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM community WHERE group_id = $1)`, groupID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("community exists %s: %w", groupID, err)
	}
	return exists, nil
}

// ExistingCommentIDs возвращает множество comment_id поста, которые уже сохранены
// с проставленной тональностью. Сервис пропускает их, чтобы не вызывать
// классификатор повторно (главная статья времени и лимитов LLM).
func (s *Storage) ExistingCommentIDs(ctx context.Context, postID string) (map[string]struct{}, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT comment_id FROM comment WHERE post_id = $1 AND sentiment IS NOT NULL`, postID)
	if err != nil {
		return nil, fmt.Errorf("existing comments for post %s: %w", postID, err)
	}
	defer rows.Close()

	ids := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan comment id: %w", err)
		}
		ids[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comment ids: %w", err)
	}
	return ids, nil
}

// ExistingLikeUserIDs возвращает множество user_id, чьи лайки на посте уже
// сохранены. Сервис пропускает их, чтобы не дублировать upsert.
func (s *Storage) ExistingLikeUserIDs(ctx context.Context, postID string) (map[string]struct{}, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT user_id FROM likes WHERE post_id = $1`, postID)
	if err != nil {
		return nil, fmt.Errorf("existing likes for post %s: %w", postID, err)
	}
	defer rows.Close()

	ids := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan like user id: %w", err)
		}
		ids[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate like user ids: %w", err)
	}
	return ids, nil
}

// UpdateUserSegment проставляет конкретному пользователю сегмент лояльности.
func (s *Storage) UpdateUserSegment(ctx context.Context, userID string, segment model.Segment) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET segment = $2::"Segment" WHERE user_id = $1`,
		userID, string(segment))
	if err != nil {
		return fmt.Errorf("update segment user %s: %w", userID, err)
	}
	return nil
}

// SegmentUsers пересчитывает сегменты всех пользователей одним запросом по
// правилу: негативный комментарий → Disloyal; иначе лайк или позитивный
// комментарий → Loyal; иначе Neutral. Возвращает число обновлённых строк.
func (s *Storage) SegmentUsers(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE users u SET segment = (
			CASE
				WHEN EXISTS (
					SELECT 1 FROM comment c
					WHERE c.user_id = u.user_id AND c.sentiment = 'negative'
				) THEN 'Disloyal'
				WHEN EXISTS (SELECT 1 FROM likes l WHERE l.user_id = u.user_id)
					OR EXISTS (
						SELECT 1 FROM comment c
						WHERE c.user_id = u.user_id AND c.sentiment = 'positive'
					) THEN 'Loyal'
				ELSE 'Neutral'
			END
		)::"Segment"`)
	if err != nil {
		return 0, fmt.Errorf("segment users: %w", err)
	}
	return tag.RowsAffected(), nil
}
