package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/barbersloyalties/backend/internal/auth"
	"github.com/barbersloyalties/backend/internal/billing"
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
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFakeHappyPathEndToEnd(t *testing.T) {
	t.Parallel()

	baseDSN := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if baseDSN == "" {
		baseDSN = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if baseDSN == "" {
		t.Skip("TEST_DATABASE_URL or DATABASE_URL is required for integration test")
	}

	schema := "it_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	dsn := withSearchPath(baseDSN, schema)

	ctx := context.Background()
	pool, err := database.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))
	}()

	applyMigration(t, pool, "0001_init.up.sql")
	applyMigration(t, pool, "0002_subscriptions_tenant_unique.up.sql")

	router := buildIntegrationRouter(t, pool)

	registerBody := map[string]any{
		"business_name": "Barber Fake Flow",
		"owner_name":    "Owner One",
		"email":         fmt.Sprintf("owner-%d@example.com", time.Now().UTC().UnixNano()),
		"phone":         "+33123456789",
		"password":      "password123",
	}
	registerResp := performJSON(t, router, http.MethodPost, "/auth/register", "", registerBody)
	if registerResp.StatusCode != http.StatusCreated {
		t.Fatalf("register expected 201, got %d body=%s", registerResp.StatusCode, string(registerResp.Body))
	}

	loginBody := map[string]any{
		"email":    registerBody["email"],
		"password": registerBody["password"],
	}
	loginResp := performJSON(t, router, http.MethodPost, "/auth/login", "", loginBody)
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login expected 200, got %d body=%s", loginResp.StatusCode, string(loginResp.Body))
	}

	var loginData struct {
		Token string `json:"token"`
	}
	decodeData(t, loginResp.Body, &loginData)
	if strings.TrimSpace(loginData.Token) == "" {
		t.Fatalf("missing token in login response")
	}
	authHeader := "Bearer " + loginData.Token

	// Subscription gating should block business endpoints while inactive.
	blockedResp := performJSON(t, router, http.MethodGet, "/customers", authHeader, nil)
	if blockedResp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("customers should be gated with inactive subscription, got %d body=%s", blockedResp.StatusCode, string(blockedResp.Body))
	}

	// Must work in fake mode even without a real Stripe price id.
	checkoutResp := performJSON(t, router, http.MethodPost, "/billing/create-subscription-checkout", authHeader, map[string]any{})
	if checkoutResp.StatusCode != http.StatusCreated {
		t.Fatalf("create subscription checkout expected 201, got %d body=%s", checkoutResp.StatusCode, string(checkoutResp.Body))
	}
	var checkoutData struct {
		CheckoutSessionID string `json:"checkout_session_id"`
	}
	decodeData(t, checkoutResp.Body, &checkoutData)
	if !strings.HasPrefix(checkoutData.CheckoutSessionID, "cs_test_mock_") {
		t.Fatalf("expected fake checkout id, got %s", checkoutData.CheckoutSessionID)
	}

	activateResp := performJSON(t, router, http.MethodPost, "/dev/fake-stripe/billing/complete-checkout", authHeader, map[string]any{
		"checkout_session_id": checkoutData.CheckoutSessionID,
	})
	if activateResp.StatusCode != http.StatusOK {
		t.Fatalf("fake billing activation expected 200, got %d body=%s", activateResp.StatusCode, string(activateResp.Body))
	}

	customerResp := performJSON(t, router, http.MethodPost, "/customers", authHeader, map[string]any{
		"full_name": "Client One",
		"phone":     "+33999999999",
	})
	if customerResp.StatusCode != http.StatusCreated {
		t.Fatalf("create customer expected 201, got %d body=%s", customerResp.StatusCode, string(customerResp.Body))
	}
	var customerData struct {
		ID string `json:"id"`
	}
	decodeData(t, customerResp.Body, &customerData)
	if customerData.ID == "" {
		t.Fatal("missing customer id")
	}

	cashResp := performJSON(t, router, http.MethodPost, "/customers/"+customerData.ID+"/pay-cash", authHeader, map[string]any{
		"amount_cents": 1500,
		"currency":     "EUR",
		"notes":        "cash visit",
	})
	if cashResp.StatusCode != http.StatusCreated {
		t.Fatalf("pay cash expected 201, got %d body=%s", cashResp.StatusCode, string(cashResp.Body))
	}

	fakeStripeCheckoutResp := performJSON(t, router, http.MethodPost, "/customers/"+customerData.ID+"/create-stripe-checkout", authHeader, map[string]any{
		"amount_cents": 2000,
		"currency":     "EUR",
		"success_url":  "barbers://success",
		"cancel_url":   "barbers://cancel",
		"description":  "Fake stripe visit",
	})
	if fakeStripeCheckoutResp.StatusCode != http.StatusCreated {
		t.Fatalf("create fake stripe checkout expected 201, got %d body=%s", fakeStripeCheckoutResp.StatusCode, string(fakeStripeCheckoutResp.Body))
	}
	var fakeStripeCheckoutData struct {
		CheckoutSessionID string `json:"checkout_session_id"`
	}
	decodeData(t, fakeStripeCheckoutResp.Body, &fakeStripeCheckoutData)

	completeStripeResp := performJSON(t, router, http.MethodPost, "/dev/fake-stripe/payments/complete-checkout", authHeader, map[string]any{
		"checkout_session_id": fakeStripeCheckoutData.CheckoutSessionID,
		"amount_cents":        2000,
		"currency":            "EUR",
	})
	if completeStripeResp.StatusCode != http.StatusOK {
		t.Fatalf("complete fake stripe payment expected 200, got %d body=%s", completeStripeResp.StatusCode, string(completeStripeResp.Body))
	}

	redeemResp := performJSON(t, router, http.MethodPost, "/customers/"+customerData.ID+"/redeem-reward", authHeader, map[string]any{
		"currency": "EUR",
		"notes":    "free cut redemption",
	})
	if redeemResp.StatusCode != http.StatusCreated {
		t.Fatalf("redeem reward expected 201, got %d body=%s", redeemResp.StatusCode, string(redeemResp.Body))
	}

	txResp := performJSON(t, router, http.MethodGet, "/transactions?customer_id="+customerData.ID, authHeader, nil)
	if txResp.StatusCode != http.StatusOK {
		t.Fatalf("transactions list expected 200, got %d body=%s", txResp.StatusCode, string(txResp.Body))
	}
	var txs []struct {
		Type          string `json:"type"`
		Status        string `json:"status"`
		PaymentMethod string `json:"payment_method"`
	}
	decodeData(t, txResp.Body, &txs)
	if len(txs) < 3 {
		t.Fatalf("expected at least 3 transactions, got %d", len(txs))
	}
	foundRedemption := false
	for _, tx := range txs {
		if tx.Type == ledger.TypeRewardRedemption && tx.Status == ledger.StatusSucceeded && tx.PaymentMethod == ledger.PaymentMethodManual {
			foundRedemption = true
			break
		}
	}
	if !foundRedemption {
		t.Fatalf("expected reward_redemption transaction in ledger list")
	}

	var totalTx int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM transactions`).Scan(&totalTx); err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	if totalTx < 3 {
		t.Fatalf("expected append-only ledger with >=3 entries, got %d", totalTx)
	}

	var paidVisits, availableRewards, usedRewards int
	if err := pool.QueryRow(ctx, `
		SELECT total_paid_visits, available_rewards, used_rewards
		FROM customer_loyalty_states
		WHERE customer_id = $1
	`, customerData.ID).Scan(&paidVisits, &availableRewards, &usedRewards); err != nil {
		t.Fatalf("read loyalty projection: %v", err)
	}
	if paidVisits != 2 {
		t.Fatalf("expected total_paid_visits=2, got %d", paidVisits)
	}
	if availableRewards != 0 || usedRewards != 1 {
		t.Fatalf("expected rewards after redemption available=0 used=1, got available=%d used=%d", availableRewards, usedRewards)
	}

	dashboardResp := performJSON(t, router, http.MethodGet, "/dashboard/summary", authHeader, nil)
	if dashboardResp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard summary expected 200, got %d body=%s", dashboardResp.StatusCode, string(dashboardResp.Body))
	}
	var summary struct {
		CustomersCount     int64 `json:"customers_count"`
		PaidVisitsCount    int64 `json:"paid_visits_count"`
		CashRevenueCents   int64 `json:"cash_revenue_cents"`
		StripeRevenueCents int64 `json:"stripe_revenue_cents"`
		AvailableRewards   int64 `json:"available_rewards_total"`
		UsedRewards        int64 `json:"used_rewards_total"`
	}
	decodeData(t, dashboardResp.Body, &summary)
	if summary.CustomersCount != 1 || summary.PaidVisitsCount != 2 || summary.CashRevenueCents != 1500 || summary.StripeRevenueCents != 2000 {
		t.Fatalf("unexpected dashboard summary: %+v (available_rewards=%d)", summary, availableRewards)
	}
	if summary.AvailableRewards != 0 || summary.UsedRewards != 1 {
		t.Fatalf("unexpected reward totals in dashboard: %+v", summary)
	}
}

type httpResult struct {
	StatusCode int
	Body       []byte
}

func performJSON(t *testing.T, router *gin.Engine, method, path, authHeader string, body any) httpResult {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(authHeader) != "" {
		req.Header.Set("Authorization", authHeader)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return httpResult{StatusCode: w.Code, Body: w.Body.Bytes()}
}

func decodeData(t *testing.T, body []byte, out any) {
	t.Helper()

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode envelope: %v body=%s", err, string(body))
	}
	if len(envelope.Data) == 0 {
		t.Fatalf("missing data in response: %s", string(body))
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		t.Fatalf("decode data payload: %v body=%s", err, string(body))
	}
}

func applyMigration(t *testing.T, pool *pgxpool.Pool, filename string) {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve current file path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	path := filepath.Join(root, "migrations", filename)

	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", filename, err)
	}
	if _, err := pool.Exec(context.Background(), string(sqlBytes)); err != nil {
		t.Fatalf("apply migration %s: %v", filename, err)
	}
}

func withSearchPath(dsn, schema string) string {
	if strings.Contains(dsn, "search_path=") {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "search_path=" + schema
}

func buildIntegrationRouter(t *testing.T, dbPool *pgxpool.Pool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tenantRepo := tenants.NewPostgresRepository(dbPool)
	tenantService := tenants.NewService(tenantRepo)
	userRepo := users.NewPostgresRepository(dbPool)
	userService := users.NewService(userRepo)
	loyaltyService := loyalty.NewService(dbPool)

	tokenManager := auth.NewTokenManager("integration-secret", "barbersloyalties", 24*time.Hour)
	authService := auth.NewService(tenantService, userService, loyaltyService, tokenManager, 2, 1)
	authHandler := auth.NewHandler(authService)

	customerRepo := customers.NewPostgresRepository(dbPool)
	customerService := customers.NewService(customerRepo)
	customerHandler := customers.NewHandler(customerService)

	ledgerRepo := ledger.NewRepository(dbPool)
	ledgerService := ledger.NewService(ledgerRepo)
	ledgerHandler := ledger.NewHandler(ledgerService)

	subscriptionRepo := subscriptions.NewRepository(dbPool)
	subscriptionService := subscriptions.NewService(subscriptionRepo, 900)

	paymentProvider := payments.NewFakeStripeProvider("https://fake.local", "")
	paymentService := payments.NewService(dbPool, ledgerService, loyaltyService, paymentProvider, payments.ServiceConfig{
		PlatformCommissionBPS: 500,
	})
	paymentHandler := payments.NewHandler(paymentService)

	billingProvider := billing.NewFakeStripeProvider("https://fake.local", "")
	billingService := billing.NewService(dbPool, subscriptionService, billingProvider, billing.ServiceConfig{
		SubscriptionPriceID: "",
		MobileSuccessURL:    "barbers://success",
		MobileCancelURL:     "barbers://cancel",
		AppBaseURL:          "http://localhost",
		PublishableKey:      "pk_test_fake",
	})
	billingHandler := billing.NewHandler(billingService)

	reportingService := reporting.NewService(dbPool)
	reportingHandler := reporting.NewHandler(reportingService)

	router := gin.New()
	router.Use(gin.Recovery())

	authGroup := router.Group("/auth")
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", authHandler.Login)

	protected := router.Group("/")
	protected.Use(middleware.AuthRequired(tokenManager))
	protected.GET("/me", authHandler.Me)
	protected.POST("/billing/create-subscription-checkout", billingHandler.CreateSubscriptionCheckout)
	protected.GET("/billing/subscription", billingHandler.GetSubscription)
	protected.POST("/dev/fake-stripe/billing/complete-checkout", billingHandler.DevFakeCompleteCheckout)
	protected.POST("/dev/fake-stripe/payments/complete-checkout", paymentHandler.DevFakeCompleteCheckout)

	business := protected.Group("/")
	business.Use(middleware.RequireActiveSubscription(subscriptionService))
	business.GET("/customers", customerHandler.List)
	business.POST("/customers", customerHandler.Create)
	business.POST("/customers/:id/pay-cash", paymentHandler.PayCash)
	business.POST("/customers/:id/create-stripe-checkout", paymentHandler.CreateStripeCheckout)
	business.POST("/customers/:id/redeem-reward", paymentHandler.RedeemReward)
	business.GET("/transactions", ledgerHandler.List)
	business.GET("/dashboard/summary", reportingHandler.DashboardSummary)

	return router
}
