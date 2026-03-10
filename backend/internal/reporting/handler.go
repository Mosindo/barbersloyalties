package reporting

import (
	"errors"
	"fmt"
	"net/http"

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

func (h *Handler) DashboardSummary(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	summary, err := h.service.DashboardSummary(c.Request.Context(), tenantID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, summary)
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidRequest):
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", fmt.Sprint(err))
	default:
		httpx.JSONError(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
