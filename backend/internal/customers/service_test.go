package customers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/barbersloyalties/backend/internal/domain"
)

type stubRepo struct {
	created  Customer
	existing Customer
}

func (s *stubRepo) Create(_ context.Context, customer Customer) (Customer, error) {
	s.created = customer
	return customer, nil
}

func (s *stubRepo) List(_ context.Context, _ ListParams) ([]Customer, error) {
	return []Customer{}, nil
}

func (s *stubRepo) GetByID(_ context.Context, tenantID, customerID string) (Customer, error) {
	if s.existing.ID == customerID && s.existing.TenantID == tenantID {
		return s.existing, nil
	}
	return Customer{}, domain.ErrNotFound
}

func (s *stubRepo) Update(_ context.Context, customer Customer) (Customer, error) {
	s.existing = customer
	return customer, nil
}

func (s *stubRepo) Archive(_ context.Context, _, _ string) (Customer, error) {
	return Customer{}, nil
}

func TestCreateRequiresTenantAndName(t *testing.T) {
	svc := NewService(&stubRepo{})

	_, err := svc.Create(context.Background(), CreateInput{})
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestCreateSuccess(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo)

	customer, err := svc.Create(context.Background(), CreateInput{
		TenantID: "tenant-1",
		FullName: "Ada Barber",
		Phone:    "+33123456789",
		Notes:    "VIP",
	})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}

	if customer.ID == "" {
		t.Fatal("expected generated id")
	}
	if repo.created.FullName != "Ada Barber" {
		t.Fatalf("expected persisted name Ada Barber, got %s", repo.created.FullName)
	}
}

func TestUpdateRejectsEmptyName(t *testing.T) {
	repo := &stubRepo{existing: Customer{
		ID:        "cust-1",
		TenantID:  "tenant-1",
		FullName:  "Existing Name",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}}
	svc := NewService(repo)
	empty := "   "

	_, err := svc.Update(context.Background(), UpdateInput{
		TenantID: "tenant-1",
		ID:       "cust-1",
		FullName: &empty,
	})
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("expected invalid request for empty name, got %v", err)
	}
}
