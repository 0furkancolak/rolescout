package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SettingsRepository stores installation settings as key/value rows.
type SettingsRepository struct{ pool *pgxpool.Pool }

func NewSettingsRepository(pool *pgxpool.Pool) *SettingsRepository {
	return &SettingsRepository{pool: pool}
}

// All returns every stored setting.
func (r *SettingsRepository) All(ctx context.Context) (map[string]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT key, value FROM app_settings`)
	if err != nil {
		return nil, translate(err, "list settings")
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, translate(err, "scan setting")
		}
		out[k] = v
	}
	return out, translate(rows.Err(), "list settings")
}

// Apply writes the given settings in one transaction. A nil value deletes the
// row, so the environment default applies again.
func (r *SettingsRepository) Apply(ctx context.Context, changes map[string]*string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return translate(err, "begin settings")
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	for k, v := range changes {
		if v == nil {
			_, err = tx.Exec(ctx, `DELETE FROM app_settings WHERE key = $1`, k)
		} else {
			_, err = tx.Exec(ctx, `INSERT INTO app_settings (key, value) VALUES ($1, $2)
				ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, k, *v)
		}
		if err != nil {
			return translate(err, "write setting")
		}
	}
	return translate(tx.Commit(ctx), "commit settings")
}
