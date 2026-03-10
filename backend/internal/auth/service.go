package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/barbersloyalties/backend/internal/domain"
	"github.com/barbersloyalties/backend/internal/loyalty"
	"github.com/barbersloyalties/backend/internal/tenants"
	"github.com/barbersloyalties/backend/internal/users"
	"golang.org/x/crypto/bcrypt"
)

type RegisterInput struct {
	BusinessName string
	OwnerName    string
	Email        string
	Phone        string
	Password     string
}

type LoginInput struct {
	Email    string
	Password string
}

type AuthResult struct {
	Token  string         `json:"token"`
	User   users.User     `json:"user"`
	Tenant tenants.Tenant `json:"tenant"`
}

type Service struct {
	tenantService  *tenants.Service
	userService    *users.Service
	loyaltyService *loyalty.Service
	tokenManager   *TokenManager
	defaultStamp   int
	defaultReward  int
}

func NewService(
	tenantService *tenants.Service,
	userService *users.Service,
	loyaltyService *loyalty.Service,
	tokenManager *TokenManager,
	defaultStamp int,
	defaultReward int,
) *Service {
	return &Service{
		tenantService:  tenantService,
		userService:    userService,
		loyaltyService: loyaltyService,
		tokenManager:   tokenManager,
		defaultStamp:   defaultStamp,
		defaultReward:  defaultReward,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)
	if email == "" || password == "" || strings.TrimSpace(input.BusinessName) == "" || strings.TrimSpace(input.OwnerName) == "" {
		return AuthResult{}, fmt.Errorf("%w: missing required fields", domain.ErrInvalidRequest)
	}
	if len(password) < 8 {
		return AuthResult{}, fmt.Errorf("%w: password must be at least 8 characters", domain.ErrInvalidRequest)
	}

	existing, err := s.userService.GetByEmail(ctx, email)
	if err == nil && existing.ID != "" {
		return AuthResult{}, fmt.Errorf("%w: email already exists", domain.ErrConflict)
	}
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return AuthResult{}, fmt.Errorf("check existing user: %w", err)
	}

	hash, err := hashPassword(password)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hash password: %w", err)
	}

	tenant, err := s.tenantService.Create(ctx, tenants.CreateTenantInput{
		BusinessName: input.BusinessName,
		OwnerName:    input.OwnerName,
		Email:        email,
		Phone:        input.Phone,
	})
	if err != nil {
		return AuthResult{}, fmt.Errorf("create tenant: %w", err)
	}

	user, err := s.userService.Create(ctx, users.CreateUserInput{
		TenantID:     tenant.ID,
		Email:        email,
		PasswordHash: hash,
		Role:         users.RoleOwner,
	})
	if err != nil {
		return AuthResult{}, fmt.Errorf("create user: %w", err)
	}

	if err := s.loyaltyService.EnsureDefaultConfig(ctx, tenant.ID, s.defaultStamp, s.defaultReward); err != nil {
		return AuthResult{}, fmt.Errorf("create default loyalty config: %w", err)
	}

	token, err := s.tokenManager.Generate(UserIdentity{
		UserID:   user.ID,
		TenantID: user.TenantID,
		Email:    user.Email,
		Role:     user.Role,
	})
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate token: %w", err)
	}

	return AuthResult{Token: token, User: user, Tenant: tenant}, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)
	if email == "" || password == "" {
		return AuthResult{}, fmt.Errorf("%w: email and password are required", domain.ErrInvalidRequest)
	}

	user, err := s.userService.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return AuthResult{}, fmt.Errorf("%w: invalid credentials", domain.ErrUnauthorized)
		}
		return AuthResult{}, fmt.Errorf("get user by email: %w", err)
	}

	if err := comparePassword(user.PasswordHash, password); err != nil {
		return AuthResult{}, fmt.Errorf("%w: invalid credentials", domain.ErrUnauthorized)
	}

	tenant, err := s.tenantService.GetByID(ctx, user.TenantID)
	if err != nil {
		return AuthResult{}, fmt.Errorf("load tenant: %w", err)
	}

	token, err := s.tokenManager.Generate(UserIdentity{
		UserID:   user.ID,
		TenantID: user.TenantID,
		Email:    user.Email,
		Role:     user.Role,
	})
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate token: %w", err)
	}

	return AuthResult{Token: token, User: user, Tenant: tenant}, nil
}

func (s *Service) Me(ctx context.Context, userID, tenantID string) (AuthResult, error) {
	user, err := s.userService.GetByID(ctx, userID)
	if err != nil {
		return AuthResult{}, fmt.Errorf("get user: %w", err)
	}
	if user.TenantID != tenantID {
		return AuthResult{}, fmt.Errorf("%w: tenant mismatch", domain.ErrForbidden)
	}

	tenant, err := s.tenantService.GetByID(ctx, tenantID)
	if err != nil {
		return AuthResult{}, fmt.Errorf("get tenant: %w", err)
	}

	return AuthResult{User: user, Tenant: tenant}, nil
}

func hashPassword(raw string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func comparePassword(hashed, raw string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(raw))
}
