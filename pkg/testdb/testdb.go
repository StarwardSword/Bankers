package testdb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StarwardSword/bank/db/migrations"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

const Env = "TEST_DB_CONNECTION_STRING"

func Schema(t *testing.T) string {
	t.Helper()

	if testing.Short() {
		t.Skip("the test needs a database")
	}

	dsn, source := connectionString(t)
	if dsn == "" {
		t.Skipf("the test needs a database: set %s", Env)
	}

	dbURL, err := url.Parse(dsn)
	if err != nil || (dbURL.Scheme != "postgres" && dbURL.Scheme != "postgresql") {
		t.Fatalf("connection string from %s must be a postgres:// URL", source)
	}

	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")

	admin, err := pgx.Connect(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connecting to database from %s: %v (is it running? try make test-db-up)", source, err)
	}
	t.Cleanup(func() { admin.Close(context.Background()) })

	if _, err := admin.Exec(t.Context(), "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatalf("creating schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("dropping schema %s: %v", schema, err)
		}
	})

	query := dbURL.Query()
	query.Set("search_path", schema)
	dbURL.RawQuery = query.Encode()

	return dbURL.String()
}

func Migrate(t *testing.T, dbURL string) *migrate.Migrate {
	t.Helper()

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("reading migrations: %v", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, dbURL)
	if err != nil {
		t.Fatalf("creating migrate: %v", err)
	}
	t.Cleanup(func() {
		if sourceErr, dbErr := m.Close(); sourceErr != nil || dbErr != nil {
			t.Errorf("closing migrate: %v", errors.Join(sourceErr, dbErr))
		}
	})

	return m
}

func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := Schema(t)
	if err := Migrate(t, dbURL).Up(); err != nil {
		t.Fatalf("applying migrations: %v", err)
	}

	pool, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func connectionString(t *testing.T) (dsn string, source string) {
	t.Helper()

	if dsn := os.Getenv(Env); dsn != "" {
		return dsn, Env
	}

	env, path, err := readDotEnv()
	if err != nil {
		t.Fatalf("reading .env: %v", err)
	}

	return env[Env], path
}

const recursionLimit = 5

func readDotEnv() (map[string]string, string, error) {
	if env, err := godotenv.Read(); err == nil {
		return env, ".env", nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}

	recursionCounter := 0
	for {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err == nil {
			env, err := godotenv.Read(path)
			return env, path, err
		}

		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return nil, "", nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, "", nil
		}
		dir = parent

		recursionCounter += 1
		if recursionCounter > recursionLimit {
			return nil, "", fmt.Errorf("testdb hit recursion limit trying to find .env file")
		}
	}
}
