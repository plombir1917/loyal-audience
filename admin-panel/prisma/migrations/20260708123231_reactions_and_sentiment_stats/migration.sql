-- CreateTable
CREATE TABLE "reaction" (
    "reaction_id" VARCHAR(48) NOT NULL,
    "post_id" VARCHAR(32) NOT NULL,
    "vk_reaction_id" INTEGER NOT NULL,
    "reaction_name" VARCHAR(32),
    "sentiment" "Sentiment",
    "count" INTEGER NOT NULL,

    CONSTRAINT "reaction_pkey" PRIMARY KEY ("reaction_id")
);

-- CreateTable
CREATE TABLE "stats_sentiment_by_reaction" (
    "reaction_bucket" VARCHAR(16) NOT NULL,
    "posts" INTEGER NOT NULL,
    "positive_comments" INTEGER NOT NULL,
    "negative_comments" INTEGER NOT NULL,
    "neutral_comments" INTEGER NOT NULL,
    "total_comments" INTEGER NOT NULL,

    CONSTRAINT "stats_sentiment_by_reaction_pkey" PRIMARY KEY ("reaction_bucket")
);

-- CreateIndex
CREATE UNIQUE INDEX "reaction_post_id_vk_reaction_id_key" ON "reaction"("post_id", "vk_reaction_id");

-- AddForeignKey
ALTER TABLE "reaction" ADD CONSTRAINT "reaction_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "post"("post_id") ON DELETE CASCADE ON UPDATE CASCADE;
