-- CreateTable
CREATE TABLE "stats_likes_distribution" (
    "like_count" INTEGER NOT NULL,
    "users" INTEGER NOT NULL,
    "share_percent" DOUBLE PRECISION NOT NULL,

    CONSTRAINT "stats_likes_distribution_pkey" PRIMARY KEY ("like_count")
);
