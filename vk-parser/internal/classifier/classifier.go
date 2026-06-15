// Package classifier — классификация тональности комментариев
// (positive / negative / neutral). Модель подключается через интерфейс
// Classifier, поэтому её можно заменить без изменения основной логики.
package classifier

import (
	"context"
	"strings"

	"github.com/plombir1917/vk-loyal-users-parser/internal/model"
)

// Classifier классифицирует тональность текста.
type Classifier interface {
	Classify(ctx context.Context, text string) (model.Sentiment, error)
}

// normalize приводит произвольный ответ модели к одной из трёх категорий.
func normalize(label string) model.Sentiment {
	label = strings.ToLower(label)
	switch {
	case containsAny(label, "positive", "позитив", "pos"):
		return model.Positive
	case containsAny(label, "negative", "негатив", "neg"):
		return model.Negative
	default:
		return model.Neutral
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
