package classifier

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/plombir1917/vk-loyal-users-parser/internal/model"
)

// stubClassifier — управляемая заглушка для проверки логики отката.
type stubClassifier struct {
	sentiment model.Sentiment
	err       error
	called    bool
}

func (s *stubClassifier) Classify(_ context.Context, _ string) (model.Sentiment, error) {
	s.called = true
	return s.sentiment, s.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestFallbackUsesSecondaryOnError(t *testing.T) {
	primary := &stubClassifier{err: errors.New("llm недоступна")}
	secondary := &stubClassifier{sentiment: model.Negative}

	f := NewFallback(primary, secondary, discardLogger())
	got, err := f.Classify(context.Background(), "текст")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != model.Negative {
		t.Errorf("Classify = %q, want %q", got, model.Negative)
	}
	if !secondary.called {
		t.Error("secondary classifier was not called on primary error")
	}
}

func TestFallbackSkipsSecondaryOnSuccess(t *testing.T) {
	primary := &stubClassifier{sentiment: model.Positive}
	secondary := &stubClassifier{sentiment: model.Negative}

	f := NewFallback(primary, secondary, discardLogger())
	got, err := f.Classify(context.Background(), "текст")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != model.Positive {
		t.Errorf("Classify = %q, want %q", got, model.Positive)
	}
	if secondary.called {
		t.Error("secondary classifier should not be called when primary succeeds")
	}
}
