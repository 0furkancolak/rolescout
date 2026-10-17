package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool points at a throwaway database created for this test run. It is nil
// when TEST_DATABASE_URL is unset, and database tests are skipped.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		os.Exit(m.Run())
	}

	code, err := runWithTestDatabase(adminURL, m)
	if err != nil {
		log.Printf("database test setup: %v", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func runWithTestDatabase(adminURL string, m *testing.M) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return 0, fmt.Errorf("connect admin: %w", err)
	}
	defer admin.Close(context.Background())

	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	name := "rolescout_test_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		return 0, fmt.Errorf("create test database: %w", err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(),
			"DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
	}()

	testURL, err := url.Parse(adminURL)
	if err != nil {
		return 0, fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	testURL.Path = "/" + name
	testPool, err = Open(ctx, testURL.String())
	if err != nil {
		return 0, err
	}
	defer testPool.Close()

	if err := RunMigrations(ctx, testPool, "../../../migrations"); err != nil {
		return 0, err
	}
	return m.Run(), nil
}

func requireDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return testPool
}
