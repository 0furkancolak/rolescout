// Package dbtest creates throwaway PostgreSQL databases for integration tests.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
)

// EnvVar names the variable holding an admin connection URL. Tests that need
// a database are skipped when it is unset.
const EnvVar = "TEST_DATABASE_URL"

// Create makes an empty database next to the one adminURL points at and
// returns its URL and a function that drops it. The role in adminURL must be
// allowed to create databases.
func Create(adminURL string) (string, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return "", nil, fmt.Errorf("connect admin: %w", err)
	}

	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	name := "rolescout_test_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		_ = admin.Close(context.Background())
		return "", nil, fmt.Errorf("create test database: %w", err)
	}

	drop := func() {
		_, _ = admin.Exec(context.Background(),
			"DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	}

	testURL, err := url.Parse(adminURL)
	if err != nil {
		drop()
		return "", nil, fmt.Errorf("parse %s: %w", EnvVar, err)
	}
	testURL.Path = "/" + name
	return testURL.String(), drop, nil
}
