package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/plombir1917/vk-loyal-users-parser/internal/classifier"
	"github.com/plombir1917/vk-loyal-users-parser/internal/core/config"
	"github.com/plombir1917/vk-loyal-users-parser/internal/core/logger"
	"github.com/plombir1917/vk-loyal-users-parser/internal/service"
	"github.com/plombir1917/vk-loyal-users-parser/internal/storage"
	"github.com/plombir1917/vk-loyal-users-parser/internal/vk"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.LogLevel)

	// Контекст, отменяемый по SIGINT/SIGTERM, — для корректного завершения.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("service starting")

	if err := run(ctx, cfg, log); err != nil {
		log.Error("service stopped with error", "err", err)
		os.Exit(1)
	}

	log.Info("service stopped")
}

// run собирает слои (хранилище, VK-клиент, классификатор, сервис) и запускает
// пайплайн сбора и сегментации.
func run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	store, err := storage.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	vkClient := vk.New(cfg.VKToken, cfg.VKRateLimit, log)
	cls := newClassifier(cfg, log)

	svc := service.New(vkClient, cls, store, service.Config{
		RegionName:         cfg.RegionName,
		SearchKeywords:     cfg.SearchKeywords,
		CollectSince:       cfg.CollectSince,
		MaxPostsPerGroup:   cfg.MaxPostsPerGroup,
		MaxCommentsPerPost: cfg.MaxCommentsPerPost,
		MaxCommunities:     cfg.MaxCommunities,

		ClassifyConcurrency: cfg.ClassifyConcurrency,
		LikeThreshold:       cfg.LikeThreshold,
		CommentThreshold:    cfg.CommentThreshold,
		CoreCombineOr:       cfg.CoreCombineOr,

		SkipExistingCommunities: cfg.SkipExistingCommunities,
		ReparseExisting:         cfg.ReparseExisting,
	}, log)

	return svc.Run(ctx)
}

// newClassifier выбирает реализацию: если задан LLM_BASE_URL — реальная модель
// (LLM) как основной классификатор со словарным fallback на случай сбоя или
// недоступности модели; иначе только встроенный словарный классификатор.
func newClassifier(cfg config.Config, log *slog.Logger) classifier.Classifier {
	lexicon := classifier.NewLexicon()
	if cfg.LLMBaseURL != "" {
		log.Info("classifier: llm + lexicon fallback", "model", cfg.LLMModel)
		llm := classifier.NewLLM(classifier.LLMConfig{
			BaseURL: cfg.LLMBaseURL,
			Model:   cfg.LLMModel,
			APIKey:  cfg.LLMAPIKey,
		})
		return classifier.NewFallback(llm, lexicon, log)
	}
	log.Info("classifier: lexicon (LLM не настроена)")
	return lexicon
}
