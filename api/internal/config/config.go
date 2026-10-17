package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	HTTPHost    string
	HTTPPort    int
	DatabaseURL string
	Migrations  string
}

func Load() (Config, error) {
	port := 8080
	if raw := os.Getenv("API_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 65535 {
			return Config{}, fmt.Errorf("invalid API_PORT")
		}
		port = parsed
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://rolescout:change-me-locally@127.0.0.1:5432/rolescout?sslmode=disable"
	}

	host := os.Getenv("API_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	migrations := os.Getenv("MIGRATIONS_DIR")
	if migrations == "" {
		migrations = "migrations"
	}

	return Config{
		HTTPHost:    host,
		HTTPPort:    port,
		DatabaseURL: dbURL,
		Migrations:  migrations,
	}, nil
}
