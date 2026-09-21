package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrDuplicateEmail is returned when attempting to register an already existing email address.
	ErrDuplicateEmail = errors.New("email already registered")
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
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id,omitempty"`
	Email         string    `json:"email"`
	FullName      string    `json:"full_name"`
	Role          string    `json:"role"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
}

// UserWithAuth represents user authentication details including credentials.
type UserWithAuth struct {
	User
	PasswordHash      string `json:"-"`
	VerificationToken string `json:"-"`
}

// AccountStore defines repository operations for organizations and subscription plans.
type AccountStore interface {
	GetOrganization(ctx context.Context, orgID string) (*Organization, error)
	GetOrganizationPlan(ctx context.Context, orgID string) (*Plan, error)
	UpdateOrganizationPlan(ctx context.Context, orgID string, planCode string) error
	GetOrganizationMembers(ctx context.Context, orgID string) ([]User, error)
	CreateUser(ctx context.Context, fullName, email, passwordHash, verificationToken string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*UserWithAuth, error)
	GetUserByID(ctx context.Context, userID string) (*User, error)
	VerifyUserEmail(ctx context.Context, email, token string) error
	CreateOrganization(ctx context.Context, name, slug, planCode string) (*Organization, error)
	AssignUserToOrg(ctx context.Context, userID, orgID, role string) error
	GetUserOrganization(ctx context.Context, userID string) (*Organization, error)
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
		SELECT id, COALESCE(org_id::text, ''), email, full_name, role, COALESCE(email_verified, false), created_at
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
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Email, &u.FullName, &u.Role, &u.EmailVerified, &u.CreatedAt); err != nil {
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

// CreateUser registers a new user identity in the database with pending email verification.
func (db *DB) CreateUser(ctx context.Context, fullName, email, passwordHash, verificationToken string) (*User, error) {
	fullName = strings.TrimSpace(fullName)
	email = strings.ToLower(strings.TrimSpace(email))

	if fullName == "" || email == "" || passwordHash == "" {
		return nil, errors.New("fullName, email, and passwordHash are required")
	}

	// Check if user already exists
	var existingID string
	err := db.QueryRowContext(ctx, "SELECT id FROM users WHERE LOWER(email) = LOWER($1)", email).Scan(&existingID)
	if err == nil {
		return nil, ErrDuplicateEmail
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("checking existing user: %w", err)
	}

	query := `
		INSERT INTO users (full_name, email, password_hash, verification_token, email_verified, role)
		VALUES ($1, $2, $3, $4, FALSE, 'member')
		RETURNING id, COALESCE(org_id::text, ''), email, full_name, role, email_verified, created_at;
	`

	var u User
	err = db.QueryRowContext(ctx, query, fullName, email, passwordHash, verificationToken).Scan(
		&u.ID,
		&u.OrgID,
		&u.Email,
		&u.FullName,
		&u.Role,
		&u.EmailVerified,
		&u.CreatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, ErrDuplicateEmail
		}
		return nil, fmt.Errorf("inserting user: %w", err)
	}

	return &u, nil
}

// GetUserByEmail queries user credentials and verification state by email address.
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*UserWithAuth, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, errors.New("email is required")
	}

	query := `
		SELECT 
			id, 
			COALESCE(org_id::text, ''), 
			email, 
			full_name, 
			role, 
			COALESCE(email_verified, false), 
			COALESCE(password_hash, ''), 
			COALESCE(verification_token, ''), 
			created_at
		FROM users
		WHERE LOWER(email) = LOWER($1);
	`

	var u UserWithAuth
	err := db.QueryRowContext(ctx, query, email).Scan(
		&u.ID,
		&u.OrgID,
		&u.Email,
		&u.FullName,
		&u.Role,
		&u.EmailVerified,
		&u.PasswordHash,
		&u.VerificationToken,
		&u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("querying user by email: %w", err)
	}

	return &u, nil
}

// GetUserByID queries a user by primary key ID.
func (db *DB) GetUserByID(ctx context.Context, userID string) (*User, error) {
	if userID == "" {
		return nil, errors.New("userID is required")
	}

	query := `
		SELECT id, COALESCE(org_id::text, ''), email, full_name, role, COALESCE(email_verified, false), created_at
		FROM users
		WHERE id = $1;
	`

	var u User
	err := db.QueryRowContext(ctx, query, userID).Scan(
		&u.ID,
		&u.OrgID,
		&u.Email,
		&u.FullName,
		&u.Role,
		&u.EmailVerified,
		&u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("querying user by id: %w", err)
	}

	return &u, nil
}

// VerifyUserEmail updates the email_verified flag for a user if the token matches.
func (db *DB) VerifyUserEmail(ctx context.Context, email, token string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return errors.New("email is required")
	}

	// The verification token is mandatory. An earlier "dev flow" branch verified
	// by email alone whenever the token was empty, which let anyone activate any
	// account without ever reading its mailbox.
	if token == "" {
		return ErrNotFound
	}

	const query = `
		UPDATE users
		SET email_verified = TRUE, verification_token = NULL
		WHERE LOWER(email) = LOWER($1) AND verification_token = $2;
	`

	res, err := db.ExecContext(ctx, query, email, token)
	if err != nil {
		return fmt.Errorf("updating user verification: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected on user verification: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// CreateOrganization provisions a new organization row in the database.
func (db *DB) CreateOrganization(ctx context.Context, name, slug, planCode string) (*Organization, error) {
	name = strings.TrimSpace(name)
	slug = strings.ToLower(strings.TrimSpace(slug))
	if planCode == "" {
		planCode = "free"
	}

	if name == "" || slug == "" {
		return nil, errors.New("name and slug are required")
	}

	// Verify plan exists
	var planID string
	var monthlyQuota, rateLimit int
	var planName string
	err := db.QueryRowContext(ctx, "SELECT id, name, monthly_quota, rate_limit_per_minute FROM plans WHERE code = $1", planCode).Scan(
		&planID, &planName, &monthlyQuota, &rateLimit,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("plan code %q not found: %w", planCode, ErrNotFound)
		}
		return nil, fmt.Errorf("looking up plan: %w", err)
	}

	query := `
		INSERT INTO organizations (name, slug, plan_id)
		VALUES ($1, $2, $3)
		RETURNING id, created_at;
	`

	var org Organization
	org.Name = name
	org.Slug = slug
	org.PlanCode = planCode
	org.PlanName = planName
	org.MonthlyQuota = monthlyQuota
	org.RateLimitPerMinute = rateLimit

	err = db.QueryRowContext(ctx, query, name, slug, planID).Scan(&org.ID, &org.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting organization: %w", err)
	}

	return &org, nil
}

// AssignUserToOrg updates a user's associated organization and role.
func (db *DB) AssignUserToOrg(ctx context.Context, userID, orgID, role string) error {
	if userID == "" || orgID == "" {
		return errors.New("userID and orgID are required")
	}
	if role == "" {
		role = "owner"
	}

	query := `
		UPDATE users
		SET org_id = $1, role = $2
		WHERE id = $3;
	`

	res, err := db.ExecContext(ctx, query, orgID, role, userID)
	if err != nil {
		return fmt.Errorf("assigning user to organization: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected on user assignment: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// GetUserOrganization retrieves the organization belonging to the specified user.
func (db *DB) GetUserOrganization(ctx context.Context, userID string) (*Organization, error) {
	if userID == "" {
		return nil, errors.New("userID is required")
	}

	var orgID sql.NullString
	query := `SELECT org_id FROM users WHERE id = $1;`
	err := db.QueryRowContext(ctx, query, userID).Scan(&orgID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("querying user org_id: %w", err)
	}

	if !orgID.Valid || orgID.String == "" {
		return nil, ErrNotFound
	}

	return db.GetOrganization(ctx, orgID.String)
}

// SessionsValidFrom returns the instant from which session tokens for a user are
// accepted. Tokens issued before it were revoked, typically by an explicit logout.
func (db *DB) SessionsValidFrom(ctx context.Context, userID string) (time.Time, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return time.Time{}, errors.New("userID is required")
	}

	var validFrom time.Time
	err := db.QueryRowContext(ctx,
		`SELECT sessions_valid_from FROM users WHERE id = $1;`, userID,
	).Scan(&validFrom)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, ErrNotFound
		}
		return time.Time{}, fmt.Errorf("querying session validity for user: %w", err)
	}

	return validFrom, nil
}

// RevokeUserSessions invalidates every session token issued to a user so far.
func (db *DB) RevokeUserSessions(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("userID is required")
	}

	res, err := db.ExecContext(ctx,
		`UPDATE users SET sessions_valid_from = NOW() WHERE id = $1;`, userID,
	)
	if err != nil {
		return fmt.Errorf("revoking user sessions: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected on session revocation: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// PasswordResetTTL bounds how long a reset token stays usable.
const PasswordResetTTL = 1 * time.Hour

// SetPasswordResetToken records the hash of a reset token for the given email.
//
// It reports ErrNotFound when no such account exists. Callers must NOT surface
// that distinction: replying differently for known and unknown addresses turns
// the reset endpoint into an account enumeration oracle.
func (db *DB) SetPasswordResetToken(ctx context.Context, email, tokenHash string, expiresAt time.Time) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return errors.New("email is required")
	}
	if tokenHash == "" {
		return errors.New("token hash is required")
	}

	res, err := db.ExecContext(ctx,
		`UPDATE users
		 SET password_reset_token_hash = $2, password_reset_expires_at = $3
		 WHERE LOWER(email) = LOWER($1);`,
		email, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("storing password reset token: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected on reset token: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ConsumePasswordResetToken atomically verifies an unexpired reset token, sets
// the new password hash, clears the token, and invalidates every existing
// session for that user.
//
// It is a single statement so a token cannot be redeemed twice concurrently:
// the same UPDATE that matches the token also clears it.
func (db *DB) ConsumePasswordResetToken(ctx context.Context, email, tokenHash, newPasswordHash string, now time.Time) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || tokenHash == "" || newPasswordHash == "" {
		return errors.New("email, token hash and new password hash are required")
	}

	res, err := db.ExecContext(ctx,
		`UPDATE users
		 SET password_hash = $3,
		     password_reset_token_hash = NULL,
		     password_reset_expires_at = NULL,
		     sessions_valid_from = $4
		 WHERE LOWER(email) = LOWER($1)
		   AND password_reset_token_hash = $2
		   AND password_reset_expires_at > $4;`,
		email, tokenHash, newPasswordHash, now)
	if err != nil {
		return fmt.Errorf("consuming password reset token: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected on password reset: %w", err)
	}
	if rows == 0 {
		// Wrong token, wrong email, already used, or expired — all the same to
		// the caller, deliberately.
		return ErrNotFound
	}
	return nil
}
