package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	Env                         string
	Port                        string
	DatabaseURL                 string
	JWTSecret                   string
	JWTIssuer                   string
	JWTExpiry                   time.Duration
	PlatformCommissionBPS       int
	SaaSMonthlyPriceCents       int
	DefaultStampThreshold       int
	DefaultRewardValue          int
	StripeSecretKey             string
	PaymentsProvider            string
	BillingProvider             string
	StripePublishableKey        string
	StripeWebhookSecret         string
	StripeSubscriptionPriceID   string
	AppBaseURL                  string
	MobileSuccessURL            string
	MobileCancelURL             string
	StripePaymentsWebhookSecret string
}

func Load() (Config, error) {
	cfg := Config{
		Env:                         getEnv("APP_ENV", "development"),
		Port:                        getEnv("PORT", "8080"),
		DatabaseURL:                 os.Getenv("DATABASE_URL"),
		JWTSecret:                   os.Getenv("JWT_SECRET"),
		JWTIssuer:                   getEnv("JWT_ISSUER", "barbersloyalties"),
		PlatformCommissionBPS:       getEnvInt("PLATFORM_COMMISSION_BPS", 500),
		SaaSMonthlyPriceCents:       getEnvInt("SAAS_MONTHLY_PRICE_CENTS", 900),
		DefaultStampThreshold:       getEnvInt("DEFAULT_STAMP_THRESHOLD", 10),
		DefaultRewardValue:          getEnvInt("DEFAULT_REWARD_VALUE", 1),
		StripeSecretKey:             os.Getenv("STRIPE_SECRET_KEY"),
		PaymentsProvider:            getEnv("PAYMENTS_PROVIDER", getEnv("PAYMENT_PROVIDER", "fake")),
		BillingProvider:             getEnv("BILLING_PROVIDER", "fake"),
		StripePublishableKey:        os.Getenv("STRIPE_PUBLISHABLE_KEY"),
		StripeWebhookSecret:         os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripeSubscriptionPriceID:   os.Getenv("STRIPE_SUBSCRIPTION_PRICE_ID"),
		AppBaseURL:                  getEnv("APP_BASE_URL", ""),
		MobileSuccessURL:            getEnv("MOBILE_SUCCESS_URL", ""),
		MobileCancelURL:             getEnv("MOBILE_CANCEL_URL", ""),
		StripePaymentsWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET_PAYMENTS", os.Getenv("STRIPE_WEBHOOK_SECRET")),
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
