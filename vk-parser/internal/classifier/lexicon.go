package classifier

import (
	"context"
	"strings"

	"github.com/plombir1917/vk-loyal-users-parser/internal/model"
)

// Lexicon — простой словарный классификатор без внешних зависимостей.
// Используется по умолчанию, когда LLM не настроена, и как fallback.
type Lexicon struct {
	positive []string
	negative []string
}

// NewLexicon создаёт словарный классификатор с базовым русским лексиконом.
func NewLexicon() *Lexicon {
	return &Lexicon{
		positive: []string{
			"спасибо", "отлично", "класс", "супер", "молодц", "хорош", "люблю",
			"прекрасн", "замечательн", "рад", "поддержив", "благодар", "лучш",
			"красив", "крут", "👍", "❤", "😍", "👏",
		},
		negative: []string{
			"плохо", "ужас", "отстой", "ненавиж", "позор", "стыд", "обман",
			"вранье", "враньё", "против", "негодов", "разочаров", "бесит",
			"кошмар", "жаль", "ужасн", "отврат", "👎", "😡", "🤮",
		},
	}
}

// Classify считает совпадения позитивных и негативных маркеров.
func (l *Lexicon) Classify(_ context.Context, text string) (model.Sentiment, error) {
	t := strings.ToLower(text)
	pos, neg := 0, 0
	for _, w := range l.positive {
		pos += strings.Count(t, w)
	}
	for _, w := range l.negative {
		neg += strings.Count(t, w)
	}
	switch {
	case neg > pos:
		return model.Negative, nil
	case pos > neg:
		return model.Positive, nil
	default:
		return model.Neutral, nil
	}
}
