package users

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

func (s *Service) Create(ctx context.Context, input CreateUserInput) (User, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	passwordHash := strings.TrimSpace(input.PasswordHash)
	role := strings.TrimSpace(input.Role)
	if role == "" {
		role = RoleOwner
	}

	if tenantID == "" || email == "" || passwordHash == "" {
		return User{}, fmt.Errorf("tenant_id, email and password are required")
	}

	now := time.Now().UTC()
	user := User{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	return s.repo.Create(ctx, user)
}

func (s *Service) GetByEmail(ctx context.Context, email string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return User{}, fmt.Errorf("email is required")
	}
	return s.repo.GetByEmail(ctx, email)
}

func (s *Service) GetByID(ctx context.Context, id string) (User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return User{}, fmt.Errorf("user id is required")
	}
	return s.repo.GetByID(ctx, id)
}
