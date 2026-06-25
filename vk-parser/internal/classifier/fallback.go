package classifier

import (
	"context"
	"log/slog"

	"github.com/plombir1917/vk-loyal-users-parser/internal/model"
)

// Fallback пробует основной классификатор (реальную модель) и при ошибке
// откатывается на запасной (словарный). Так сбой или недоступность модели не
// прерывает пайплайн и не приводит к молчаливому neutral — комментарий
// классифицируется словарём.
type Fallback struct {
	primary   Classifier
	secondary Classifier
	log       *slog.Logger
}

// NewFallback создаёт классификатор с откатом primary → secondary.
func NewFallback(primary, secondary Classifier, log *slog.Logger) *Fallback {
	return &Fallback{primary: primary, secondary: secondary, log: log}
}

// Classify вызывает основной классификатор; при ошибке логирует и использует
// запасной.
func (f *Fallback) Classify(ctx context.Context, text string) (model.Sentiment, error) {
	s, err := f.primary.Classify(ctx, text)
	if err != nil {
		f.log.Warn("classifier: откат на запасной классификатор", "err", err)
		return f.secondary.Classify(ctx, text)
	}
	return s, nil
}
