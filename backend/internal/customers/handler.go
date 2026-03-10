package customers

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

type createCustomerRequest struct {
	FullName string `json:"full_name" binding:"required"`
	Phone    string `json:"phone"`
	Notes    string `json:"notes"`
}

type updateCustomerRequest struct {
	FullName *string `json:"full_name"`
	Phone    *string `json:"phone"`
	Notes    *string `json:"notes"`
}

func (h *Handler) Create(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)

	var req createCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}

	customer, err := h.service.Create(c.Request.Context(), CreateInput{
		TenantID: tenantID,
		FullName: req.FullName,
		Phone:    req.Phone,
		Notes:    req.Notes,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusCreated, customer)
}

func (h *Handler) List(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)

	limit := parseIntOrDefault(c.Query("limit"), 20)
	offset := parseIntOrDefault(c.Query("offset"), 0)

	customers, err := h.service.List(c.Request.Context(), ListParams{
		TenantID: tenantID,
		Search:   c.Query("search"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, customers)
}

func (h *Handler) GetByID(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	customerID := c.Param("id")

	customer, err := h.service.GetByID(c.Request.Context(), tenantID, customerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, customer)
}

func (h *Handler) Update(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	customerID := c.Param("id")

	var req updateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}

	customer, err := h.service.Update(c.Request.Context(), UpdateInput{
		TenantID: tenantID,
		ID:       customerID,
		FullName: req.FullName,
		Phone:    req.Phone,
		Notes:    req.Notes,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, customer)
}

func (h *Handler) Archive(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	customerID := c.Param("id")

	customer, err := h.service.Archive(c.Request.Context(), tenantID, customerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, customer)
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidRequest):
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", fmt.Sprint(err))
	case errors.Is(err, domain.ErrNotFound):
		httpx.JSONError(c, http.StatusNotFound, "not_found", "customer not found")
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
