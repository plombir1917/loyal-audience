-- CreateEnum
CREATE TYPE "Sentiment" AS ENUM ('positive', 'negative', 'neutral');

-- CreateEnum
CREATE TYPE "Segment" AS ENUM ('Loyal', 'Neutral', 'Disloyal');

-- CreateTable
CREATE TABLE "community" (
    "group_id" VARCHAR(32) NOT NULL,
    "group_name" VARCHAR(50) NOT NULL,
    "group_url" VARCHAR(50) NOT NULL,
    "group_description" VARCHAR(100) NOT NULL,
    "group_subscribers" INTEGER NOT NULL,
    "region" TEXT,
    "city" TEXT,

    CONSTRAINT "community_pkey" PRIMARY KEY ("group_id")
);

-- CreateTable
CREATE TABLE "post" (
    "post_id" VARCHAR(32) NOT NULL,
    "group_id" VARCHAR(32) NOT NULL,
    "post_text" TEXT NOT NULL,
    "post_date" TIMESTAMP(3) NOT NULL,
    "post_url" VARCHAR(50) NOT NULL,

    CONSTRAINT "post_pkey" PRIMARY KEY ("post_id")
);

-- CreateTable
CREATE TABLE "comment" (
    "comment_id" VARCHAR(32) NOT NULL,
    "post_id" VARCHAR(32) NOT NULL,
    "user_id" VARCHAR(32) NOT NULL,
    "comment_text" TEXT NOT NULL,
    "comment_date" TIMESTAMP(3) NOT NULL,
    "sentiment" "Sentiment",

    CONSTRAINT "comment_pkey" PRIMARY KEY ("comment_id")
);

-- CreateTable
CREATE TABLE "likes" (
    "like_id" VARCHAR(32) NOT NULL,
    "post_id" VARCHAR(32) NOT NULL,
    "user_id" VARCHAR(32) NOT NULL,

    CONSTRAINT "likes_pkey" PRIMARY KEY ("like_id")
);

-- CreateTable
CREATE TABLE "users" (
    "user_id" VARCHAR(32) NOT NULL,
    "user_vk_id" BIGINT NOT NULL,
    "user_profile_url" VARCHAR(50) NOT NULL,
    "segment" "Segment",

    CONSTRAINT "users_pkey" PRIMARY KEY ("user_id")
);

-- CreateIndex
CREATE UNIQUE INDEX "likes_post_id_user_id_key" ON "likes"("post_id", "user_id");

-- CreateIndex
CREATE UNIQUE INDEX "users_user_vk_id_key" ON "users"("user_vk_id");

-- AddForeignKey
ALTER TABLE "post" ADD CONSTRAINT "post_group_id_fkey" FOREIGN KEY ("group_id") REFERENCES "community"("group_id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "comment" ADD CONSTRAINT "comment_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "post"("post_id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "comment" ADD CONSTRAINT "comment_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "users"("user_id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "likes" ADD CONSTRAINT "likes_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "post"("post_id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "likes" ADD CONSTRAINT "likes_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "users"("user_id") ON DELETE CASCADE ON UPDATE CASCADE;
