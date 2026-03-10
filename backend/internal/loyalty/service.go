package loyalty

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/barbersloyalties/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type queryable interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type commandable interface {
	queryable
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Config struct {
	ID             string
	TenantID       string
	StampThreshold int
	RewardValue    int
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type State struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenant_id"`
	CustomerID       string     `json:"customer_id"`
	StampsCount      int        `json:"stamps_count"`
	AvailableRewards int        `json:"available_rewards"`
	UsedRewards      int        `json:"used_rewards"`
	TotalPaidVisits  int        `json:"total_paid_visits"`
	LastVisitAt      *time.Time `json:"last_visit_at,omitempty"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) EnsureDefaultConfig(ctx context.Context, tenantID string, stampThreshold, rewardValue int) error {
	_, err := s.getActiveConfig(ctx, s.pool, tenantID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

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

func (s *Service) ApplyPaidVisit(ctx context.Context, q commandable, tenantID, customerID string, visitAt time.Time) (State, bool, error) {
	tenantID = strings.TrimSpace(tenantID)
	customerID = strings.TrimSpace(customerID)
	if tenantID == "" || customerID == "" {
		return State{}, false, fmt.Errorf("%w: tenant_id and customer_id are required", domain.ErrInvalidRequest)
	}
	if q == nil {
		q = s.pool
	}
	if visitAt.IsZero() {
		visitAt = time.Now().UTC()
	}

	cfg, err := s.getActiveConfig(ctx, q, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return State{}, false, fmt.Errorf("%w: active loyalty config not found", domain.ErrInvalidRequest)
		}
		return State{}, false, err
	}
	if cfg.StampThreshold <= 0 || cfg.RewardValue <= 0 {
		return State{}, false, fmt.Errorf("%w: loyalty configuration is invalid", domain.ErrInvalidRequest)
	}

	if err := s.ensureStateRow(ctx, q, tenantID, customerID, visitAt); err != nil {
		return State{}, false, err
	}

	state, err := s.getStateForUpdate(ctx, q, tenantID, customerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return State{}, false, fmt.Errorf("loyalty state row missing after ensure: %v", err)
		}
		return State{}, false, err
	}

	nextStamps, nextRewards, nextTotalVisits, rewardUnlocked, err := ApplyStampCard(
		state.StampsCount,
		state.AvailableRewards,
		state.TotalPaidVisits,
		cfg.StampThreshold,
		cfg.RewardValue,
	)
	if err != nil {
		return State{}, false, fmt.Errorf("%w: loyalty rule failed", domain.ErrInvalidRequest)
	}
	state.StampsCount = nextStamps
	state.AvailableRewards = nextRewards
	state.TotalPaidVisits = nextTotalVisits
	state.LastVisitAt = &visitAt
	state.UpdatedAt = visitAt

	updated, err := s.updateState(ctx, q, state)
	if err != nil {
		return State{}, false, err
	}

	return updated, rewardUnlocked, nil
}

func (s *Service) RedeemReward(ctx context.Context, q commandable, tenantID, customerID string, redeemedAt time.Time) (State, error) {
	tenantID = strings.TrimSpace(tenantID)
	customerID = strings.TrimSpace(customerID)
	if tenantID == "" || customerID == "" {
		return State{}, fmt.Errorf("%w: tenant_id and customer_id are required", domain.ErrInvalidRequest)
	}
	if q == nil {
		q = s.pool
	}
	if redeemedAt.IsZero() {
		redeemedAt = time.Now().UTC()
	}

	if err := s.ensureStateRow(ctx, q, tenantID, customerID, redeemedAt); err != nil {
		return State{}, err
	}

	state, err := s.getStateForUpdate(ctx, q, tenantID, customerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return State{}, fmt.Errorf("loyalty state row missing after ensure: %v", err)
		}
		return State{}, err
	}
	if state.AvailableRewards <= 0 {
		return State{}, fmt.Errorf("%w: no rewards available for redemption", domain.ErrInvalidRequest)
	}

	state.AvailableRewards--
	state.UsedRewards++
	state.UpdatedAt = redeemedAt

	updated, err := s.updateState(ctx, q, state)
	if err != nil {
		return State{}, err
	}
	return updated, nil
}

func (s *Service) ensureStateRow(ctx context.Context, q commandable, tenantID, customerID string, now time.Time) error {
	insert := `
		INSERT INTO customer_loyalty_states (
			id, tenant_id, customer_id, stamps_count, available_rewards,
			used_rewards, total_paid_visits, updated_at
		)
		VALUES ($1, $2, $3, 0, 0, 0, 0, $4)
		ON CONFLICT (tenant_id, customer_id) DO NOTHING
	`

	_, err := q.Exec(ctx, insert, uuid.NewString(), tenantID, customerID, now)
	if err != nil {
		return fmt.Errorf("ensure loyalty state row: %w", err)
	}
	return nil
}

func (s *Service) getStateForUpdate(ctx context.Context, q queryable, tenantID, customerID string) (State, error) {
	query := `
		SELECT id, tenant_id, customer_id, stamps_count, available_rewards,
			used_rewards, total_paid_visits, last_visit_at, updated_at
		FROM customer_loyalty_states
		WHERE tenant_id = $1 AND customer_id = $2
		FOR UPDATE
	`

	var out State
	err := q.QueryRow(ctx, query, tenantID, customerID).Scan(
		&out.ID,
		&out.TenantID,
		&out.CustomerID,
		&out.StampsCount,
		&out.AvailableRewards,
		&out.UsedRewards,
		&out.TotalPaidVisits,
		&out.LastVisitAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return State{}, domain.ErrNotFound
		}
		return State{}, fmt.Errorf("get loyalty state for update: %w", err)
	}

	return out, nil
}

func (s *Service) updateState(ctx context.Context, q queryable, state State) (State, error) {
	query := `
		UPDATE customer_loyalty_states
		SET stamps_count = $3,
			available_rewards = $4,
			used_rewards = $5,
			total_paid_visits = $6,
			last_visit_at = $7,
			updated_at = $8
		WHERE tenant_id = $1 AND customer_id = $2
		RETURNING id, tenant_id, customer_id, stamps_count, available_rewards,
			used_rewards, total_paid_visits, last_visit_at, updated_at
	`

	var out State
	err := q.QueryRow(ctx, query,
		state.TenantID,
		state.CustomerID,
		state.StampsCount,
		state.AvailableRewards,
		state.UsedRewards,
		state.TotalPaidVisits,
		state.LastVisitAt,
		state.UpdatedAt,
	).Scan(
		&out.ID,
		&out.TenantID,
		&out.CustomerID,
		&out.StampsCount,
		&out.AvailableRewards,
		&out.UsedRewards,
		&out.TotalPaidVisits,
		&out.LastVisitAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return State{}, fmt.Errorf("update loyalty state: %w", err)
	}

	return out, nil
}

func (s *Service) getActiveConfig(ctx context.Context, q queryable, tenantID string) (Config, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return Config{}, fmt.Errorf("%w: tenant_id is required", domain.ErrInvalidRequest)
	}

	query := `
		SELECT id, tenant_id, stamp_threshold, reward_value, is_active, created_at, updated_at
		FROM loyalty_configs
		WHERE tenant_id = $1 AND is_active = true
		ORDER BY updated_at DESC
		LIMIT 1
	`

	var out Config
	err := q.QueryRow(ctx, query, tenantID).Scan(
		&out.ID,
		&out.TenantID,
		&out.StampThreshold,
		&out.RewardValue,
		&out.IsActive,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Config{}, domain.ErrNotFound
		}
		return Config{}, fmt.Errorf("get active loyalty config: %w", err)
	}

	return out, nil
}
