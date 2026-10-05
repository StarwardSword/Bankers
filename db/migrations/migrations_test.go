package migrations_test

import (
	"context"
	"errors"
	"testing"

	"github.com/StarwardSword/bank/pkg/testdb"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5"
)

// Every migration must be reversible: after its down the up applies again,
// and after all downs nothing is left in the schema.
func TestMigrations(t *testing.T) {
	dbURL := testdb.Schema(t)
	m := testdb.Migrate(t, dbURL)

	if err := m.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}
	if tables, routines := schemaObjects(t, dbURL); tables == 0 || routines == 0 {
		t.Fatalf("after up the test schema has %d table(s) and %d function(s); want both", tables, routines)
	}

	for {
		version, dirty, err := m.Version()
		if errors.Is(err, migrate.ErrNilVersion) {
			break
		}
		if err != nil || dirty {
			t.Fatalf("version = %d, dirty = %t, err = %v", version, dirty, err)
		}

		if err := m.Steps(-1); err != nil {
			t.Fatalf("down %d: %v", version, err)
		}
		if err := m.Steps(1); err != nil {
			t.Fatalf("up %d after its down: %v", version, err)
		}
		if err := m.Steps(-1); err != nil {
			t.Fatalf("down %d again: %v", version, err)
		}
	}

	if tables, routines := schemaObjects(t, dbURL); tables != 0 || routines != 0 {
		t.Fatalf("after all downs the schema still has %d table(s) and %d function(s)", tables, routines)
	}

	if err := m.Up(); err != nil {
		t.Fatalf("up after all downs: %v", err)
	}
}

// Counts tables (except the migrate version table) and functions in the schema of dbURL.
func schemaObjects(t *testing.T, dbURL string) (tables, routines int) {
	t.Helper()

	conn, err := pgx.Connect(t.Context(), dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	defer conn.Close(context.Background())

	err = conn.QueryRow(t.Context(), `
		SELECT
			(
				SELECT count(*)
				FROM information_schema.tables
				WHERE table_schema = current_schema() AND table_name <> 'schema_migrations'
			)
			,(
				SELECT count(*)
				FROM information_schema.routines
				WHERE routine_schema = current_schema()
			)
	`).Scan(&tables, &routines)
	if err != nil {
		t.Fatalf("inspecting schema: %v", err)
	}

	return tables, routines
}
