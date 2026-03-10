package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/barbersloyalties/backend/internal/domain"
	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Append(ctx context.Context, q commandable, input CreateInput) (Transaction, error) {
	validated, err := validateCreateInput(input)
	if err != nil {
		return Transaction{}, err
	}
	return s.repo.Append(ctx, q, validated)
}

func (s *Service) List(ctx context.Context, params ListParams) ([]Transaction, error) {
	params.TenantID = strings.TrimSpace(params.TenantID)
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

func (s *Service) GetByID(ctx context.Context, tenantID, id string) (Transaction, error) {
	tenantID = strings.TrimSpace(tenantID)
	id = strings.TrimSpace(id)
	if tenantID == "" || id == "" {
		return Transaction{}, fmt.Errorf("%w: tenant_id and transaction id are required", domain.ErrInvalidRequest)
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

func validateCreateInput(input CreateInput) (CreateInput, error) {
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.Type = strings.TrimSpace(input.Type)
	input.PaymentMethod = strings.TrimSpace(input.PaymentMethod)
	input.Status = strings.TrimSpace(input.Status)
	input.Currency = normalizeCurrency(input.Currency)
	input.ExternalProvider = strings.TrimSpace(input.ExternalProvider)
	input.ExternalReference = strings.TrimSpace(input.ExternalReference)
	if input.MetadataJSON == nil {
		input.MetadataJSON = json.RawMessage("{}")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}

	if input.TenantID == "" || input.Type == "" || input.PaymentMethod == "" || input.Status == "" {
		return CreateInput{}, fmt.Errorf("%w: tenant_id, type, payment_method and status are required", domain.ErrInvalidRequest)
	}
	if input.AmountCents < 0 {
		return CreateInput{}, fmt.Errorf("%w: amount_cents cannot be negative", domain.ErrInvalidRequest)
	}

	return input, nil
}

func inputID(_ CreateInput) string {
	return uuid.NewString()
}

func normalizeCurrency(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return DefaultCurrency
	}
	return strings.ToUpper(trimmed)
}
