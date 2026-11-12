package app

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConfigureDBRejectsInvalidURI(t *testing.T) {
	pool, err := configureDB("://invalid")
	if err == nil {
		pool.Close()
		t.Fatal("expected invalid connection URI to fail")
	}
}

func TestConfigureDBAppliesMigrationsAndPreservesData(t *testing.T) {
	databaseURI := os.Getenv("TEST_DATABASE_URI")
	if databaseURI == "" {
		t.Skip("TEST_DATABASE_URI is required for the database startup test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, databaseURI)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE"); err != nil {
			t.Errorf("remove test schema: %v", err)
		}
	}()
	uri, err := url.Parse(databaseURI)
	if err != nil {
		t.Fatal(err)
	}
	query := uri.Query()
	query.Set("search_path", schema)
	uri.RawQuery = query.Encode()

	pool, err := configureDB(uri.String())
	if err != nil {
		t.Fatal(err)
	}
	var userID int64
	err = pool.QueryRow(ctx, `INSERT INTO users (login, passwordHash) VALUES ('startup-test', 'hash') RETURNING id`).Scan(&userID)
	pool.Close()
	if err != nil {
		t.Fatalf("write to freshly migrated database: %v", err)
	}

	for restart := 1; restart <= 2; restart++ {
		pool, err := configureDB(uri.String())
		if err != nil {
			t.Fatalf("restart %d: %v", restart, err)
		}
		var storedID, version int64
		readErr := pool.QueryRow(ctx, `SELECT id FROM users WHERE login = 'startup-test'`).Scan(&storedID)
		versionErr := pool.QueryRow(ctx, `SELECT MAX(version_id) FROM goose_db_version WHERE is_applied`).Scan(&version)
		pool.Close()
		if readErr != nil || versionErr != nil || storedID != userID || version != 3 {
			t.Fatalf("restart %d: userID=%d want=%d, version=%d want=3, read=%v migration=%v", restart, storedID, userID, version, readErr, versionErr)
		}
	}
}
