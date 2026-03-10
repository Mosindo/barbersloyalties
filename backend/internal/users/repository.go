package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/barbersloyalties/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, user User) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, user User) (User, error) {
	query := `
		INSERT INTO users (id, tenant_id, email, password_hash, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, tenant_id, email, password_hash, role, created_at, updated_at
	`

	var out User
	err := r.pool.QueryRow(ctx, query,
		user.ID,
		user.TenantID,
		user.Email,
		user.PasswordHash,
		user.Role,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(
		&out.ID,
		&out.TenantID,
		&out.Email,
		&out.PasswordHash,
		&out.Role,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	return out, nil
}

func (r *PostgresRepository) GetByEmail(ctx context.Context, email string) (User, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, role, created_at, updated_at
		FROM users
		WHERE lower(email) = lower($1)
	`

	var out User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&out.ID,
		&out.TenantID,
		&out.Email,
		&out.PasswordHash,
		&out.Role,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, domain.ErrNotFound
		}
		return User{}, fmt.Errorf("get user by email: %w", err)
	}
	return out, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (User, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, role, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var out User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&out.ID,
		&out.TenantID,
		&out.Email,
		&out.PasswordHash,
		&out.Role,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, domain.ErrNotFound
		}
		return User{}, fmt.Errorf("get user by id: %w", err)
	}
	return out, nil
}
