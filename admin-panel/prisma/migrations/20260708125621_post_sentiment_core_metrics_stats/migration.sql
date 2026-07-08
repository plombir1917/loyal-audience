-- AlterTable
ALTER TABLE "post" ADD COLUMN     "sentiment" "Sentiment";

-- AlterTable
ALTER TABLE "users" ADD COLUMN     "comment_negative" INTEGER NOT NULL DEFAULT 0,
ADD COLUMN     "comment_neutral" INTEGER NOT NULL DEFAULT 0,
ADD COLUMN     "comment_positive" INTEGER NOT NULL DEFAULT 0,
ADD COLUMN     "is_core" BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN     "like_count" INTEGER NOT NULL DEFAULT 0;

-- CreateTable
CREATE TABLE "stats_post_sentiment_map" (
    "post_sentiment" VARCHAR(16) NOT NULL,
    "posts" INTEGER NOT NULL,
    "positive_comments" INTEGER NOT NULL,
    "negative_comments" INTEGER NOT NULL,
    "neutral_comments" INTEGER NOT NULL,
    "likes" INTEGER NOT NULL,

    CONSTRAINT "stats_post_sentiment_map_pkey" PRIMARY KEY ("post_sentiment")
);

-- CreateTable
CREATE TABLE "stats_core" (
    "variant" VARCHAR(32) NOT NULL,
    "total_users" INTEGER NOT NULL,
    "core_users" INTEGER NOT NULL,
    "core_share" DOUBLE PRECISION NOT NULL,

    CONSTRAINT "stats_core_pkey" PRIMARY KEY ("variant")
);
