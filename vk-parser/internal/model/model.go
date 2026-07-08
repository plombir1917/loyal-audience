// Package model содержит доменные сущности, общие для слоёв vk, classifier,
// storage и service. Структура соответствует схеме данных (Prisma в admin-panel).
package model

import "time"

// Sentiment — тональность комментария.
type Sentiment string

const (
	Positive Sentiment = "positive"
	Negative Sentiment = "negative"
	Neutral  Sentiment = "neutral"
)

// Segment — сегмент пользователя по лояльности.
type Segment string

const (
	SegmentLoyal    Segment = "Loyal"
	SegmentNeutral  Segment = "Neutral"
	SegmentDisloyal Segment = "Disloyal"
)

// Community — сообщество ВК.
type Community struct {
	GroupID     string // VK group id (строка)
	Name        string
	URL         string
	Description string
	Subscribers int
	Region      string
	City        string
}

// Post — публикация сообщества.
type Post struct {
	PostID  string // "{ownerID}_{postID}"
	GroupID string
	Text    string
	Date    time.Time
	URL     string
	OwnerID int // отрицательный id владельца стены (для wall.* методов)
	VKID    int // числовой id поста внутри стены

	// Reactions — агрегат реакций поста (приходит инлайн из wall.get). Поле
	// транзиентное: UpsertPost его не пишет, реакции сохраняются отдельно.
	Reactions []Reaction
}

// Reaction — агрегат одного типа реакции (эмодзи) под публикацией. VK отдаёт
// только суммарные счётчики по объекту, без привязки к пользователю.
type Reaction struct {
	ReactionID   string // "{postID}_{vkReactionID}"
	PostID       string
	VKReactionID int    // id реакции в VK (0 — обычный лайк, 1..N — эмодзи)
	Name         string // человекочитаемое имя, best-effort ("" — неизвестна)
	Sentiment    *Sentiment
	Count        int
}

// User — пользователь ВК.
type User struct {
	UserID     string // строковое представление VKID
	VKID       int
	ProfileURL string
}

// Like — лайк пользователя под публикацией.
type Like struct {
	LikeID string // "{postID}_{userID}"
	PostID string
	UserID string
}

// Comment — комментарий к публикации.
type Comment struct {
	CommentID string // "{ownerID}_{commentID}"
	PostID    string
	UserID    string
	Text      string
	Date      time.Time
	Sentiment *Sentiment // nil, пока не классифицирован
}
