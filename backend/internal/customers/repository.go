package customers

import (
	"context"
	"errors"
	"fmt"

	"github.com/barbersloyalties/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, customer Customer) (Customer, error)
	List(ctx context.Context, params ListParams) ([]Customer, error)
	GetByID(ctx context.Context, tenantID, customerID string) (Customer, error)
	Update(ctx context.Context, customer Customer) (Customer, error)
	Archive(ctx context.Context, tenantID, customerID string) (Customer, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, customer Customer) (Customer, error) {
	query := `
		INSERT INTO customers (id, tenant_id, full_name, phone, notes, is_archived, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, full_name, phone, notes, is_archived, created_at, updated_at
	`

	var out Customer
	err := r.pool.QueryRow(ctx, query,
		customer.ID,
		customer.TenantID,
		customer.FullName,
		customer.Phone,
		customer.Notes,
		customer.IsArchived,
		customer.CreatedAt,
		customer.UpdatedAt,
	).Scan(
		&out.ID,
		&out.TenantID,
		&out.FullName,
		&out.Phone,
		&out.Notes,
		&out.IsArchived,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return Customer{}, fmt.Errorf("insert customer: %w", err)
	}
	return out, nil
}

func (r *PostgresRepository) List(ctx context.Context, params ListParams) ([]Customer, error) {
	query := `
		SELECT id, tenant_id, full_name, phone, notes, is_archived, created_at, updated_at
		FROM customers
		WHERE tenant_id = $1
		  AND (
			$2 = ''
			OR lower(full_name) LIKE '%' || lower($2) || '%'
			OR phone LIKE '%' || $2 || '%'
		  )
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.pool.Query(ctx, query, params.TenantID, params.Search, params.Limit, params.Offset)
	if err != nil {
		return nil, fmt.Errorf("list customers: %w", err)
	}
	defer rows.Close()

	customers := make([]Customer, 0, params.Limit)
	for rows.Next() {
		var item Customer
		if err := rows.Scan(
			&item.ID,
			&item.TenantID,
			&item.FullName,
			&item.Phone,
			&item.Notes,
			&item.IsArchived,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan customer: %w", err)
		}
		customers = append(customers, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate customers: %w", err)
	}

	return customers, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, tenantID, customerID string) (Customer, error) {
	query := `
		SELECT id, tenant_id, full_name, phone, notes, is_archived, created_at, updated_at
		FROM customers
		WHERE tenant_id = $1 AND id = $2
	`

	var out Customer
	err := r.pool.QueryRow(ctx, query, tenantID, customerID).Scan(
		&out.ID,
		&out.TenantID,
		&out.FullName,
		&out.Phone,
		&out.Notes,
		&out.IsArchived,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Customer{}, domain.ErrNotFound
		}
		return Customer{}, fmt.Errorf("get customer by id: %w", err)
	}

	return out, nil
}

func (r *PostgresRepository) Update(ctx context.Context, customer Customer) (Customer, error) {
	query := `
		UPDATE customers
		SET full_name = $3,
			phone = $4,
			notes = $5,
			updated_at = $6
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, full_name, phone, notes, is_archived, created_at, updated_at
	`

	var out Customer
	err := r.pool.QueryRow(ctx, query,
		customer.TenantID,
		customer.ID,
		customer.FullName,
		customer.Phone,
		customer.Notes,
		customer.UpdatedAt,
	).Scan(
		&out.ID,
		&out.TenantID,
		&out.FullName,
		&out.Phone,
		&out.Notes,
		&out.IsArchived,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Customer{}, domain.ErrNotFound
		}
		return Customer{}, fmt.Errorf("update customer: %w", err)
	}
	return out, nil
}

func (r *PostgresRepository) Archive(ctx context.Context, tenantID, customerID string) (Customer, error) {
	query := `
		UPDATE customers
		SET is_archived = true,
			updated_at = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, full_name, phone, notes, is_archived, created_at, updated_at
	`

	var out Customer
	err := r.pool.QueryRow(ctx, query, tenantID, customerID).Scan(
		&out.ID,
		&out.TenantID,
		&out.FullName,
		&out.Phone,
		&out.Notes,
		&out.IsArchived,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Customer{}, domain.ErrNotFound
		}
		return Customer{}, fmt.Errorf("archive customer: %w", err)
	}

	return out, nil
}
