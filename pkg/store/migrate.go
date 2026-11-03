package store

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies the embedded SQL migrations to the database. A clean,
// already-applied schema is a no-op returning nil (idempotent re-runs);
// every other failure is returned for the caller to fail fast on
// (login design spec §9: migrations run at apiserver startup, exit on error).
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	driver, err := pgxmigrate.WithInstance(sqlDB, &pgxmigrate.Config{
		MultiStatementEnabled: true,
		MultiStatementMaxSize: 10 * 1024 * 1024,
	})
	if err != nil {
		return fmt.Errorf("init postgres migrate driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
