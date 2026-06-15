package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/plombir1917/vk-loyal-users-parser/internal/model"
)

// LLM — классификатор поверх OpenAI-совместимого chat/completions API
// (подходит Ollama, llama.cpp, OpenRouter и т.п.). Модель меняется через env.
type LLM struct {
	baseURL string
	model   string
	apiKey  string
	http    *http.Client
}

// LLMConfig — параметры подключения к модели.
type LLMConfig struct {
	BaseURL string
	Model   string
	APIKey  string
}

// NewLLM создаёт LLM-классификатор.
func NewLLM(cfg LLMConfig) *LLM {
	return &LLM{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		model:   cfg.Model,
		apiKey:  cfg.APIKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

const systemPrompt = "Ты классификатор тональности комментариев на русском языке. " +
	"Ответь строго одним словом: positive, negative или neutral. Без пояснений."

type chatRequest struct {
	Model       string        `json:"model"`
	Temperature float64       `json:"temperature"`
	Messages    []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Classify отправляет текст модели и нормализует ответ. При любой ошибке
// возвращает neutral, чтобы пайплайн не прерывался из-за одного комментария.
func (l *LLM) Classify(ctx context.Context, text string) (model.Sentiment, error) {
	body, err := json.Marshal(chatRequest{
		Model:       l.model,
		Temperature: 0,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: text},
		},
	})
	if err != nil {
		return model.Neutral, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return model.Neutral, err
	}
	req.Header.Set("Content-Type", "application/json")
	if l.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+l.apiKey)
	}

	resp, err := l.http.Do(req)
	if err != nil {
		return model.Neutral, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.Neutral, fmt.Errorf("llm status %d", resp.StatusCode)
	}

	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return model.Neutral, err
	}
	if len(parsed.Choices) == 0 {
		return model.Neutral, fmt.Errorf("llm: пустой ответ")
	}
	return normalize(parsed.Choices[0].Message.Content), nil
}
