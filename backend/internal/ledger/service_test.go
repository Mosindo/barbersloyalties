package ledger

import (
	"errors"
	"testing"

	"github.com/barbersloyalties/backend/internal/domain"
)

func TestValidateCreateInputRequiresCoreFields(t *testing.T) {
	_, err := validateCreateInput(CreateInput{})
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestValidateCreateInputRejectsNegativeAmount(t *testing.T) {
	_, err := validateCreateInput(CreateInput{
		TenantID:      "tenant-1",
		Type:          TypeVisitPayment,
		PaymentMethod: PaymentMethodCash,
		Status:        StatusSucceeded,
		AmountCents:   -1,
	})
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("expected invalid request for negative amount, got %v", err)
	}
}

func TestValidateCreateInputSetsDefaults(t *testing.T) {
	input, err := validateCreateInput(CreateInput{
		TenantID:      "tenant-1",
		Type:          TypeVisitPayment,
		PaymentMethod: PaymentMethodCash,
		Status:        StatusSucceeded,
		AmountCents:   1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.Currency != DefaultCurrency {
		t.Fatalf("expected default currency %s, got %s", DefaultCurrency, input.Currency)
	}
	if input.MetadataJSON == nil {
		t.Fatal("expected metadata json default")
	}
	if input.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}
}
