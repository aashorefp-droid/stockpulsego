package config

import (
	"os"
	"strings"
)

type Config struct {
	Port             string
	AlpacaAPIKey     string
	AlpacaAPISecret  string
	AlpacaDataBase   string
	PolygonAPIKey    string
	TelegramBotToken string
	TelegramChatID   string
	CORSOrigins      []string
	DBPath           string
}

func Load() *Config {
	cors := os.Getenv("CORS_ORIGINS")
	var origins []string
	if cors == "" {
		origins = []string{
			"http://localhost:3000", "http://localhost:3001",
			"http://localhost:3002", "http://localhost:3003",
		}
	} else {
		for _, s := range strings.Split(cors, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				origins = append(origins, s)
			}
		}
	}

	return &Config{
		Port:             getenv("PORT", "8000"),
		AlpacaAPIKey:     strings.TrimSpace(os.Getenv("ALPACA_API_KEY")),
		AlpacaAPISecret:  strings.TrimSpace(os.Getenv("ALPACA_API_SECRET")),
		AlpacaDataBase:   getenv("ALPACA_DATA_BASE", "https://data.alpaca.markets"),
		PolygonAPIKey:    strings.TrimSpace(os.Getenv("POLYGON_API_KEY")),
		TelegramBotToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChatID:   strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")),
		CORSOrigins:      origins,
		DBPath:           getenv("DB_PATH", "earnings_tracker.db"),
	}
}

func getenv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v != "" {
		return v
	}
	return fallback
}
