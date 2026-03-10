package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	Env                  string
	Port                 string
	DatabaseURL          string
	JWTSecret            string
	JWTIssuer            string
	JWTExpiry            time.Duration
	PlatformCommissionBPS int
	SaaSMonthlyPriceCents int
	DefaultStampThreshold int
	DefaultRewardValue    int
}

func Load() (Config, error) {
	cfg := Config{
		Env:                   getEnv("APP_ENV", "development"),
		Port:                  getEnv("PORT", "8080"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		JWTIssuer:             getEnv("JWT_ISSUER", "barbersloyalties"),
		PlatformCommissionBPS: getEnvInt("PLATFORM_COMMISSION_BPS", 500),
		SaaSMonthlyPriceCents: getEnvInt("SAAS_MONTHLY_PRICE_CENTS", 900),
		DefaultStampThreshold: getEnvInt("DEFAULT_STAMP_THRESHOLD", 10),
		DefaultRewardValue:    getEnvInt("DEFAULT_REWARD_VALUE", 1),
	}

	expiryMinutes := getEnvInt("JWT_EXPIRY_MINUTES", 1440)
	cfg.JWTExpiry = time.Duration(expiryMinutes) * time.Minute

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
