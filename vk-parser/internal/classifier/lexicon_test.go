package classifier

import (
	"context"
	"testing"

	"github.com/plombir1917/vk-loyal-users-parser/internal/model"
)

func TestLexiconClassify(t *testing.T) {
	c := NewLexicon()
	cases := []struct {
		name string
		text string
		want model.Sentiment
	}{
		{"positive", "Спасибо, отличная работа, супер!", model.Positive},
		{"negative", "Это просто ужас и позор, обман людей", model.Negative},
		{"neutral", "Сегодня в городе прошло собрание", model.Neutral},
		{"empty", "", model.Neutral},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := c.Classify(context.Background(), tc.text)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Classify(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]model.Sentiment{
		"positive":      model.Positive,
		"NEGATIVE":      model.Negative,
		"нейтрально":    model.Neutral,
		"Позитивный":    model.Positive,
		"что-то другое": model.Neutral,
	}
	for in, want := range cases {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
