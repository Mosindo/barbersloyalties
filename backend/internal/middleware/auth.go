package middleware

import (
	"strings"

	"github.com/barbersloyalties/backend/internal/httpx"
	"github.com/barbersloyalties/backend/internal/identity"
	"github.com/gin-gonic/gin"
)

const (
	ctxUserIDKey   = "user_id"
	ctxTenantIDKey = "tenant_id"
	ctxRoleKey     = "role"
	ctxEmailKey    = "email"
)

type tokenParser interface {
	Parse(rawToken string) (*identity.Claims, error)
}

func AuthRequired(parser tokenParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			httpx.JSONError(c, 401, "unauthorized", "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			httpx.JSONError(c, 401, "unauthorized", "invalid authorization header")
			c.Abort()
			return
		}

		claims, err := parser.Parse(parts[1])
		if err != nil {
			httpx.JSONError(c, 401, "unauthorized", "invalid token")
			c.Abort()
			return
		}

		c.Set(ctxUserIDKey, claims.UserID)
		c.Set(ctxTenantIDKey, claims.TenantID)
		c.Set(ctxRoleKey, claims.Role)
		c.Set(ctxEmailKey, claims.Email)
		c.Next()
	}
}

func GetTenantID(c *gin.Context) string {
	value, _ := c.Get(ctxTenantIDKey)
	tenantID, _ := value.(string)
	return tenantID
}

func GetUserID(c *gin.Context) string {
	value, _ := c.Get(ctxUserIDKey)
	userID, _ := value.(string)
	return userID
}

func GetRole(c *gin.Context) string {
	value, _ := c.Get(ctxRoleKey)
	role, _ := value.(string)
	return role
}

func GetEmail(c *gin.Context) string {
	value, _ := c.Get(ctxEmailKey)
	email, _ := value.(string)
	return email
}
