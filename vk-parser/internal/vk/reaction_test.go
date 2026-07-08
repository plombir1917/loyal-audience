package vk

import (
	"testing"

	"github.com/plombir1917/vk-loyal-users-parser/internal/model"
)

// TestReactionMeta фиксирует маппинг id реакции VK в имя и тональность:
// 0 — обычный лайк (neutral, учитывается таблицей likes), 1..4 — позитивные
// эмодзи, 5..6 — негативные, неизвестный id — пустое имя и neutral.
func TestReactionMeta(t *testing.T) {
	cases := []struct {
		id       int
		wantName string
		wantSent model.Sentiment
	}{
		{0, "like", model.Neutral},
		{1, "heart", model.Positive},
		{2, "fire", model.Positive},
		{3, "haha", model.Positive},
		{4, "wow", model.Positive},
		{5, "sad", model.Negative},
		{6, "angry", model.Negative},
	}
	for _, c := range cases {
		name, s := reactionMeta(c.id)
		if name != c.wantName {
			t.Errorf("reactionMeta(%d) name = %q, want %q", c.id, name, c.wantName)
		}
		if s == nil || *s != c.wantSent {
			t.Errorf("reactionMeta(%d) sentiment = %v, want %q", c.id, s, c.wantSent)
		}
	}

	name, s := reactionMeta(99) // кастомная/неизвестная реакция
	if name != "" {
		t.Errorf("reactionMeta(unknown) name = %q, want empty", name)
	}
	if s == nil || *s != model.Neutral {
		t.Errorf("reactionMeta(unknown) sentiment = %v, want neutral", s)
	}
}
