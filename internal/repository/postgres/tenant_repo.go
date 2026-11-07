package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/relayforge/relayforge/internal/domain"
	"github.com/relayforge/relayforge/internal/repository"
)

type TenantRepo struct {
	db *DB
}

func NewTenantRepo(db *DB) repository.TenantRepository {
	return &TenantRepo{db: db}
}

func (r *TenantRepo) CreateTenant(ctx context.Context, name, slug, plan string) (*domain.Tenant, error) {
	query := `
		INSERT INTO tenants (name, slug, plan)
		VALUES ($1, $2, $3)
		RETURNING id, name, slug, plan, created_at, updated_at
	`
	t := &domain.Tenant{}
	err := r.db.Pool.QueryRow(ctx, query, name, slug, plan).Scan(
		&t.ID, &t.Name, &t.Slug, &t.Plan, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}
	return t, nil
}

func (r *TenantRepo) GetTenantByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	query := `SELECT id, name, slug, plan, created_at, updated_at FROM tenants WHERE id = $1 LIMIT 1`
	t := &domain.Tenant{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&t.ID, &t.Name, &t.Slug, &t.Plan, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get tenant by id: %w", err)
	}
	return t, nil
}

func (r *TenantRepo) GetTenantBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	query := `SELECT id, name, slug, plan, created_at, updated_at FROM tenants WHERE slug = $1 LIMIT 1`
	t := &domain.Tenant{}
	err := r.db.Pool.QueryRow(ctx, query, slug).Scan(
		&t.ID, &t.Name, &t.Slug, &t.Plan, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get tenant by slug: %w", err)
	}
	return t, nil
}

func (r *TenantRepo) ListTenants(ctx context.Context, limit, offset int32) ([]*domain.Tenant, error) {
	query := `
		SELECT id, name, slug, plan, created_at, updated_at 
		FROM tenants 
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}
	defer rows.Close()

	var result []*domain.Tenant
	for rows.Next() {
		t := &domain.Tenant{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Plan, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, nil
}

