package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/ingest"
)

const countriesUsage = "usage: admin countries backfill\n\nbackfill sets the country of open and closed jobs that have a location but no country."

// backfillCountries resolves the country of jobs stored before the place
// list grew (Turkish provinces, US states and more) and reports how many
// jobs gained one.
func backfillCountries(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	rows, err := pool.Query(ctx, `SELECT location, count(*) FROM jobs WHERE country_code IS NULL AND location IS NOT NULL GROUP BY location`)
	if err != nil {
		return err
	}
	type loc struct {
		text string
		n    int
	}
	var locs []loc
	for rows.Next() {
		var l loc
		if err := rows.Scan(&l.text, &l.n); err != nil {
			rows.Close()
			return err
		}
		locs = append(locs, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	updated := 0
	for _, l := range locs {
		code := ingest.InferCountry(l.text)
		if code == "" {
			continue
		}
		tag, err := pool.Exec(ctx, `UPDATE jobs SET country_code = $1 WHERE location = $2 AND country_code IS NULL`, code, l.text)
		if err != nil {
			return err
		}
		updated += int(tag.RowsAffected())
	}
	fmt.Fprintf(out, "%d of %d jobs without a country gained one (%d distinct locations)\n", updated, countTotal(locs, func(l loc) int { return l.n }), len(locs))
	return nil
}

func countTotal[T any](xs []T, f func(T) int) int {
	n := 0
	for _, x := range xs {
		n += f(x)
	}
	return n
}
