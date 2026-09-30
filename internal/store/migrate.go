package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/esauvisky/gitgram/migrations"
)

// migrate applies every embedded migration whose NNNN_ prefix is greater
// than the database's PRAGMA user_version, each in its own transaction that
// also bumps user_version.
func migrate(ctx context.Context, db *sql.DB) error {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var current int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}

	for _, name := range names {
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			return fmt.Errorf("migration %q: missing NNNN_ prefix", name)
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return fmt.Errorf("migration %q: bad version prefix: %w", name, err)
		}
		if version <= current {
			continue
		}
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return fmt.Errorf("read migration %q: %w", name, err)
		}
		if err := applyMigration(ctx, db, version, string(body)); err != nil {
			return fmt.Errorf("apply migration %q: %w", name, err)
		}
		current = version
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, version int, body string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, body); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(version)); err != nil {
		return err
	}
	return tx.Commit()
}
