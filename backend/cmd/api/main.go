package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/barbersloyalties/backend/internal/auth"
	"github.com/barbersloyalties/backend/internal/config"
	"github.com/barbersloyalties/backend/internal/customers"
	"github.com/barbersloyalties/backend/internal/database"
	"github.com/barbersloyalties/backend/internal/loyalty"
	"github.com/barbersloyalties/backend/internal/middleware"
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

	protected.GET("/customers", customerHandler.List)
	protected.POST("/customers", customerHandler.Create)
	protected.GET("/customers/:id", customerHandler.GetByID)
	protected.PATCH("/customers/:id", customerHandler.Update)
	protected.POST("/customers/:id/archive", customerHandler.Archive)

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
