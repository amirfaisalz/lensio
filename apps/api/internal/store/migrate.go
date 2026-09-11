package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/amirfaisalz/nusaid/apps/api/migrations"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// NewMigrator returns a migrate.Migrate instance configured with embedded migrations and the database pool.
func NewMigrator(db *sql.DB) (*migrate.Migrate, error) {
	if db == nil {
		return nil, errors.New("database handle is nil")
	}

	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("creating migration source driver: %w", err)
	}

	dbDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("creating migration database driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", dbDriver)
	if err != nil {
		return nil, fmt.Errorf("initializing migrator: %w", err)
	}

	return m, nil
}

// RunMigrationsUp applies all pending database migrations.
func RunMigrationsUp(db *sql.DB) error {
	m, err := NewMigrator(db)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("applying migrations up: %w", err)
	}

	return nil
}

// RunMigrationsDown rolls back all applied migrations.
func RunMigrationsDown(db *sql.DB) error {
	m, err := NewMigrator(db)
	if err != nil {
		return err
	}

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("applying migrations down: %w", err)
	}

	return nil
}
