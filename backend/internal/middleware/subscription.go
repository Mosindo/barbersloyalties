package middleware

import (
	"context"

	"github.com/barbersloyalties/backend/internal/httpx"
	"github.com/barbersloyalties/backend/internal/subscriptions"
	"github.com/gin-gonic/gin"
)

type subscriptionAccessChecker interface {
	HasActiveAccess(ctx context.Context, tenantID string) (bool, subscriptions.Subscription, error)
}

func RequireActiveSubscription(checker subscriptionAccessChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := GetTenantID(c)
		if tenantID == "" {
			httpx.JSONError(c, 401, "unauthorized", "missing tenant context")
			c.Abort()
			return
		}

		active, _, err := checker.HasActiveAccess(c.Request.Context(), tenantID)
		if err != nil {
			httpx.JSONError(c, 500, "internal_error", "failed to check subscription status")
			c.Abort()
			return
		}
		if !active {
			httpx.JSONError(c, 402, "subscription_required", "active subscription required")
			c.Abort()
			return
		}

		c.Next()
	}
}
