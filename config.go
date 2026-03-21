package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken        string
	MarzbanURL      string
	MarzbanUsername string
	MarzbanPassword string
	DatabaseURL     string
	AdminIDs        []int64
	Prices          map[string]int64 // В Telegram Stars
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	return &Config{
		BotToken:        getEnv("BOT_TOKEN", ""),
		MarzbanURL:      getEnv("MARZBAN_URL", "https://your-marzban.com"),
		MarzbanUsername: getEnv("MARZBAN_USERNAME", "admin"),
		MarzbanPassword: getEnv("MARZBAN_PASSWORD", ""),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://user:pass@localhost/vpn_bot?sslmode=disable"),
		Prices: map[string]int64{
			"1month":  100,
			"3months": 250,
			"6months": 450,
			"1year":   850,
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
