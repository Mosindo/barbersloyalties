package auth

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

type registerRequest struct {
	BusinessName string `json:"business_name" binding:"required"`
	OwnerName    string `json:"owner_name" binding:"required"`
	Email        string `json:"email" binding:"required,email"`
	Phone        string `json:"phone"`
	Password     string `json:"password" binding:"required"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}

	result, err := h.service.Register(c.Request.Context(), RegisterInput{
		BusinessName: req.BusinessName,
		OwnerName:    req.OwnerName,
		Email:        req.Email,
		Phone:        req.Phone,
		Password:     req.Password,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusCreated, result)
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}

	result, err := h.service.Login(c.Request.Context(), LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, result)
}

func (h *Handler) Logout(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func (h *Handler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	tenantID := middleware.GetTenantID(c)

	result, err := h.service.Me(c.Request.Context(), userID, tenantID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.JSONData(c, http.StatusOK, result)
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidRequest):
		httpx.JSONError(c, http.StatusBadRequest, "invalid_request", rootMessage(err))
	case errors.Is(err, domain.ErrConflict):
		httpx.JSONError(c, http.StatusConflict, "conflict", rootMessage(err))
	case errors.Is(err, domain.ErrUnauthorized):
		httpx.JSONError(c, http.StatusUnauthorized, "unauthorized", "invalid credentials")
	case errors.Is(err, domain.ErrForbidden):
		httpx.JSONError(c, http.StatusForbidden, "forbidden", "forbidden")
	default:
		httpx.JSONError(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func rootMessage(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprint(err)
}
