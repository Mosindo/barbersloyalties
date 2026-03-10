package customers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/barbersloyalties/backend/internal/domain"
	"github.com/google/uuid"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Customer, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	fullName := strings.TrimSpace(input.FullName)
	phone := strings.TrimSpace(input.Phone)
	notes := strings.TrimSpace(input.Notes)

	if tenantID == "" || fullName == "" {
		return Customer{}, fmt.Errorf("%w: tenant_id and full_name are required", domain.ErrInvalidRequest)
	}

	now := time.Now().UTC()
	customer := Customer{
		ID:         uuid.NewString(),
		TenantID:   tenantID,
		FullName:   fullName,
		Phone:      phone,
		Notes:      notes,
		IsArchived: false,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	return s.repo.Create(ctx, customer)
}

func (s *Service) List(ctx context.Context, params ListParams) ([]Customer, error) {
	params.TenantID = strings.TrimSpace(params.TenantID)
	params.Search = strings.TrimSpace(params.Search)
	if params.TenantID == "" {
		return nil, fmt.Errorf("%w: tenant_id is required", domain.ErrInvalidRequest)
	}

	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 20
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	return s.repo.List(ctx, params)
}

func (s *Service) GetByID(ctx context.Context, tenantID, id string) (Customer, error) {
	tenantID = strings.TrimSpace(tenantID)
	id = strings.TrimSpace(id)
	if tenantID == "" || id == "" {
		return Customer{}, fmt.Errorf("%w: tenant_id and customer id are required", domain.ErrInvalidRequest)
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (Customer, error) {
	existing, err := s.GetByID(ctx, input.TenantID, input.ID)
	if err != nil {
		return Customer{}, err
	}

	if input.FullName != nil {
		value := strings.TrimSpace(*input.FullName)
		if value == "" {
			return Customer{}, fmt.Errorf("%w: full_name cannot be empty", domain.ErrInvalidRequest)
		}
		existing.FullName = value
	}
	if input.Phone != nil {
		existing.Phone = strings.TrimSpace(*input.Phone)
	}
	if input.Notes != nil {
		existing.Notes = strings.TrimSpace(*input.Notes)
	}

	existing.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, existing)
}

func (s *Service) Archive(ctx context.Context, tenantID, id string) (Customer, error) {
	tenantID = strings.TrimSpace(tenantID)
	id = strings.TrimSpace(id)
	if tenantID == "" || id == "" {
		return Customer{}, fmt.Errorf("%w: tenant_id and customer id are required", domain.ErrInvalidRequest)
	}
	return s.repo.Archive(ctx, tenantID, id)
}
