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

// UpsertReaction сохраняет агрегат реакции поста (идемпотентно по паре
// post_id + vk_reaction_id). Счётчик и тональность обновляются — реакции со
// временем набираются, поэтому при повторном прогоне значения освежаются.
func (s *Storage) UpsertReaction(ctx context.Context, r model.Reaction) error {
	var name *string
	if r.Name != "" {
		name = &r.Name
	}
	var sentiment *string
	if r.Sentiment != nil {
		v := string(*r.Sentiment)
		sentiment = &v
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO reaction (reaction_id, post_id, vk_reaction_id, reaction_name, sentiment, count)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (post_id, vk_reaction_id) DO UPDATE SET
			reaction_name = EXCLUDED.reaction_name,
			sentiment = EXCLUDED.sentiment,
			count = EXCLUDED.count`,
		r.ReactionID, r.PostID, r.VKReactionID, name, sentiment, r.Count)
	if err != nil {
		return fmt.Errorf("upsert reaction %s: %w", r.ReactionID, err)
	}
	return nil
}

// RecomputeStats пересчитывает материализованные таблицы статистики. likeThr —
// порог лайков для варианта stats_core.likes_only. Все три таблицы имеют
// фиксированный набор строк и обновляются по конфликту.
func (s *Storage) RecomputeStats(ctx context.Context, likeThr int) error {
	if err := s.recomputeSentimentByReaction(ctx); err != nil {
		return err
	}
	if err := s.recomputePostSentimentMap(ctx); err != nil {
		return err
	}
	if err := s.recomputeCore(ctx, likeThr); err != nil {
		return err
	}
	if err := s.recomputeLikesDistribution(ctx); err != nil {
		return err
	}
	if err := s.recomputeCommentsByLikes(ctx); err != nil {
		return err
	}
	return nil
}

// recomputeCommentsByLikes: комментарии пользователей в разрезе числа
// поставленных ими лайков — по строке на каждое встречающееся значение
// like_count (от 1): всего комментариев и разбивка по тональности (метрики
// comment_* уже посчитаны RecomputeUsers). Ключи динамические, поэтому таблица
// пересобирается в транзакции (DELETE + INSERT).
func (s *Storage) recomputeCommentsByLikes(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("recompute comments_by_likes: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM stats_comments_by_likes`); err != nil {
		return fmt.Errorf("recompute comments_by_likes: delete: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO stats_comments_by_likes
			(like_count, comments, positive_comments, negative_comments, neutral_comments)
		SELECT like_count,
			SUM(comment_positive + comment_negative + comment_neutral),
			SUM(comment_positive),
			SUM(comment_negative),
			SUM(comment_neutral)
		FROM users
		WHERE like_count > 0
		GROUP BY like_count`); err != nil {
		return fmt.Errorf("recompute comments_by_likes: insert: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("recompute comments_by_likes: commit: %w", err)
	}
	return nil
}

// recomputeLikesDistribution: распределение пользователей по числу поставленных
// лайков — по строке на каждое встречающееся значение like_count (от 1):
// абсолютное число таких пользователей и их доля (в %) от общего числа
// пользователей. Ключи динамические (набор значений like_count меняется между
// прогонами), поэтому таблица полностью пересобирается в транзакции
// (DELETE + INSERT), иначе остались бы строки для исчезнувших значений.
func (s *Storage) recomputeLikesDistribution(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("recompute likes_distribution: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM stats_likes_distribution`); err != nil {
		return fmt.Errorf("recompute likes_distribution: delete: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO stats_likes_distribution (like_count, users, share_percent)
		SELECT like_count,
			COUNT(*),
			COALESCE(100.0 * COUNT(*)::float / NULLIF((SELECT COUNT(*) FROM users), 0), 0)
		FROM users
		WHERE like_count > 0
		GROUP BY like_count`); err != nil {
		return fmt.Errorf("recompute likes_distribution: insert: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("recompute likes_distribution: commit: %w", err)
	}
	return nil
}

// recomputeSentimentByReaction (задача 2): распределение тональностей
// комментариев по бакету доминирующей реакции поста. Пост относится к бакету
// positive/negative по перевесу эмоциональных реакций (лайк id 0 — нейтральный,
// учитывается отдельно), neutral — при равенстве, none — если реакций нет.
func (s *Storage) recomputeSentimentByReaction(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO stats_sentiment_by_reaction
			(reaction_bucket, posts, positive_comments, negative_comments, neutral_comments, total_comments)
		SELECT b.bucket,
			COALESCE(agg.posts, 0),
			COALESCE(agg.positive_comments, 0),
			COALESCE(agg.negative_comments, 0),
			COALESCE(agg.neutral_comments, 0),
			COALESCE(agg.total_comments, 0)
		FROM (VALUES ('positive'), ('negative'), ('neutral'), ('none')) AS b(bucket)
		LEFT JOIN (
			WITH post_bucket AS (
				SELECT p.post_id,
					CASE
						WHEN COALESCE(r.pos, 0) = 0 AND COALESCE(r.neg, 0) = 0 AND COALESCE(r.neu, 0) = 0 THEN 'none'
						WHEN COALESCE(r.pos, 0) > COALESCE(r.neg, 0) THEN 'positive'
						WHEN COALESCE(r.neg, 0) > COALESCE(r.pos, 0) THEN 'negative'
						ELSE 'neutral'
					END AS bucket
				FROM post p
				LEFT JOIN (
					SELECT post_id,
						SUM(count) FILTER (WHERE sentiment = 'positive') AS pos,
						SUM(count) FILTER (WHERE sentiment = 'negative') AS neg,
						SUM(count) FILTER (WHERE sentiment = 'neutral')  AS neu
					FROM reaction
					GROUP BY post_id
				) r ON r.post_id = p.post_id
			)
			SELECT pb.bucket,
				COUNT(DISTINCT pb.post_id) AS posts,
				COUNT(*) FILTER (WHERE c.sentiment = 'positive') AS positive_comments,
				COUNT(*) FILTER (WHERE c.sentiment = 'negative') AS negative_comments,
				COUNT(*) FILTER (WHERE c.sentiment = 'neutral')  AS neutral_comments,
				COUNT(c.comment_id) AS total_comments
			FROM post_bucket pb
			LEFT JOIN comment c ON c.post_id = pb.post_id
			GROUP BY pb.bucket
		) agg ON agg.bucket = b.bucket
		ON CONFLICT (reaction_bucket) DO UPDATE SET
			posts = EXCLUDED.posts,
			positive_comments = EXCLUDED.positive_comments,
			negative_comments = EXCLUDED.negative_comments,
			neutral_comments = EXCLUDED.neutral_comments,
			total_comments = EXCLUDED.total_comments`)
	if err != nil {
		return fmt.Errorf("recompute sentiment_by_reaction: %w", err)
	}
	return nil
}

// recomputePostSentimentMap (задача 4А): связь тональности комментариев и лайков
// с тональностью поста. По строке на тональность поста (none — не классифицирован).
func (s *Storage) recomputePostSentimentMap(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO stats_post_sentiment_map
			(post_sentiment, posts, positive_comments, negative_comments, neutral_comments, likes)
		SELECT b.ps,
			COALESCE(pc.posts, 0),
			COALESCE(cc.pos, 0),
			COALESCE(cc.neg, 0),
			COALESCE(cc.neu, 0),
			COALESCE(lc.likes, 0)
		FROM (VALUES ('positive'), ('negative'), ('neutral'), ('none')) AS b(ps)
		LEFT JOIN (
			SELECT COALESCE(sentiment::text, 'none') AS ps, COUNT(*) AS posts
			FROM post GROUP BY 1
		) pc ON pc.ps = b.ps
		LEFT JOIN (
			SELECT COALESCE(p.sentiment::text, 'none') AS ps,
				COUNT(*) FILTER (WHERE c.sentiment = 'positive') AS pos,
				COUNT(*) FILTER (WHERE c.sentiment = 'negative') AS neg,
				COUNT(*) FILTER (WHERE c.sentiment = 'neutral')  AS neu
			FROM post p LEFT JOIN comment c ON c.post_id = p.post_id
			GROUP BY 1
		) cc ON cc.ps = b.ps
		LEFT JOIN (
			SELECT COALESCE(p.sentiment::text, 'none') AS ps, COUNT(l.like_id) AS likes
			FROM post p LEFT JOIN likes l ON l.post_id = p.post_id
			GROUP BY 1
		) lc ON lc.ps = b.ps
		ON CONFLICT (post_sentiment) DO UPDATE SET
			posts = EXCLUDED.posts,
			positive_comments = EXCLUDED.positive_comments,
			negative_comments = EXCLUDED.negative_comments,
			neutral_comments = EXCLUDED.neutral_comments,
			likes = EXCLUDED.likes`)
	if err != nil {
		return fmt.Errorf("recompute post_sentiment_map: %w", err)
	}
	return nil
}

// recomputeCore (задачи 4Б/5): сводка по ядру в двух вариантах. likes_only —
// core по лайкам (> likeThr); likes_plus_comments — по колонке is_core (полная
// формула, посчитанная RecomputeUsers).
func (s *Storage) recomputeCore(ctx context.Context, likeThr int) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO stats_core (variant, total_users, core_users, core_share)
		SELECT 'likes_only',
			COUNT(*),
			COUNT(*) FILTER (WHERE like_count > $1),
			COALESCE(COUNT(*) FILTER (WHERE like_count > $1)::float / NULLIF(COUNT(*), 0), 0)
		FROM users
		UNION ALL
		SELECT 'likes_plus_comments',
			COUNT(*),
			COUNT(*) FILTER (WHERE is_core),
			COALESCE(COUNT(*) FILTER (WHERE is_core)::float / NULLIF(COUNT(*), 0), 0)
		FROM users
		ON CONFLICT (variant) DO UPDATE SET
			total_users = EXCLUDED.total_users,
			core_users = EXCLUDED.core_users,
			core_share = EXCLUDED.core_share`, likeThr)
	if err != nil {
		return fmt.Errorf("recompute core: %w", err)
	}
	return nil
}

// AllCommunityIDs возвращает множество group_id всех уже сохранённых сообществ.
// Загружается один раз в начале прогона, чтобы при SKIP_EXISTING_COMMUNITIES
// отсеивать известные группы на самом раннем этапе — до обогащения через
// groups.getById, — и не тратить на них время.
func (s *Storage) AllCommunityIDs(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.pool.Query(ctx, `SELECT group_id FROM community`)
	if err != nil {
		return nil, fmt.Errorf("all community ids: %w", err)
	}
	defer rows.Close()

	ids := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan group id: %w", err)
		}
		ids[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group ids: %w", err)
	}
	return ids, nil
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

// UpdatePostSentiment проставляет посту тональность его содержания.
func (s *Storage) UpdatePostSentiment(ctx context.Context, postID string, sentiment model.Sentiment) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE post SET sentiment = $2::"Sentiment" WHERE post_id = $1`,
		postID, string(sentiment))
	if err != nil {
		return fmt.Errorf("update post sentiment %s: %w", postID, err)
	}
	return nil
}

// ExistingPostSentiments возвращает post_id постов сообщества, уже имеющих
// тональность. Сервис пропускает их, чтобы не классифицировать повторно.
func (s *Storage) ExistingPostSentiments(ctx context.Context, groupID string) (map[string]struct{}, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT post_id FROM post WHERE group_id = $1 AND sentiment IS NOT NULL`, groupID)
	if err != nil {
		return nil, fmt.Errorf("existing post sentiments for group %s: %w", groupID, err)
	}
	defer rows.Close()

	ids := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan post id: %w", err)
		}
		ids[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate post ids: %w", err)
	}
	return ids, nil
}

// RecomputeUsers одним запросом пересчитывает метрики вовлечённости и метку
// каждого пользователя. is_core = (лайков > likeThr) [AND|OR] (позитивных
// комментариев > commentThr); операция — combineOr. Сегмент: ядро → Loyal;
// иначе есть негативный комментарий → Disloyal; иначе Neutral. Возвращает число
// обновлённых строк.
func (s *Storage) RecomputeUsers(ctx context.Context, likeThr, commentThr int, combineOr bool) (int64, error) {
	op := " AND "
	if combineOr {
		op = " OR "
	}
	// core — булево выражение ядра; подставляется и в is_core, и в CASE сегмента.
	core := fmt.Sprintf("((m.like_count > $1)%s(m.comment_positive > $2))", op)

	query := fmt.Sprintf(`
		UPDATE users u SET
			like_count       = m.like_count,
			comment_positive = m.comment_positive,
			comment_negative = m.comment_negative,
			comment_neutral  = m.comment_neutral,
			is_core          = %[1]s,
			segment          = (
				CASE
					WHEN %[1]s THEN 'Loyal'
					WHEN m.comment_negative > 0 THEN 'Disloyal'
					ELSE 'Neutral'
				END
			)::"Segment"
		FROM (
			SELECT us.user_id,
				COALESCE(l.cnt, 0) AS like_count,
				COALESCE(c.pos, 0) AS comment_positive,
				COALESCE(c.neg, 0) AS comment_negative,
				COALESCE(c.neu, 0) AS comment_neutral
			FROM users us
			LEFT JOIN (
				SELECT user_id, COUNT(*) AS cnt FROM likes GROUP BY user_id
			) l ON l.user_id = us.user_id
			LEFT JOIN (
				SELECT user_id,
					COUNT(*) FILTER (WHERE sentiment = 'positive') AS pos,
					COUNT(*) FILTER (WHERE sentiment = 'negative') AS neg,
					COUNT(*) FILTER (WHERE sentiment = 'neutral')  AS neu
				FROM comment GROUP BY user_id
			) c ON c.user_id = us.user_id
		) m
		WHERE u.user_id = m.user_id`, core)

	tag, err := s.pool.Exec(ctx, query, likeThr, commentThr)
	if err != nil {
		return 0, fmt.Errorf("recompute users: %w", err)
	}
	return tag.RowsAffected(), nil
}
