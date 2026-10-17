package database

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/infrastructure/database/dbtest"
)

// testPool points at a throwaway database created for this test run. It is nil
// when TEST_DATABASE_URL is unset, and database tests are skipped.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	adminURL := os.Getenv(dbtest.EnvVar)
	if adminURL == "" {
		os.Exit(m.Run())
	}

	dbURL, drop, err := dbtest.Create(adminURL)
	if err != nil {
		log.Fatalf("database test setup: %v", err)
	}
	code := run(m, dbURL)
	drop()
	os.Exit(code)
}

func run(m *testing.M, dbURL string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	testPool, err = Open(ctx, dbURL)
	if err != nil {
		log.Printf("open test database: %v", err)
		return 1
	}
	defer testPool.Close()

	if err := RunMigrations(ctx, testPool, "../../../migrations"); err != nil {
		log.Printf("migrate test database: %v", err)
		return 1
	}
	return m.Run()
}

func requireDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return testPool
}
