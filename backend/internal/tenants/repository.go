package tenants

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, tenant Tenant) (Tenant, error)
	GetByID(ctx context.Context, id string) (Tenant, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, tenant Tenant) (Tenant, error) {
	query := `
		INSERT INTO tenants (id, business_name, owner_name, email, phone, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, business_name, owner_name, email, phone, created_at, updated_at
	`

	var out Tenant
	err := r.pool.QueryRow(ctx, query,
		tenant.ID,
		tenant.BusinessName,
		tenant.OwnerName,
		tenant.Email,
		tenant.Phone,
		tenant.CreatedAt,
		tenant.UpdatedAt,
	).Scan(
		&out.ID,
		&out.BusinessName,
		&out.OwnerName,
		&out.Email,
		&out.Phone,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return Tenant{}, fmt.Errorf("insert tenant: %w", err)
	}
	return out, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (Tenant, error) {
	query := `
		SELECT id, business_name, owner_name, email, phone, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`
	var out Tenant
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&out.ID,
		&out.BusinessName,
		&out.OwnerName,
		&out.Email,
		&out.Phone,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return Tenant{}, fmt.Errorf("get tenant by id: %w", err)
	}
	return out, nil
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
