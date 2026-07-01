// Package config загружает конфигурацию сервиса из переменных окружения.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// defaultSearchKeywords — темы-затравки для groups.search на каждый город.
// Поиск ВК требует непустой q и матчит его по названию/описанию, а city_id
// сужает выдачу до города. Разные затравки достают разные пласты сообществ
// (сверх тех, что содержат само имя города), результаты дедуплицируются.
var defaultSearchKeywords = []string{
	"новости", "подслушано", "типичный", "барахолка", "объявления",
	"работа", "вакансии", "недвижимость", "авто", "дтп", "чп",
	"происшествия", "знакомства", "афиша", "туризм", "спорт",
	"бизнес", "мамы", "школа", "продам", "куплю", "услуги",
}

// loadDotenv подхватывает переменные из .env (а как запасной источник — из
// .env.example). Уже заданные переменные окружения имеют приоритет и не
// переопределяются, поэтому порядок: окружение → .env → .env.example.
func loadDotenv() {
	_ = godotenv.Load(".env")
}

// Config — параметры запуска сервиса.
type Config struct {
	// VKToken — сервисный токен доступа к API ВКонтакте.
	VKToken string
	// LogLevel — уровень логирования: debug | info | warn | error.
	LogLevel string
	// DatabaseURL — строка подключения к Postgres (общая с admin-panel).
	DatabaseURL string

	// Classifier — параметры модели классификации тональности.
	// Если LLMBaseURL пуст, используется встроенный словарный классификатор.
	LLMBaseURL string
	LLMModel   string
	LLMAPIKey  string

	// Сбор данных.
	RegionName         string    // название региона для фильтрации сообществ
	SearchKeywords     []string  // доп. поисковые запросы на город (сверх имени города)
	CollectSince       time.Time // нижняя граница даты публикаций
	MaxPostsPerGroup   int       // 0 — без ограничения
	MaxCommentsPerPost int       // 0 — без ограничения
	MaxCommunities     int       // 0 — без ограничения (для smoke-прогона)
	VKRateLimit        int       // запросов в секунду к API ВК

	// SkipExistingCommunities — если true, сообщество, уже сохранённое в БД,
	// пропускается целиком (без захода в посты/комментарии). Ускоряет повторные
	// прогоны ценой того, что новые посты в известных сообществах не подхватятся.
	SkipExistingCommunities bool
}

// Load читает конфигурацию из окружения, подставляя значения по умолчанию.
func Load() Config {
	loadDotenv()
	return Config{
		VKToken:     getEnv("VK_TOKEN", ""),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		DatabaseURL: getEnv("DATABASE_URL", ""),

		LLMBaseURL: getEnv("LLM_BASE_URL", ""),
		LLMModel:   getEnv("LLM_MODEL", ""),
		LLMAPIKey:  getEnv("LLM_API_KEY", ""),

		RegionName:         getEnv("REGION_NAME", "Чувашская Республика"),
		SearchKeywords:     getEnvList("SEARCH_KEYWORDS", defaultSearchKeywords),
		CollectSince:       getEnvDate("COLLECT_SINCE", startOfYear()),
		MaxPostsPerGroup:   getEnvInt("MAX_POSTS_PER_GROUP", 100),
		MaxCommentsPerPost: getEnvInt("MAX_COMMENTS_PER_POST", 100),
		MaxCommunities:     getEnvInt("MAX_COMMUNITIES", 0),
		VKRateLimit:        getEnvInt("VK_RATE_LIMIT", 3),

		SkipExistingCommunities: getEnvBool("SKIP_EXISTING_COMMUNITIES", false),
	}
}

// Validate проверяет обязательные параметры.
func (c Config) Validate() error {
	if c.VKToken == "" {
		return fmt.Errorf("VK_TOKEN is required")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	return nil
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// getEnvList читает список значений, разделённых запятыми. Пустые элементы и
// пробелы по краям отбрасываются. Пустая/незаданная переменная — значение по
// умолчанию; явный список полностью заменяет дефолт.
func getEnvList(key string, def []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	out := make([]string, 0)
	for part := range strings.SplitSeq(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

// getEnvDate читает дату в формате YYYY-MM-DD.
func getEnvDate(key string, def time.Time) time.Time {
	if v, ok := os.LookupEnv(key); ok {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			return t
		}
	}
	return def
}

// startOfYear — 1 января текущего года (UTC).
func startOfYear() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
}
