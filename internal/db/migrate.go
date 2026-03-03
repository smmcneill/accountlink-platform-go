package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"hash/crc32"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

var versionedMigrationPattern = regexp.MustCompile(`^V([0-9][0-9._-]*)__([^.]+)\.sql$`)

type migration struct {
	path        string
	version     string
	description string
	parts       []int
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `SELECT pg_advisory_lock(hashtext('flyway_schema_history'))`); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}

	defer func() {
		_, _ = pool.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext('flyway_schema_history'))`)
	}()

	if err := ensureSchemaHistory(ctx, pool); err != nil {
		return err
	}

	migrations, err := listMigrations(migrationFiles, "migrations")
	if err != nil {
		return err
	}

	applied, err := loadAppliedMigrations(ctx, pool)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		raw, err := migrationFiles.ReadFile(m.path)
		if err != nil {
			return err
		}

		checksum := int32(crc32.ChecksumIEEE(raw))
		if prevChecksum, ok := applied[m.version]; ok {
			if prevChecksum != checksum {
				return fmt.Errorf("checksum mismatch for migration %s (%s): db=%d file=%d", m.version, m.path, prevChecksum, checksum)
			}

			continue
		}

		if err := applyMigration(ctx, pool, m, raw, checksum); err != nil {
			return err
		}
	}

	return nil
}

func ensureSchemaHistory(ctx context.Context, pool *pgxpool.Pool) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS flyway_schema_history (
	installed_rank INT NOT NULL PRIMARY KEY,
	version VARCHAR(50),
	description VARCHAR(200) NOT NULL,
	type VARCHAR(20) NOT NULL,
	script VARCHAR(1000) NOT NULL,
	checksum INT,
	installed_by VARCHAR(100) NOT NULL,
	installed_on TIMESTAMPTZ NOT NULL DEFAULT now(),
	execution_time INT NOT NULL,
	success BOOLEAN NOT NULL
);
CREATE INDEX IF NOT EXISTS flyway_schema_history_s_idx ON flyway_schema_history (success);`

	_, err := pool.Exec(ctx, ddl)

	return err
}

func loadAppliedMigrations(ctx context.Context, pool *pgxpool.Pool) (map[string]int32, error) {
	rows, err := pool.Query(ctx, `SELECT version, checksum FROM flyway_schema_history WHERE success = true AND version IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]int32)

	for rows.Next() {
		var (
			version  string
			checksum int32
		)
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, err
		}

		applied[version] = checksum
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return applied, nil
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, m migration, raw []byte, checksum int32) error {
	started := time.Now()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}

	committed := false

	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()

	statements := splitSQLStatements(string(raw))
	for _, stmt := range statements {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migration failed for file %s statement %q: %w", m.path, stmt, err)
		}
	}

	executionMS := int(time.Since(started).Milliseconds())

	_, err = tx.Exec(ctx, `
INSERT INTO flyway_schema_history (
	installed_rank,
	version,
	description,
	type,
	script,
	checksum,
	installed_by,
	execution_time,
	success
) VALUES (
	(SELECT COALESCE(MAX(installed_rank), 0) + 1 FROM flyway_schema_history),
	$1, $2, 'SQL', $3, $4, current_user, $5, true
)`, m.version, m.description, migrationScriptName(m.path), checksum, executionMS)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	committed = true

	return nil
}

func listMigrations(files fs.FS, dir string) ([]migration, error) {
	entries, err := fs.ReadDir(files, dir)
	if err != nil {
		return nil, err
	}

	migrations := make([]migration, 0, len(entries))
	seenVersions := make(map[string]struct{})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		m, err := parseVersionedMigration(entry.Name(), dir)
		if err != nil {
			return nil, err
		}

		if _, dup := seenVersions[m.version]; dup {
			return nil, fmt.Errorf("duplicate migration version: %s", m.version)
		}

		seenVersions[m.version] = struct{}{}
		migrations = append(migrations, m)
	}

	sort.Slice(migrations, func(i, j int) bool {
		return compareVersionParts(migrations[i].parts, migrations[j].parts) < 0
	})

	return migrations, nil
}

func parseVersionedMigration(fileName, dir string) (migration, error) {
	matches := versionedMigrationPattern.FindStringSubmatch(fileName)
	if len(matches) != 3 {
		return migration{}, fmt.Errorf("invalid migration filename %q, expected V<version>__<description>.sql", fileName)
	}

	parts, err := parseVersionParts(matches[1])
	if err != nil {
		return migration{}, fmt.Errorf("invalid migration version in %q: %w", fileName, err)
	}

	return migration{
		path:        dir + "/" + fileName,
		version:     matches[1],
		description: strings.ReplaceAll(matches[2], "_", " "),
		parts:       parts,
	}, nil
}

func parseVersionParts(version string) ([]int, error) {
	normalized := strings.ReplaceAll(version, "-", ".")
	normalized = strings.ReplaceAll(normalized, "_", ".")
	tokens := strings.Split(normalized, ".")

	parts := make([]int, 0, len(tokens))
	for _, token := range tokens {
		if token == "" {
			return nil, errors.New("empty version token")
		}

		n, err := strconv.Atoi(token)
		if err != nil {
			return nil, err
		}

		parts = append(parts, n)
	}

	return parts, nil
}

func compareVersionParts(a, b []int) int {
	max := len(a)
	if len(b) > max {
		max = len(b)
	}

	for i := 0; i < max; i++ {
		av := 0
		if i < len(a) {
			av = a[i]
		}

		bv := 0
		if i < len(b) {
			bv = b[i]
		}

		if av < bv {
			return -1
		}

		if av > bv {
			return 1
		}
	}

	return 0
}

func migrationScriptName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return path
	}

	return parts[len(parts)-1]
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
