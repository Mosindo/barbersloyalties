package ledger

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

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

func (h *Handler) List(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	limit := parseIntOrDefault(c.Query("limit"), 20)
	offset := parseIntOrDefault(c.Query("offset"), 0)

	items, err := h.service.List(c.Request.Context(), ListParams{
		TenantID:      tenantID,
		CustomerID:    c.Query("customer_id"),
		Type:          c.Query("type"),
		PaymentMethod: c.Query("payment_method"),
		Status:        c.Query("status"),
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, items)
}

func (h *Handler) GetByID(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	txID := c.Param("id")

	tx, err := h.service.GetByID(c.Request.Context(), tenantID, txID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, tx)
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidRequest):
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", fmt.Sprint(err))
	case errors.Is(err, domain.ErrNotFound):
		httpx.JSONError(c, http.StatusNotFound, "not_found", "transaction not found")
	default:
		httpx.JSONError(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func parseIntOrDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
