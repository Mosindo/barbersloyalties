package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/barbersloyalties/backend/internal/domain"
	"github.com/jackc/pgx/v5"
)

type subscriptionQueryable interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Service struct {
	repo              *Repository
	defaultMonthlyCts int64
}

func NewService(repo *Repository, defaultMonthlyPriceCents int64) *Service {
	return &Service{repo: repo, defaultMonthlyCts: defaultMonthlyPriceCents}
}

func (s *Service) GetOrCreateByTenant(ctx context.Context, q subscriptionQueryable, tenantID string) (Subscription, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return Subscription{}, fmt.Errorf("%w: tenant_id is required", domain.ErrInvalidRequest)
	}

	sub, err := s.repo.GetByTenantID(ctx, q, tenantID)
	if err == nil {
		return sub, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return Subscription{}, err
	}

	return s.repo.UpsertByTenant(ctx, q, UpsertByTenantInput{
		TenantID:             tenantID,
		StripeCustomerID:     "",
		StripeSubscriptionID: "",
		Status:               StatusInactive,
		MonthlyPriceCents:    s.defaultMonthlyCts,
		CurrentPeriodEnd:     nil,
	})
}

func (s *Service) UpsertByTenant(ctx context.Context, q subscriptionQueryable, input UpsertByTenantInput) (Subscription, error) {
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.Status = strings.TrimSpace(input.Status)
	if input.TenantID == "" {
		return Subscription{}, fmt.Errorf("%w: tenant_id is required", domain.ErrInvalidRequest)
	}
	if input.Status == "" {
		input.Status = StatusInactive
	}
	if input.MonthlyPriceCents <= 0 {
		input.MonthlyPriceCents = s.defaultMonthlyCts
	}
	if input.MonthlyPriceCents <= 0 {
		return Subscription{}, fmt.Errorf("%w: monthly price must be positive", domain.ErrInvalidRequest)
	}

	return s.repo.UpsertByTenant(ctx, q, input)
}

func (s *Service) UpdateByStripeRefs(ctx context.Context, q subscriptionQueryable, input UpdateByStripeRefsInput) (Subscription, error) {
	input.Status = strings.TrimSpace(input.Status)
	input.StripeSubscriptionID = strings.TrimSpace(input.StripeSubscriptionID)
	input.StripeCustomerID = strings.TrimSpace(input.StripeCustomerID)
	if input.Status == "" {
		return Subscription{}, fmt.Errorf("%w: status is required", domain.ErrInvalidRequest)
	}
	if input.StripeSubscriptionID == "" && input.StripeCustomerID == "" {
		return Subscription{}, fmt.Errorf("%w: stripe subscription id or customer id is required", domain.ErrInvalidRequest)
	}

	return s.repo.UpdateByStripeRefs(ctx, q, input)
}

func (s *Service) HasActiveAccess(ctx context.Context, tenantID string) (bool, Subscription, error) {
	sub, err := s.GetOrCreateByTenant(ctx, nil, tenantID)
	if err != nil {
		return false, Subscription{}, err
	}
	return isActiveStatus(sub.Status), sub, nil
}

func isActiveStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case StatusActive, StatusTrialing:
		return true
	default:
		return false
	}
}
