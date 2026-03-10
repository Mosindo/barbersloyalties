package tenants

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, input CreateTenantInput) (Tenant, error) {
	businessName := strings.TrimSpace(input.BusinessName)
	ownerName := strings.TrimSpace(input.OwnerName)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	phone := strings.TrimSpace(input.Phone)

	if businessName == "" || ownerName == "" || email == "" {
		return Tenant{}, fmt.Errorf("business_name, owner_name and email are required")
	}

	now := time.Now().UTC()
	tenant := Tenant{
		ID:           uuid.NewString(),
		BusinessName: businessName,
		OwnerName:    ownerName,
		Email:        email,
		Phone:        phone,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	return s.repo.Create(ctx, tenant)
}

func (s *Service) GetByID(ctx context.Context, id string) (Tenant, error) {
	if strings.TrimSpace(id) == "" {
		return Tenant{}, fmt.Errorf("tenant id is required")
	}
	return s.repo.GetByID(ctx, id)
}
