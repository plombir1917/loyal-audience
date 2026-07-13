import { Injectable } from '@nestjs/common';
import { PrismaService } from '../../../../../prisma/prisma.service.js';

export interface RecalcResult {
  users_updated: number;
  likes_distribution_rows: number;
  comments_by_likes_rows: number;
}

/**
 * Пересчёт метрик пользователей и материализованных stats-таблиц из данных, уже
 * лежащих в БД. Это тот же расчёт, что делает Go-парсер в конце прогона
 * (RecomputeUsers + RecomputeStats), но его можно запустить кнопкой «Рассчитать»
 * в админке — вне зависимости от выполнения парсера. SQL зеркалит
 * vk-parser/internal/storage/storage.go; пороги ядра берутся из окружения
 * (те же имена и значения по умолчанию, что у парсера).
 */
@Injectable()
export class StatsService {
  constructor(private readonly prisma: PrismaService) {}

  private intEnv(key: string, def: number): number {
    const n = Number.parseInt(process.env[key] ?? '', 10);
    return Number.isFinite(n) ? n : def;
  }

  private get likeThreshold(): number {
    return this.intEnv('LIKE_THRESHOLD', 1);
  }

  private get commentThreshold(): number {
    return this.intEnv('COMMENT_THRESHOLD', 1);
  }

  private get combineOr(): boolean {
    return (process.env.CORE_COMBINE ?? 'and').toLowerCase() === 'or';
  }

  /** Полный пересчёт: сначала метрики пользователей, затем все stats-таблицы. */
  async recompute(): Promise<RecalcResult> {
    const usersUpdated = await this.recomputeUsers();
    await this.recomputeSentimentByReaction();
    await this.recomputePostSentimentMap();
    await this.recomputeCore();
    const likesRows = await this.recomputeLikesDistribution();
    const commentsRows = await this.recomputeCommentsByLikes();
    return {
      users_updated: usersUpdated,
      likes_distribution_rows: likesRows,
      comments_by_likes_rows: commentsRows,
    };
  }

  // Метрики вовлечённости и метка каждого пользователя.
  // is_core = (лайков > LIKE_THRESHOLD) [AND|OR] (позитивных > COMMENT_THRESHOLD);
  // сегмент: ядро → Loyal; иначе есть негативный комментарий → Disloyal; иначе
  // Neutral. Пороги — валидированные целые, op — контролируемый литерал, поэтому
  // безопасно подставляются в текст запроса.
  private recomputeUsers(): Promise<number> {
    const op = this.combineOr ? ' OR ' : ' AND ';
    const likeThr = this.likeThreshold;
    const commentThr = this.commentThreshold;
    const core = `((m.like_count > ${likeThr})${op}(m.comment_positive > ${commentThr}))`;
    return this.prisma.$executeRawUnsafe(`
      UPDATE users u SET
        like_count       = m.like_count,
        comment_positive = m.comment_positive,
        comment_negative = m.comment_negative,
        comment_neutral  = m.comment_neutral,
        is_core          = ${core},
        segment          = (
          CASE
            WHEN ${core} THEN 'Loyal'
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
      WHERE u.user_id = m.user_id`);
  }

  // Задача 2: распределение тональностей комментариев по бакету доминирующей
  // реакции поста.
  private recomputeSentimentByReaction(): Promise<number> {
    return this.prisma.$executeRawUnsafe(`
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
        total_comments = EXCLUDED.total_comments`);
  }

  // Задача 4А: связь тональности комментариев и лайков с тональностью поста.
  private recomputePostSentimentMap(): Promise<number> {
    return this.prisma.$executeRawUnsafe(`
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
        likes = EXCLUDED.likes`);
  }

  // Задачи 4Б/5: сводка по ядру в двух вариантах (likes_only / likes_plus_comments).
  private recomputeCore(): Promise<number> {
    const likeThr = this.likeThreshold;
    return this.prisma.$executeRawUnsafe(`
      INSERT INTO stats_core (variant, total_users, core_users, core_share)
      SELECT 'likes_only',
        COUNT(*),
        COUNT(*) FILTER (WHERE like_count > ${likeThr}),
        COALESCE(COUNT(*) FILTER (WHERE like_count > ${likeThr})::float / NULLIF(COUNT(*), 0), 0)
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
        core_share = EXCLUDED.core_share`);
  }

  // Распределение пользователей по числу лайков. Ключи динамические, поэтому
  // таблица полностью пересобирается в транзакции (DELETE + INSERT).
  private async recomputeLikesDistribution(): Promise<number> {
    const [, inserted] = await this.prisma.$transaction([
      this.prisma.$executeRawUnsafe(`DELETE FROM stats_likes_distribution`),
      this.prisma.$executeRawUnsafe(`
        INSERT INTO stats_likes_distribution (like_count, users, share_percent)
        SELECT like_count,
          COUNT(*),
          COALESCE(100.0 * COUNT(*)::float / NULLIF((SELECT COUNT(*) FROM users), 0), 0)
        FROM users
        WHERE like_count > 0
        GROUP BY like_count`),
    ]);
    return inserted;
  }

  // Комментарии пользователей в разрезе числа поставленных лайков. Ключи
  // динамические — пересобираем в транзакции (DELETE + INSERT).
  private async recomputeCommentsByLikes(): Promise<number> {
    const [, inserted] = await this.prisma.$transaction([
      this.prisma.$executeRawUnsafe(`DELETE FROM stats_comments_by_likes`),
      this.prisma.$executeRawUnsafe(`
        INSERT INTO stats_comments_by_likes
          (like_count, comments, positive_comments, negative_comments, neutral_comments)
        SELECT like_count,
          SUM(comment_positive + comment_negative + comment_neutral),
          SUM(comment_positive),
          SUM(comment_negative),
          SUM(comment_neutral)
        FROM users
        WHERE like_count > 0
        GROUP BY like_count`),
    ]);
    return inserted;
  }
}
