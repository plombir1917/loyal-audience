-- CreateTable
CREATE TABLE "stats_comments_by_likes" (
    "like_count" INTEGER NOT NULL,
    "comments" INTEGER NOT NULL,
    "positive_comments" INTEGER NOT NULL,
    "negative_comments" INTEGER NOT NULL,
    "neutral_comments" INTEGER NOT NULL,

    CONSTRAINT "stats_comments_by_likes_pkey" PRIMARY KEY ("like_count")
);
