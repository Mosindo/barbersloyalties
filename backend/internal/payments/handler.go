package payments

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

type payCashRequest struct {
	AmountCents int64  `json:"amount_cents" binding:"required"`
	Currency    string `json:"currency"`
	Notes       string `json:"notes"`
}

type createStripeCheckoutRequest struct {
	AmountCents int64  `json:"amount_cents" binding:"required"`
	Currency    string `json:"currency"`
	SuccessURL  string `json:"success_url" binding:"required"`
	CancelURL   string `json:"cancel_url" binding:"required"`
	Description string `json:"description"`
	Notes       string `json:"notes"`
}

type redeemRewardRequest struct {
	Currency string `json:"currency"`
	Notes    string `json:"notes"`
}

type devFakeCheckoutWebhookRequest struct {
	CheckoutSessionID string `json:"checkout_session_id" binding:"required"`
	AmountCents       int64  `json:"amount_cents"`
	Currency          string `json:"currency"`
	PaymentIntentID   string `json:"payment_intent_id"`
}

type devFakeRefundWebhookRequest struct {
	CheckoutSessionID string `json:"checkout_session_id" binding:"required"`
	AmountCents       int64  `json:"amount_cents"`
	Currency          string `json:"currency"`
	PaymentIntentID   string `json:"payment_intent_id"`
	RefundID          string `json:"refund_id"`
}

func (h *Handler) PayCash(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	customerID := c.Param("id")

	var req payCashRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}

	result, err := h.service.RecordCashVisit(c.Request.Context(), RecordCashVisitInput{
		TenantID:    tenantID,
		CustomerID:  customerID,
		AmountCents: req.AmountCents,
		Currency:    req.Currency,
		Notes:       req.Notes,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusCreated, result)
}

func (h *Handler) CreateStripeCheckout(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	customerID := c.Param("id")

	var req createStripeCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}

	result, err := h.service.CreateStripeCheckout(c.Request.Context(), CreateStripeCheckoutInput{
		TenantID:    tenantID,
		CustomerID:  customerID,
		AmountCents: req.AmountCents,
		Currency:    req.Currency,
		SuccessURL:  req.SuccessURL,
		CancelURL:   req.CancelURL,
		Description: req.Description,
		Notes:       req.Notes,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusCreated, result)
}

func (h *Handler) RedeemReward(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	customerID := c.Param("id")

	var req redeemRewardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}

	result, err := h.service.RedeemReward(c.Request.Context(), RedeemRewardInput{
		TenantID:   tenantID,
		CustomerID: customerID,
		Currency:   req.Currency,
		Notes:      req.Notes,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusCreated, result)
}

func (h *Handler) StripePaymentsWebhook(c *gin.Context) {
	payload, err := c.GetRawData()
	if err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid webhook payload")
		return
	}

	signature := c.GetHeader("Stripe-Signature")
	result, err := h.service.HandleStripePaymentsWebhook(c.Request.Context(), payload, signature)
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

	var req devFakeCheckoutWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		httpx.JSONError(c, http.StatusUnauthorized, "unauthorized", "missing tenant context")
		return
	}

	paymentIntentID := req.PaymentIntentID
	if paymentIntentID == "" {
		paymentIntentID = newMockID("pi")
	}

	payload, err := buildFakeStripeEvent(stripeEventCheckoutCompleted, map[string]any{
		"object":         "checkout.session",
		"id":             req.CheckoutSessionID,
		"payment_status": "paid",
		"amount_total":   req.AmountCents,
		"currency":       req.Currency,
		"payment_intent": paymentIntentID,
		"metadata": map[string]string{
			"tenant_id": tenantID,
		},
	})
	if err != nil {
		httpx.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to build fake webhook payload")
		return
	}

	result, err := h.service.HandleStripePaymentsWebhook(c.Request.Context(), payload, fakeWebhookSignatureValue)
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, result)
}

func (h *Handler) DevFakePaymentFailed(c *gin.Context) {
	if !h.service.IsFakeProvider() {
		httpx.JSONError(c, http.StatusNotFound, "not_found", "fake stripe provider is not enabled")
		return
	}

	var req devFakeCheckoutWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		httpx.JSONError(c, http.StatusUnauthorized, "unauthorized", "missing tenant context")
		return
	}

	payload, err := buildFakeStripeEvent(stripeEventCheckoutAsyncFailed, map[string]any{
		"object":         "checkout.session",
		"id":             req.CheckoutSessionID,
		"payment_status": "unpaid",
		"amount_total":   req.AmountCents,
		"currency":       req.Currency,
		"payment_intent": req.PaymentIntentID,
		"metadata": map[string]string{
			"tenant_id": tenantID,
		},
	})
	if err != nil {
		httpx.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to build fake webhook payload")
		return
	}

	result, err := h.service.HandleStripePaymentsWebhook(c.Request.Context(), payload, fakeWebhookSignatureValue)
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, result)
}

func (h *Handler) DevFakeRefund(c *gin.Context) {
	if !h.service.IsFakeProvider() {
		httpx.JSONError(c, http.StatusNotFound, "not_found", "fake stripe provider is not enabled")
		return
	}

	var req devFakeRefundWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		httpx.JSONError(c, http.StatusUnauthorized, "unauthorized", "missing tenant context")
		return
	}

	refundID := req.RefundID
	if refundID == "" {
		refundID = newMockID("re")
	}
	paymentIntentID := req.PaymentIntentID
	if paymentIntentID == "" {
		paymentIntentID = newMockID("pi")
	}

	payload, err := buildFakeStripeEvent(stripeEventChargeRefunded, map[string]any{
		"object":          "charge",
		"id":              refundID,
		"amount_refunded": req.AmountCents,
		"currency":        req.Currency,
		"payment_intent":  paymentIntentID,
		"metadata": map[string]string{
			"tenant_id":         tenantID,
			"stripe_session_id": req.CheckoutSessionID,
		},
	})
	if err != nil {
		httpx.JSONError(c, http.StatusInternalServerError, "internal_error", "failed to build fake webhook payload")
		return
	}

	result, err := h.service.HandleStripePaymentsWebhook(c.Request.Context(), payload, fakeWebhookSignatureValue)
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
