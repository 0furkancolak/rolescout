package database

import (
	"context"
	"reflect"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
)

// The tests use a fixed "now" in 2031 so rows from other tests that share the
// database never fall into the window.
func TestActivityCountsPerDayAndIsolation(t *testing.T) {
	f := newFixture(t)
	other := newFixture(t)
	pool := requireDB(t)
	ctx := context.Background()
	now := time.Date(2031, 3, 10, 12, 0, 0, 0, time.UTC)
	at := func(day, hour int) time.Time { return time.Date(2031, 3, day, hour, 0, 0, 0, time.UTC) }

	research := NewResearchRepository(pool)
	jobsRepo := NewJobRepository(pool)
	apps := NewApplicationRepository(pool)
	mkJob := func(src string, first time.Time) domain.Job {
		j, _, err := jobsRepo.UpsertListing(ctx, IngestedListing{SourceID: src, ExternalID: uniq("e"), URL: "https://x.example/" + uniq("u"),
			DedupeKey: uniq("dk"), Title: "T", CompanyName: "Acme", SeenAt: first})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE jobs SET first_seen_at = $2, last_seen_at = $2 WHERE id = $1`, j.ID, first); err != nil {
			t.Fatal(err)
		}
		return j
	}

	// Jobs first seen: one on the 9th, two on the 10th, one outside the window.
	j9 := mkJob(f.source.ID, at(9, 8))
	j10a := mkJob(f.source.ID, at(10, 1))
	j10b := mkJob(f.source.ID, at(10, 11))
	mkJob(f.source.ID, at(1, 8))

	profile, err := research.CreateProfile(ctx, f.workspace.ID, profileParams(uniq("act")))
	if err != nil {
		t.Fatal(err)
	}
	otherProfile, err := research.CreateProfile(ctx, other.workspace.ID, profileParams(uniq("act")))
	if err != nil {
		t.Fatal(err)
	}
	match := func(profileID, jobID string, when time.Time) {
		if _, err := pool.Exec(ctx, `INSERT INTO research_matches (profile_id, job_id, score, first_matched_at) VALUES ($1, $2, 80, $3)`, profileID, jobID, when); err != nil {
			t.Fatal(err)
		}
	}
	match(profile.ID, j10a.ID, at(10, 1))
	match(profile.ID, j10b.ID, at(10, 11))
	match(profile.ID, j9.ID, time.Date(2031, 3, 8, 23, 59, 59, 0, time.UTC))
	match(otherProfile.ID, j9.ID, at(9, 10)) // another workspace: must not count

	app, err := apps.Create(ctx, f.workspace.ID, j9.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	otherApp, err := apps.Create(ctx, other.workspace.ID, j9.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	pool.Exec(ctx, `UPDATE applications SET created_at = $2 WHERE id = $1`, app.ID, at(9, 9))
	pool.Exec(ctx, `UPDATE applications SET created_at = $2 WHERE id = $1`, otherApp.ID, at(9, 9))

	event := func(ws, appID, typ string, when time.Time) {
		if _, err := pool.Exec(ctx, `INSERT INTO application_events (workspace_id, application_id, type, occurred_at) VALUES ($1, $2, $3, $4)`, ws, appID, typ, when); err != nil {
			t.Fatal(err)
		}
	}
	event(f.workspace.ID, app.ID, "interview", at(12, 10))
	event(f.workspace.ID, app.ID, "interview", at(20, 10))
	event(f.workspace.ID, app.ID, "interview", at(5, 10)) // already happened
	event(f.workspace.ID, app.ID, "deadline", at(12, 10)) // not an interview
	event(other.workspace.ID, otherApp.ID, "interview", at(12, 10))

	got, err := NewActivityRepository(pool).Activity(ctx, f.workspace.ID, 3, now)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.Activity{
		Days:                []string{"2031-03-08", "2031-03-09", "2031-03-10"},
		NewMatches:          []int{1, 0, 2},
		ApplicationsCreated: []int{0, 1, 0},
		UpcomingInterviews:  2,
	}
	if !reflect.DeepEqual(got.Days, want.Days) || !reflect.DeepEqual(got.NewMatches, want.NewMatches) ||
		!reflect.DeepEqual(got.ApplicationsCreated, want.ApplicationsCreated) || got.UpcomingInterviews != want.UpcomingInterviews {
		t.Fatalf("activity = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(got.JobsCollected, []int{0, 1, 2}) {
		t.Fatalf("jobs collected = %v, want [0 1 2]", got.JobsCollected)
	}
}

func TestActivityEmptyWorkspaceIsZeroFilled(t *testing.T) {
	f := newFixture(t)
	now := time.Date(2032, 1, 2, 0, 0, 0, 0, time.UTC)
	got, err := NewActivityRepository(requireDB(t)).Activity(context.Background(), f.workspace.ID, 14, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Days) != 14 || got.Days[13] != "2032-01-02" || got.Days[0] != "2031-12-20" {
		t.Fatalf("days = %v", got.Days)
	}
	for _, s := range [][]int{got.NewMatches, got.JobsCollected, got.ApplicationsCreated} {
		if len(s) != 14 {
			t.Fatalf("series length = %d", len(s))
		}
		for _, n := range s {
			if n != 0 {
				t.Fatalf("series = %v, want zeros", s)
			}
		}
	}
}
