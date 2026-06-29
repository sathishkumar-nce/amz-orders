package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationHistoryTable = "schema_migrations"

// RunSQLMigrations executes every .sql file in a directory in version order based on file names like v1.sql, v2.sql, v10.sql.
// The SQL files should be idempotent because this runs on every startup.
func RunSQLMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	if err := ensureMigrationHistoryTable(ctx, pool); err != nil {
		return fmt.Errorf("ensure migration history table: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migration dir %q: %w", dir, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	sort.SliceStable(files, func(i, j int) bool {
		leftVersion, leftOK := migrationVersion(filepath.Base(files[i]))
		rightVersion, rightOK := migrationVersion(filepath.Base(files[j]))

		switch {
		case leftOK && rightOK && leftVersion != rightVersion:
			return leftVersion < rightVersion
		case leftOK != rightOK:
			return leftOK
		default:
			return filepath.Base(files[i]) < filepath.Base(files[j])
		}
	})

	if len(files) == 0 {
		log.Printf("⚠️  No SQL migration files found in %s", dir)
		return nil
	}

	applied, err := loadAppliedMigrations(ctx, pool)
	if err != nil {
		return fmt.Errorf("load applied migrations: %w", err)
	}

	if len(applied) == 0 {
		existingSchema, err := hasExistingApplicationSchema(ctx, pool)
		if err != nil {
			return fmt.Errorf("check existing application schema: %w", err)
		}
		if existingSchema {
			if err := markMigrationsApplied(ctx, pool, files); err != nil {
				return fmt.Errorf("bootstrap applied migrations: %w", err)
			}
			log.Printf("🗂️  Existing schema detected; marked %d migrations as already applied", len(files))
			applied, err = loadAppliedMigrations(ctx, pool)
			if err != nil {
				return fmt.Errorf("reload applied migrations: %w", err)
			}
		}
	}

	for _, file := range files {
		fileName := filepath.Base(file)
		if applied[fileName] {
			log.Printf("⏭️  Skipping already applied migration: %s", fileName)
			continue
		}

		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration file %q: %w", file, err)
		}

		log.Printf("🗄️  Running SQL migration: %s", fileName)
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("execute migration %q: %w", file, err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s (file_name)
			VALUES ($1)
			ON CONFLICT (file_name) DO NOTHING
		`, migrationHistoryTable), fileName); err != nil {
			return fmt.Errorf("record migration %q: %w", file, err)
		}
		log.Printf("✅ SQL migration complete: %s", fileName)
	}

	return nil
}

func ensureMigrationHistoryTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			file_name TEXT PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`, migrationHistoryTable))
	return err
}

func loadAppliedMigrations(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT file_name FROM %s`, migrationHistoryTable))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var fileName string
		if err := rows.Scan(&fileName); err != nil {
			return nil, err
		}
		applied[fileName] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return applied, nil
}

func hasExistingApplicationSchema(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name IN ('amazon_orders', 'amazon_order_products')
		)
	`).Scan(&exists)
	return exists, err
}

func markMigrationsApplied(ctx context.Context, pool *pgxpool.Pool, files []string) error {
	for _, file := range files {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s (file_name)
			VALUES ($1)
			ON CONFLICT (file_name) DO NOTHING
		`, migrationHistoryTable), filepath.Base(file)); err != nil {
			return err
		}
	}
	return nil
}

var migrationVersionPattern = regexp.MustCompile(`^v(\d+)\.sql$`)

func migrationVersion(fileName string) (int, bool) {
	matches := migrationVersionPattern.FindStringSubmatch(strings.ToLower(strings.TrimSpace(fileName)))
	if len(matches) != 2 {
		return 0, false
	}

	version, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false
	}

	return version, true
}
