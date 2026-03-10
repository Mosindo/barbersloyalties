package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/barbersloyalties/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type queryable interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type commandable interface {
	queryable
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Append(ctx context.Context, q commandable, input CreateInput) (Transaction, error) {
	if q == nil {
		q = r.pool
	}

	if input.MetadataJSON == nil {
		input.MetadataJSON = json.RawMessage("{}")
	}

	query := `
		INSERT INTO transactions (
			id, tenant_id, customer_id, type, payment_method, status,
			amount_cents, currency, platform_fee_cents,
			external_provider, external_reference, related_transaction_id,
			metadata_json, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, tenant_id, customer_id, type, payment_method, status,
			amount_cents, currency, platform_fee_cents,
			external_provider, external_reference, related_transaction_id,
			metadata_json, created_at
	`

	var out Transaction
	err := q.QueryRow(ctx, query,
		inputID(input),
		strings.TrimSpace(input.TenantID),
		input.CustomerID,
		strings.TrimSpace(input.Type),
		strings.TrimSpace(input.PaymentMethod),
		strings.TrimSpace(input.Status),
		input.AmountCents,
		normalizeCurrency(input.Currency),
		input.PlatformFeeCents,
		strings.TrimSpace(input.ExternalProvider),
		strings.TrimSpace(input.ExternalReference),
		input.RelatedTransactionID,
		input.MetadataJSON,
		input.CreatedAt,
	).Scan(
		&out.ID,
		&out.TenantID,
		&out.CustomerID,
		&out.Type,
		&out.PaymentMethod,
		&out.Status,
		&out.AmountCents,
		&out.Currency,
		&out.PlatformFeeCents,
		&out.ExternalProvider,
		&out.ExternalReference,
		&out.RelatedTransactionID,
		&out.MetadataJSON,
		&out.CreatedAt,
	)
	if err != nil {
		return Transaction{}, fmt.Errorf("append transaction: %w", err)
	}

	return out, nil
}

func (r *Repository) List(ctx context.Context, params ListParams) ([]Transaction, error) {
	query := `
		SELECT id, tenant_id, customer_id, type, payment_method, status,
			amount_cents, currency, platform_fee_cents,
			external_provider, external_reference, related_transaction_id,
			metadata_json, created_at
		FROM transactions
		WHERE tenant_id = $1
		  AND ($2 = '' OR customer_id::text = $2)
		  AND ($3 = '' OR type = $3)
		  AND ($4 = '' OR payment_method = $4)
		  AND ($5 = '' OR status = $5)
		ORDER BY created_at DESC
		LIMIT $6 OFFSET $7
	`

	rows, err := r.pool.Query(ctx, query,
		strings.TrimSpace(params.TenantID),
		strings.TrimSpace(params.CustomerID),
		strings.TrimSpace(params.Type),
		strings.TrimSpace(params.PaymentMethod),
		strings.TrimSpace(params.Status),
		params.Limit,
		params.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	items := make([]Transaction, 0, params.Limit)
	for rows.Next() {
		item, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transactions: %w", err)
	}

	return items, nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (Transaction, error) {
	query := `
		SELECT id, tenant_id, customer_id, type, payment_method, status,
			amount_cents, currency, platform_fee_cents,
			external_provider, external_reference, related_transaction_id,
			metadata_json, created_at
		FROM transactions
		WHERE tenant_id = $1 AND id = $2
	`

	row := r.pool.QueryRow(ctx, query, strings.TrimSpace(tenantID), strings.TrimSpace(id))
	var out Transaction
	err := row.Scan(
		&out.ID,
		&out.TenantID,
		&out.CustomerID,
		&out.Type,
		&out.PaymentMethod,
		&out.Status,
		&out.AmountCents,
		&out.Currency,
		&out.PlatformFeeCents,
		&out.ExternalProvider,
		&out.ExternalReference,
		&out.RelatedTransactionID,
		&out.MetadataJSON,
		&out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Transaction{}, domain.ErrNotFound
		}
		return Transaction{}, fmt.Errorf("get transaction by id: %w", err)
	}

	return out, nil
}

func scanTransaction(rows pgx.Rows) (Transaction, error) {
	var out Transaction
	if err := rows.Scan(
		&out.ID,
		&out.TenantID,
		&out.CustomerID,
		&out.Type,
		&out.PaymentMethod,
		&out.Status,
		&out.AmountCents,
		&out.Currency,
		&out.PlatformFeeCents,
		&out.ExternalProvider,
		&out.ExternalReference,
		&out.RelatedTransactionID,
		&out.MetadataJSON,
		&out.CreatedAt,
	); err != nil {
		return Transaction{}, fmt.Errorf("scan transaction: %w", err)
	}
	return out, nil
}
