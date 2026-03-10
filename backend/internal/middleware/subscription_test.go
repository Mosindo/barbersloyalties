package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/barbersloyalties/backend/internal/subscriptions"
	"github.com/gin-gonic/gin"
)

type fakeSubscriptionChecker struct {
	active bool
	err    error
}

func (f fakeSubscriptionChecker) HasActiveAccess(_ context.Context, _ string) (bool, subscriptions.Subscription, error) {
	return f.active, subscriptions.Subscription{}, f.err
}

func TestRequireActiveSubscription_NoTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequireActiveSubscription(fakeSubscriptionChecker{active: true}))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireActiveSubscription_Inactive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("tenant_id", "tenant-1")
		c.Next()
	})
	r.Use(RequireActiveSubscription(fakeSubscriptionChecker{active: false}))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", w.Code)
	}
}

func TestRequireActiveSubscription_Active(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("tenant_id", "tenant-1")
		c.Next()
	})
	r.Use(RequireActiveSubscription(fakeSubscriptionChecker{active: true}))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}
