package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/barbersloyalties/backend/internal/auth"
	"github.com/barbersloyalties/backend/internal/billing"
	"github.com/barbersloyalties/backend/internal/config"
	"github.com/barbersloyalties/backend/internal/customers"
	"github.com/barbersloyalties/backend/internal/database"
	"github.com/barbersloyalties/backend/internal/ledger"
	"github.com/barbersloyalties/backend/internal/loyalty"
	"github.com/barbersloyalties/backend/internal/middleware"
	"github.com/barbersloyalties/backend/internal/payments"
	"github.com/barbersloyalties/backend/internal/reporting"
	"github.com/barbersloyalties/backend/internal/subscriptions"
	"github.com/barbersloyalties/backend/internal/tenants"
	"github.com/barbersloyalties/backend/internal/users"
	"github.com/barbersloyalties/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	appLogger := logger.New(cfg.Env)

	ctx := context.Background()
	dbPool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		appLogger.Error("database connection failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer dbPool.Close()

	tenantRepo := tenants.NewPostgresRepository(dbPool)
	tenantService := tenants.NewService(tenantRepo)

	userRepo := users.NewPostgresRepository(dbPool)
	userService := users.NewService(userRepo)

	loyaltyService := loyalty.NewService(dbPool)

	tokenManager := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTExpiry)
	authService := auth.NewService(
		tenantService,
		userService,
		loyaltyService,
		tokenManager,
		cfg.DefaultStampThreshold,
		cfg.DefaultRewardValue,
	)
	authHandler := auth.NewHandler(authService)

	customerRepo := customers.NewPostgresRepository(dbPool)
	customerService := customers.NewService(customerRepo)
	customerHandler := customers.NewHandler(customerService)

	ledgerRepo := ledger.NewRepository(dbPool)
	ledgerService := ledger.NewService(ledgerRepo)
	ledgerHandler := ledger.NewHandler(ledgerService)

	subscriptionRepo := subscriptions.NewRepository(dbPool)
	subscriptionService := subscriptions.NewService(subscriptionRepo, int64(cfg.SaaSMonthlyPriceCents))

	var paymentsProvider payments.PaymentsProvider
	switch strings.ToLower(strings.TrimSpace(cfg.PaymentsProvider)) {
	case "fake":
		paymentsProvider = payments.NewFakeStripeProvider(cfg.AppBaseURL, cfg.StripePaymentsWebhookSecret)
	case "stripe":
		paymentsProvider = payments.NewStripeProvider(cfg.StripeSecretKey, cfg.StripePaymentsWebhookSecret, "")
	default:
		log.Fatalf("invalid PAYMENTS_PROVIDER: %s (expected fake or stripe)", cfg.PaymentsProvider)
	}

	paymentService := payments.NewService(dbPool, ledgerService, loyaltyService, paymentsProvider, payments.ServiceConfig{
		PlatformCommissionBPS: cfg.PlatformCommissionBPS,
	})
	paymentHandler := payments.NewHandler(paymentService)

	var billingProvider billing.BillingProvider
	switch strings.ToLower(strings.TrimSpace(cfg.BillingProvider)) {
	case "fake":
		billingProvider = billing.NewFakeStripeProvider(cfg.AppBaseURL, cfg.StripeWebhookSecret)
	case "stripe":
		billingProvider = billing.NewStripeProvider(cfg.StripeSecretKey, cfg.StripeWebhookSecret)
	default:
		log.Fatalf("invalid BILLING_PROVIDER: %s (expected fake or stripe)", cfg.BillingProvider)
	}

	billingService := billing.NewService(dbPool, subscriptionService, billingProvider, billing.ServiceConfig{
		SubscriptionPriceID: cfg.StripeSubscriptionPriceID,
		MobileSuccessURL:    cfg.MobileSuccessURL,
		MobileCancelURL:     cfg.MobileCancelURL,
		AppBaseURL:          cfg.AppBaseURL,
		PublishableKey:      cfg.StripePublishableKey,
	})
	billingHandler := billing.NewHandler(billingService)

	reportingService := reporting.NewService(dbPool)
	reportingHandler := reporting.NewHandler(reportingService)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(requestLogger(appLogger))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authGroup := router.Group("/auth")
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/logout", authHandler.Logout)

	protected := router.Group("/")
	protected.Use(middleware.AuthRequired(tokenManager))
	protected.GET("/me", authHandler.Me)
	protected.GET("/billing/subscription", billingHandler.GetSubscription)
	protected.POST("/billing/create-subscription-checkout", billingHandler.CreateSubscriptionCheckout)
	protected.POST("/billing/create-portal-session", billingHandler.CreatePortalSession)

	business := protected.Group("/")
	business.Use(middleware.RequireActiveSubscription(subscriptionService))
	business.GET("/customers", customerHandler.List)
	business.POST("/customers", customerHandler.Create)
	business.GET("/customers/:id", customerHandler.GetByID)
	business.PATCH("/customers/:id", customerHandler.Update)
	business.POST("/customers/:id/archive", customerHandler.Archive)
	business.POST("/customers/:id/pay-cash", paymentHandler.PayCash)
	business.POST("/customers/:id/create-stripe-checkout", paymentHandler.CreateStripeCheckout)
	business.POST("/customers/:id/redeem-reward", paymentHandler.RedeemReward)
	business.GET("/transactions", ledgerHandler.List)
	business.GET("/transactions/:id", ledgerHandler.GetByID)
	business.GET("/dashboard/summary", reportingHandler.DashboardSummary)

	router.POST("/webhooks/stripe/payments", paymentHandler.StripePaymentsWebhook)
	router.POST("/webhooks/stripe/billing", billingHandler.StripeBillingWebhook)

	if strings.ToLower(strings.TrimSpace(cfg.Env)) != "production" {
		if billingService.IsFakeProvider() {
			protected.POST("/dev/fake-stripe/billing/complete-checkout", billingHandler.DevFakeCompleteCheckout)
			protected.POST("/dev/fake-stripe/billing/payment-failed", billingHandler.DevFakePaymentFailed)
		}
		if paymentService.IsFakeProvider() {
			protected.POST("/dev/fake-stripe/payments/complete-checkout", paymentHandler.DevFakeCompleteCheckout)
			protected.POST("/dev/fake-stripe/payments/payment-failed", paymentHandler.DevFakePaymentFailed)
			protected.POST("/dev/fake-stripe/payments/refund", paymentHandler.DevFakeRefund)
		}
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		appLogger.Info("api listening", slog.String("port", cfg.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("api server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	waitForShutdown(appLogger, server)
}

func waitForShutdown(log *slog.Logger, server *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("shutdown failed", slog.String("error", err.Error()))
		return
	}
	log.Info("server stopped")
}

func requestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		log.Info("http_request",
			slog.String("request_id", middleware.GetRequestID(c)),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("duration", time.Since(start)),
		)
	}
}
