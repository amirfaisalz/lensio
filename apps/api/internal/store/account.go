package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Plan represents an API subscription tier with defined quota and rate limits.
type Plan struct {
	ID                 string    `json:"id"`
	Code               string    `json:"code"`
	Name               string    `json:"name"`
	MonthlyQuota       int       `json:"monthly_quota"`
	RateLimitPerMinute int       `json:"rate_limit_per_minute"`
	CreatedAt          time.Time `json:"created_at"`
}

// Organization represents tenant account information and current subscription plan.
type Organization struct {
	ID                 string    `json:"organization_id"`
	Name               string    `json:"organization_name"`
	Slug               string    `json:"slug"`
	PlanCode           string    `json:"plan_code"`
	PlanName           string    `json:"plan_name"`
	MonthlyQuota       int       `json:"monthly_quota"`
	RateLimitPerMinute int       `json:"rate_limit_per_minute"`
	ActiveKeysCount    int       `json:"active_keys_count"`
	CreatedAt          time.Time `json:"created_at"`
}

// User represents a tenant organization member or user account.
type User struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// AccountStore defines repository operations for organizations and subscription plans.
type AccountStore interface {
	GetOrganization(ctx context.Context, orgID string) (*Organization, error)
	GetOrganizationPlan(ctx context.Context, orgID string) (*Plan, error)
	UpdateOrganizationPlan(ctx context.Context, orgID string, planCode string) error
	GetOrganizationMembers(ctx context.Context, orgID string) ([]User, error)
}

// GetOrganization retrieves tenant account metadata, plan details, and active key count.
func (db *DB) GetOrganization(ctx context.Context, orgID string) (*Organization, error) {
	if orgID == "" {
		return nil, errors.New("orgID is required")
	}

	query := `
		SELECT 
			o.id, 
			o.name, 
			o.slug, 
			COALESCE(p.code, 'free'), 
			COALESCE(p.name, 'Free Tier'),
			COALESCE(p.monthly_quota, 100), 
			COALESCE(p.rate_limit_per_minute, 10),
			(
				SELECT COUNT(*) 
				FROM api_keys k 
				WHERE k.org_id = o.id 
				  AND k.revoked_at IS NULL 
				  AND (k.expires_at IS NULL OR k.expires_at > NOW())
			) AS active_keys_count,
			o.created_at
		FROM organizations o
		LEFT JOIN plans p ON o.plan_id = p.id
		WHERE o.id = $1;
	`

	var org Organization
	err := db.QueryRowContext(ctx, query, orgID).Scan(
		&org.ID,
		&org.Name,
		&org.Slug,
		&org.PlanCode,
		&org.PlanName,
		&org.MonthlyQuota,
		&org.RateLimitPerMinute,
		&org.ActiveKeysCount,
		&org.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("querying organization: %w", err)
	}

	return &org, nil
}

// GetOrganizationPlan retrieves the current active subscription plan for the organization.
func (db *DB) GetOrganizationPlan(ctx context.Context, orgID string) (*Plan, error) {
	if orgID == "" {
		return nil, errors.New("orgID is required")
	}

	query := `
		SELECT p.id, p.code, p.name, p.monthly_quota, p.rate_limit_per_minute, p.created_at
		FROM organizations o
		JOIN plans p ON o.plan_id = p.id
		WHERE o.id = $1;
	`

	var plan Plan
	err := db.QueryRowContext(ctx, query, orgID).Scan(
		&plan.ID,
		&plan.Code,
		&plan.Name,
		&plan.MonthlyQuota,
		&plan.RateLimitPerMinute,
		&plan.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Fallback: If organization has no plan attached, query the default 'free' plan
			fallbackQuery := `
				SELECT id, code, name, monthly_quota, rate_limit_per_minute, created_at
				FROM plans
				WHERE code = 'free'
				LIMIT 1;
			`
			errFallback := db.QueryRowContext(ctx, fallbackQuery).Scan(
				&plan.ID,
				&plan.Code,
				&plan.Name,
				&plan.MonthlyQuota,
				&plan.RateLimitPerMinute,
				&plan.CreatedAt,
			)
			if errFallback != nil {
				return nil, fmt.Errorf("querying fallback free plan: %w", errFallback)
			}
			return &plan, nil
		}
		return nil, fmt.Errorf("querying organization plan: %w", err)
	}

	return &plan, nil
}

// UpdateOrganizationPlan updates the subscription plan for an organization by plan code.
func (db *DB) UpdateOrganizationPlan(ctx context.Context, orgID string, planCode string) error {
	if orgID == "" || planCode == "" {
		return errors.New("orgID and planCode are required")
	}

	// Verify planCode exists
	var planID string
	err := db.QueryRowContext(ctx, "SELECT id FROM plans WHERE code = $1", planCode).Scan(&planID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("unknown plan code %q: %w", planCode, ErrNotFound)
		}
		return fmt.Errorf("looking up plan by code: %w", err)
	}

	query := `
		UPDATE organizations
		SET plan_id = $1, updated_at = NOW()
		WHERE id = $2;
	`

	res, err := db.ExecContext(ctx, query, planID, orgID)
	if err != nil {
		return fmt.Errorf("updating organization plan: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected on plan update: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// GetOrganizationMembers retrieves all registered users belonging to an organization.
func (db *DB) GetOrganizationMembers(ctx context.Context, orgID string) ([]User, error) {
	if orgID == "" {
		return nil, errors.New("orgID is required")
	}

	query := `
		SELECT id, org_id, email, full_name, role, created_at
		FROM users
		WHERE org_id = $1
		ORDER BY created_at ASC;
	`

	rows, err := db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("querying organization members: %w", err)
	}
	defer rows.Close()

	var members []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Email, &u.FullName, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning user row: %w", err)
		}
		members = append(members, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user rows: %w", err)
	}
	if members == nil {
		members = []User{}
	}

	return members, nil
}

