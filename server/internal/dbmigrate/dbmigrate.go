// Package dbmigrate applies the SQL migrations embedded in server/migrations at
// process start, so a deploy is a single image with no separate migrate step.
package dbmigrate

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/emilijan/beljot/server/migrations"
)

// Up applies every pending migration and returns the resulting schema version.
// It opens its own short-lived connection: the driver closes whatever *sql.DB
// it is handed, so sharing the app's pool would tear the pool down. A Postgres
// advisory lock inside golang-migrate keeps two replicas from racing. A dirty
// version (a half-applied migration) is reported by Up itself as
// migrate.ErrDirty, so it cannot slip through to Version.
func Up(databaseURL string) (uint, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return 0, fmt.Errorf("open embedded migrations: %w", err)
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return 0, fmt.Errorf("open database: %w", err)
	}
	driver, err := migratepgx.WithInstance(db, &migratepgx.Config{})
	if err != nil {
		_ = db.Close()
		return 0, fmt.Errorf("connect for migrations: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		_ = db.Close()
		return 0, fmt.Errorf("init migrator: %w", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			slog.Warn("closing migrator", "sourceError", srcErr, "databaseError", dbErr)
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return 0, fmt.Errorf("apply migrations: %w", err)
	}
	version, _, err := m.Version()
	if err != nil {
		return 0, fmt.Errorf("read migration version: %w", err)
	}
	return version, nil
}
