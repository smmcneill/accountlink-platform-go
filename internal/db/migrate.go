package db

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	raw, err := migrationFiles.ReadFile("migrations/V1__init.sql")
	if err != nil {
		return err
	}

	statements := splitSQLStatements(string(raw))
	for _, stmt := range statements {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migration failed for statement %q: %w", stmt, err)
		}
	}

	return nil
}

func splitSQLStatements(src string) []string {
	parts := strings.Split(src, ";")

	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}

		out = append(out, s)
	}

	return out
}
