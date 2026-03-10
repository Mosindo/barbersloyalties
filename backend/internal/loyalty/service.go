package loyalty

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	ID             string
	TenantID       string
	StampThreshold int
	RewardValue    int
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) EnsureDefaultConfig(ctx context.Context, tenantID string, stampThreshold, rewardValue int) error {
	query := `
		SELECT id
		FROM loyalty_configs
		WHERE tenant_id = $1 AND is_active = true
		LIMIT 1
	`

	var existing string
	err := s.pool.QueryRow(ctx, query, tenantID).Scan(&existing)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check active loyalty config: %w", err)
	}

	// If there is no active config, create one.
	insert := `
		INSERT INTO loyalty_configs (id, tenant_id, stamp_threshold, reward_value, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, $5, $6)
	`
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, insert, uuid.NewString(), tenantID, stampThreshold, rewardValue, now, now)
	if err != nil {
		return fmt.Errorf("insert default loyalty config: %w", err)
	}

	return nil
}
