package billing

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/barbersloyalties/backend/internal/domain"
	"github.com/barbersloyalties/backend/internal/httpx"
	"github.com/barbersloyalties/backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type createSubscriptionCheckoutRequest struct {
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

type createPortalSessionRequest struct {
	ReturnURL string `json:"return_url"`
}

type devFakeBillingCheckoutRequest struct {
	CheckoutSessionID    string `json:"checkout_session_id"`
	StripeCustomerID     string `json:"stripe_customer_id"`
	StripeSubscriptionID string `json:"stripe_subscription_id"`
}

type devFakeBillingPaymentFailedRequest struct {
	StripeCustomerID     string `json:"stripe_customer_id"`
	StripeSubscriptionID string `json:"stripe_subscription_id"`
}

func (h *Handler) CreateSubscriptionCheckout(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)

	var req createSubscriptionCheckoutRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
			return
		}
	}

	result, err := h.service.CreateSubscriptionCheckout(c.Request.Context(), CreateSubscriptionCheckoutInput{
		TenantID:   tenantID,
		SuccessURL: req.SuccessURL,
		CancelURL:  req.CancelURL,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusCreated, result)
}

func (h *Handler) GetSubscription(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	result, err := h.service.GetSubscription(c.Request.Context(), tenantID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	httpx.JSONData(c, http.StatusOK, result)
}

func (h *Handler) CreatePortalSession(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)

	var req createPortalSessionRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
			return
		}
	}

	result, err := h.service.CreatePortalSession(c.Request.Context(), CreatePortalSessionInput{
		TenantID:  tenantID,
		ReturnURL: req.ReturnURL,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusCreated, result)
}

func (h *Handler) StripeBillingWebhook(c *gin.Context) {
	payload, err := c.GetRawData()
	if err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid webhook payload")
		return
	}

	signature := c.GetHeader("Stripe-Signature")
	result, err := h.service.HandleStripeBillingWebhook(c.Request.Context(), payload, signature)
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, result)
}

func (h *Handler) DevFakeCompleteCheckout(c *gin.Context) {
	if !h.service.IsFakeProvider() {
		httpx.JSONError(c, http.StatusNotFound, "not_found", "fake stripe provider is not enabled")
		return
	}

	var req devFakeBillingCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		httpx.JSONError(c, http.StatusUnauthorized, "unauthorized", "missing tenant context")
		return
	}

	checkoutSessionID := req.CheckoutSessionID
	if checkoutSessionID == "" {
		checkoutSessionID = newMockID("cs_test")
	}

	customerID := req.StripeCustomerID
	if customerID == "" {
		customerID = newMockID("cus")
	}

	subscriptionID := req.StripeSubscriptionID
	if subscriptionID == "" {
		subscriptionID = newMockID("sub")
	}

	checkoutPayload, err := buildFakeStripeEvent("checkout.session.completed", map[string]any{
		"object":              "checkout.session",
		"id":                  checkoutSessionID,
		"mode":                "subscription",
		"client_reference_id": tenantID,
		"metadata": map[string]string{
			"tenant_id": tenantID,
		},
		"customer":     customerID,
		"subscription": subscriptionID,
	})
	if err != nil {
		httpx.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to build fake checkout payload")
		return
	}

	checkoutResult, err := h.service.HandleStripeBillingWebhook(c.Request.Context(), checkoutPayload, fakeWebhookSignatureValue)
	if err != nil {
		h.handleError(c, err)
		return
	}

	periodEnd := time.Now().UTC().Add(30 * 24 * time.Hour).Unix()
	subscriptionPayload, err := buildFakeStripeEvent("customer.subscription.updated", map[string]any{
		"object":             "subscription",
		"id":                 subscriptionID,
		"status":             "active",
		"current_period_end": periodEnd,
		"metadata": map[string]string{
			"tenant_id": tenantID,
		},
		"customer": customerID,
	})
	if err != nil {
		httpx.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to build fake subscription payload")
		return
	}

	subscriptionResult, err := h.service.HandleStripeBillingWebhook(c.Request.Context(), subscriptionPayload, fakeWebhookSignatureValue)
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, gin.H{
		"checkout_session_id":    checkoutSessionID,
		"stripe_customer_id":     customerID,
		"stripe_subscription_id": subscriptionID,
		"events": []WebhookResult{
			checkoutResult,
			subscriptionResult,
		},
	})
}

func (h *Handler) DevFakePaymentFailed(c *gin.Context) {
	if !h.service.IsFakeProvider() {
		httpx.JSONError(c, http.StatusNotFound, "not_found", "fake stripe provider is not enabled")
		return
	}

	var req devFakeBillingPaymentFailedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		httpx.JSONError(c, http.StatusUnauthorized, "unauthorized", "missing tenant context")
		return
	}

	customerID := req.StripeCustomerID
	if customerID == "" {
		customerID = newMockID("cus")
	}
	subscriptionID := req.StripeSubscriptionID
	if subscriptionID == "" {
		subscriptionID = newMockID("sub")
	}

	payload, err := buildFakeStripeEvent("customer.subscription.updated", map[string]any{
		"object":             "subscription",
		"id":                 subscriptionID,
		"status":             "past_due",
		"current_period_end": time.Now().UTC().Add(30 * 24 * time.Hour).Unix(),
		"metadata": map[string]string{
			"tenant_id": tenantID,
		},
		"customer": customerID,
	})
	if err != nil {
		httpx.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to build fake payment-failed payload")
		return
	}

	result, err := h.service.HandleStripeBillingWebhook(c.Request.Context(), payload, fakeWebhookSignatureValue)
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, result)
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidRequest):
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", fmt.Sprint(err))
	case errors.Is(err, domain.ErrNotFound):
		httpx.JSONError(c, http.StatusNotFound, "not_found", "resource not found")
	default:
		httpx.JSONError(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func buildFakeStripeEvent(eventType string, object map[string]any) ([]byte, error) {
	return json.Marshal(map[string]any{
		"id":               newMockEventID(),
		"object":           "event",
		"api_version":      "2026-02-25.clover",
		"created":          time.Now().UTC().Unix(),
		"livemode":         false,
		"pending_webhooks": 1,
		"request": map[string]any{
			"id":              nil,
			"idempotency_key": nil,
		},
		"type": eventType,
		"data": map[string]any{
			"object": object,
		},
	})
}
