package database

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"

	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

//go:embed seed/*.sql
var seedFS embed.FS

// Open connects to PostgreSQL using a lib/pq DSN.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// Migrate runs embedded schema migrations in filename order.
// Each file is idempotent (IF NOT EXISTS / ON CONFLICT), so re-runs are safe.
func Migrate(db *sql.DB) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := []string{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		b, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(b)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

// SeedIfEmpty runs the embedded demo seed only when the users table is empty.
// It never overwrites existing data.
func SeedIfEmpty(db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	entries, err := seedFS.ReadDir("seed")
	if err != nil {
		return false, err
	}
	names := []string{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		b, err := seedFS.ReadFile("seed/" + name)
		if err != nil {
			return false, err
		}
		if _, err := db.Exec(string(b)); err != nil {
			return false, fmt.Errorf("seed %s: %w", name, err)
		}
	}
	return true, nil
}
