package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
)

func uniq(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

type fixture struct {
	user      domain.User
	workspace domain.Workspace
	source    domain.JobSource
	job       domain.Job
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := requireDB(t)
	ctx := context.Background()

	user, err := NewUserRepository(pool).Create(ctx, CreateUserParams{
		Email: uniq("user") + "@example.com", DisplayName: "Test User",
	})
	if err != nil {
		t.Fatal(err)
	}
	ws, err := NewWorkspaceRepository(pool).Create(ctx, "Personal", user.ID)
	if err != nil {
		t.Fatal(err)
	}
	jobs := NewJobRepository(pool)
	source, err := jobs.CreateSource(ctx, uniq("src-"), "Example Source", "https://jobs.example.com")
	if err != nil {
		t.Fatal(err)
	}
	job, _, err := jobs.UpsertListing(ctx, IngestedListing{
		SourceID: source.ID, ExternalID: uniq("ext"), URL: "https://jobs.example.com/1",
		DedupeKey: uniq("dk"), Title: "Backend Engineer", CompanyName: "Acme",
	})
	if err != nil {
		t.Fatal(err)
	}
	return fixture{user: user, workspace: ws, source: source, job: job}
}

func requireErr(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestMigrationsAreRepeatableAndSerialized(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = RunMigrations(ctx, pool, "../../../migrations")
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	files, err := os.ReadDir("../../../migrations")
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".sql") {
			want++
		}
	}
	var got int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM rolescout_schema_migrations`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("applied migrations = %d, want %d", got, want)
	}
}

func TestUserEmailIsNormalizedAndUnique(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	users := NewUserRepository(pool)

	email := uniq("Mixed") + "@Example.com"
	u, err := users.Create(ctx, CreateUserParams{Email: "  " + email + " ", DisplayName: "A", Locale: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != strings.ToLower(email) {
		t.Fatalf("email = %q, want lower-cased", u.Email)
	}
	if _, err := users.GetByEmail(ctx, strings.ToUpper(email)); err != nil {
		t.Fatalf("lookup by email ignoring case: %v", err)
	}

	_, err = users.Create(ctx, CreateUserParams{Email: strings.ToUpper(email), DisplayName: "B"})
	requireErr(t, err, domain.ErrConflict)

	_, err = users.Create(ctx, CreateUserParams{Email: uniq("x") + "@example.com", DisplayName: "C", Locale: "de"})
	requireErr(t, err, domain.ErrInvalid)
}

func TestWorkspaceCreateAddsOwner(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	workspaces := NewWorkspaceRepository(testPool)

	m, err := workspaces.Membership(ctx, f.workspace.ID, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if m.Role != domain.RoleOwner {
		t.Fatalf("role = %s, want owner", m.Role)
	}

	list, err := workspaces.ListForUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != f.workspace.ID {
		t.Fatalf("workspaces = %+v", list)
	}

	other := newFixture(t)
	_, err = workspaces.Membership(ctx, f.workspace.ID, other.user.ID)
	requireErr(t, err, domain.ErrNotFound)

	_, err = workspaces.Create(ctx, "Orphan", "00000000-0000-0000-0000-000000000000")
	requireErr(t, err, domain.ErrNotFound)
}

func TestUpsertListingIsIdempotentAndDeduplicates(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	jobs := NewJobRepository(testPool)

	first := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	in := IngestedListing{
		SourceID: f.source.ID, ExternalID: "abc-1", URL: "https://jobs.example.com/abc-1",
		SeenAt: first, DedupeKey: uniq("dk"), Title: "Go Developer", CompanyName: "Acme",
		WorkArrangement: "remote", Technologies: []string{"go", "postgresql"},
	}
	job1, listing1, err := jobs.UpsertListing(ctx, in)
	if err != nil {
		t.Fatal(err)
	}

	in.SeenAt = first.Add(24 * time.Hour)
	in.Title = "Senior Go Developer"
	job2, listing2, err := jobs.UpsertListing(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if job1.ID != job2.ID || listing1.ID != listing2.ID {
		t.Fatal("re-ingesting the same listing created new rows")
	}
	if job2.Title != "Senior Go Developer" {
		t.Fatalf("title = %q, want updated", job2.Title)
	}
	if !job2.FirstSeenAt.Equal(first) || !job2.LastSeenAt.Equal(in.SeenAt) {
		t.Fatalf("seen window = %v..%v", job2.FirstSeenAt, job2.LastSeenAt)
	}

	other, err := jobs.CreateSource(ctx, uniq("src-"), "Second Source", "https://other.example.com")
	if err != nil {
		t.Fatal(err)
	}
	in.SourceID, in.ExternalID, in.URL = other.ID, "zzz", "https://other.example.com/zzz"
	job3, _, err := jobs.UpsertListing(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if job3.ID != job1.ID {
		t.Fatal("same dedupe key on another source created a second job")
	}

	listings, err := jobs.ListListings(ctx, job1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 2 {
		t.Fatalf("listings = %d, want 2", len(listings))
	}

	in.URL = "ftp://bad"
	_, _, err = jobs.UpsertListing(ctx, in)
	requireErr(t, err, domain.ErrInvalid)
}

func TestSavedJobs(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	jobs := NewJobRepository(testPool)

	for range 2 {
		if err := jobs.SaveJob(ctx, f.workspace.ID, f.job.ID, &f.user.ID); err != nil {
			t.Fatal(err)
		}
	}
	saved, err := jobs.ListSavedJobs(ctx, f.workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 {
		t.Fatalf("saved jobs = %d, want 1", len(saved))
	}

	if err := jobs.UnsaveJob(ctx, f.workspace.ID, f.job.ID); err != nil {
		t.Fatal(err)
	}
	requireErr(t, jobs.UnsaveJob(ctx, f.workspace.ID, f.job.ID), domain.ErrNotFound)
}

func TestApplicationHistory(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	apps := NewApplicationRepository(testPool)

	app, err := apps.Create(ctx, f.workspace.ID, f.job.ID, &f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if app.Status != domain.StatusPlanned {
		t.Fatalf("status = %s, want planned", app.Status)
	}
	_, err = apps.Create(ctx, f.workspace.ID, f.job.ID, nil)
	requireErr(t, err, domain.ErrConflict)

	applied := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	a1, err := apps.AddAttempt(ctx, f.workspace.ID, app.ID, AddAttemptParams{AppliedAt: applied, Channel: "company site"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := apps.ChangeStatus(ctx, f.workspace.ID, app.ID, domain.StatusApplied, &f.user.ID); err != nil {
		t.Fatal(err)
	}
	// Setting the same status again must not add history.
	if _, err := apps.ChangeStatus(ctx, f.workspace.ID, app.ID, domain.StatusApplied, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := apps.ChangeStatus(ctx, f.workspace.ID, app.ID, domain.StatusRejected, nil); err != nil {
		t.Fatal(err)
	}
	a2, err := apps.AddAttempt(ctx, f.workspace.ID, app.ID, AddAttemptParams{AppliedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if a1.AttemptNumber != 1 || a2.AttemptNumber != 2 {
		t.Fatalf("attempt numbers = %d, %d", a1.AttemptNumber, a2.AttemptNumber)
	}

	attempts, err := apps.ListAttempts(ctx, f.workspace.ID, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 || attempts[0].Channel != "company site" {
		t.Fatalf("earlier attempt was not preserved: %+v", attempts)
	}

	events, err := apps.ListEvents(ctx, f.workspace.ID, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	var statusChanges []string
	for _, e := range events {
		if e.Type == domain.EventStatusChanged {
			statusChanges = append(statusChanges, string(*e.FromStatus)+"->"+string(*e.ToStatus))
		}
	}
	// The first attempt moved planned->applied; the second reopened the
	// rejected application.
	if got := strings.Join(statusChanges, ","); got != "planned->applied,applied->rejected,rejected->applied" {
		t.Fatalf("status history = %s", got)
	}
	if len(events) != 6 { // created, 3 status changes, 2 attempts
		t.Fatalf("events = %d, want 6", len(events))
	}

	_, err = apps.ChangeStatus(ctx, f.workspace.ID, app.ID, domain.StatusAccepted, nil)
	var notAllowed domain.ErrTransitionNotAllowed
	if !errors.As(err, &notAllowed) || !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("applied->accepted error = %v, want transition conflict", err)
	}

	_, err = apps.ChangeStatus(ctx, f.workspace.ID, app.ID, "ghosted", nil)
	requireErr(t, err, domain.ErrInvalid)

	_, err = apps.AddEvent(ctx, f.workspace.ID, app.ID, ScheduleEventParams{Type: domain.EventStatusChanged, OccurredAt: time.Now()})
	requireErr(t, err, domain.ErrInvalid)
	interview, err := apps.AddEvent(ctx, f.workspace.ID, app.ID, ScheduleEventParams{
		Type: domain.EventInterview, OccurredAt: time.Now().Add(48 * time.Hour),
		AttemptID: &a2.ID, Details: map[string]any{"round": "technical"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if interview.Details["round"] != "technical" {
		t.Fatalf("details = %v", interview.Details)
	}

	status := domain.StatusApplied
	list, err := apps.List(ctx, f.workspace.ID, &status)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("filtered applications = %d, want 1", len(list))
	}
}

func TestEventAttemptMustBelongToApplication(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	apps := NewApplicationRepository(testPool)
	jobs := NewJobRepository(testPool)

	other, _, err := jobs.UpsertListing(ctx, IngestedListing{
		SourceID: f.source.ID, ExternalID: uniq("ext"), URL: "https://jobs.example.com/2",
		DedupeKey: uniq("dk"), Title: "Frontend Engineer", CompanyName: "Acme",
	})
	if err != nil {
		t.Fatal(err)
	}
	appA, err := apps.Create(ctx, f.workspace.ID, f.job.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	appB, err := apps.Create(ctx, f.workspace.ID, other.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	attemptA, err := apps.AddAttempt(ctx, f.workspace.ID, appA.ID, AddAttemptParams{AppliedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}

	_, err = apps.AddEvent(ctx, f.workspace.ID, appB.ID, ScheduleEventParams{
		Type: domain.EventFollowUp, OccurredAt: time.Now(), AttemptID: &attemptA.ID,
	})
	requireErr(t, err, domain.ErrNotFound)
}

func TestConcurrentAttemptsGetSequentialNumbers(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	apps := NewApplicationRepository(testPool)

	app, err := apps.Create(ctx, f.workspace.ID, f.job.ID, nil)
	if err != nil {
		t.Fatal(err)
	}

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = apps.AddAttempt(ctx, f.workspace.ID, app.ID, AddAttemptParams{AppliedAt: time.Now()})
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	attempts, err := apps.ListAttempts(ctx, f.workspace.ID, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range attempts {
		if a.AttemptNumber != i+1 {
			t.Fatalf("attempt %d has number %d", i, a.AttemptNumber)
		}
	}
	if len(attempts) != n {
		t.Fatalf("attempts = %d, want %d", len(attempts), n)
	}
}

func TestWorkspaceIsolation(t *testing.T) {
	a := newFixture(t)
	b := newFixture(t)
	ctx := context.Background()
	apps := NewApplicationRepository(testPool)
	annotations := NewAnnotationRepository(testPool)

	appA, err := apps.Create(ctx, a.workspace.ID, a.job.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	tagB, err := annotations.CreateTag(ctx, b.workspace.ID, "urgent")
	if err != nil {
		t.Fatal(err)
	}

	_, err = apps.Get(ctx, b.workspace.ID, appA.ID)
	requireErr(t, err, domain.ErrNotFound)
	_, err = apps.ChangeStatus(ctx, b.workspace.ID, appA.ID, domain.StatusWithdrawn, nil)
	requireErr(t, err, domain.ErrNotFound)
	_, err = apps.AddAttempt(ctx, b.workspace.ID, appA.ID, AddAttemptParams{AppliedAt: time.Now()})
	requireErr(t, err, domain.ErrNotFound)
	_, err = apps.AddEvent(ctx, b.workspace.ID, appA.ID, ScheduleEventParams{Type: domain.EventDeadline, OccurredAt: time.Now()})
	requireErr(t, err, domain.ErrNotFound)

	// Mixing workspaces is rejected by composite foreign keys.
	requireErr(t, annotations.TagApplication(ctx, a.workspace.ID, appA.ID, tagB.ID), domain.ErrNotFound)
	requireErr(t, annotations.TagApplication(ctx, b.workspace.ID, appA.ID, tagB.ID), domain.ErrNotFound)
	_, err = annotations.AddNote(ctx, b.workspace.ID, appA.ID, nil, "not mine")
	requireErr(t, err, domain.ErrNotFound)

	events, err := apps.ListEvents(ctx, b.workspace.ID, appA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("workspace B can read %d events of workspace A", len(events))
	}
	list, err := apps.List(ctx, b.workspace.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("workspace B lists %d applications of workspace A", len(list))
	}
}

func TestTagsAndNotes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	apps := NewApplicationRepository(testPool)
	annotations := NewAnnotationRepository(testPool)

	app, err := apps.Create(ctx, f.workspace.ID, f.job.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	tag, err := annotations.CreateTag(ctx, f.workspace.ID, " Remote ")
	if err != nil {
		t.Fatal(err)
	}
	_, err = annotations.CreateTag(ctx, f.workspace.ID, "remote")
	requireErr(t, err, domain.ErrConflict)

	for range 2 {
		if err := annotations.TagApplication(ctx, f.workspace.ID, app.ID, tag.ID); err != nil {
			t.Fatal(err)
		}
	}
	tags, err := annotations.ListApplicationTags(ctx, f.workspace.ID, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Name != "Remote" {
		t.Fatalf("tags = %+v", tags)
	}

	note, err := annotations.AddNote(ctx, f.workspace.ID, app.ID, &f.user.ID, "Recruiter call went well")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := annotations.UpdateNote(ctx, f.workspace.ID, app.ID, note.ID, "Recruiter call went very well")
	if err != nil {
		t.Fatal(err)
	}
	if !updated.UpdatedAt.After(note.UpdatedAt) && !updated.UpdatedAt.Equal(note.UpdatedAt) {
		t.Fatal("updated_at moved backwards")
	}
	_, err = annotations.AddNote(ctx, f.workspace.ID, app.ID, nil, "   ")
	requireErr(t, err, domain.ErrInvalid)

	notes, err := annotations.ListNotes(ctx, f.workspace.ID, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].Body != "Recruiter call went very well" {
		t.Fatalf("notes = %+v", notes)
	}
}

func TestSearchJobs(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	jobs := NewJobRepository(testPool)

	// A unique company keeps this test independent of jobs created elsewhere.
	company := uniq("Search Co 100%_")
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	add := func(i int, title, arrangement string, tech ...string) {
		t.Helper()
		_, _, err := jobs.UpsertListing(ctx, IngestedListing{
			SourceID: f.source.ID, ExternalID: uniq("ext"), URL: "https://jobs.example.com/s",
			SeenAt: base.Add(time.Duration(i) * time.Hour), DedupeKey: uniq("dk"),
			Title: title, CompanyName: company, WorkArrangement: arrangement, Technologies: tech,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	add(1, "Go Engineer", "remote", "Go", "PostgreSQL")
	add(2, "Frontend Engineer", "hybrid", "react")
	add(3, "Platform Engineer", "remote", "go", "kubernetes")
	add(4, "Data Analyst", "onsite", "sql")

	titles := func(js []domain.Job) string {
		out := make([]string, len(js))
		for i, j := range js {
			out[i] = j.Title
		}
		return strings.Join(out, ",")
	}

	got, more, err := jobs.SearchJobs(ctx, JobFilter{Query: company, WorkArrangement: "remote", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if titles(got) != "Platform Engineer,Go Engineer" || more {
		t.Fatalf("remote jobs = %s (more=%v)", titles(got), more)
	}

	got, _, err = jobs.SearchJobs(ctx, JobFilter{Query: company, Technologies: []string{"GO", "postgresql"}, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if titles(got) != "Go Engineer" {
		t.Fatalf("technology filter = %s", titles(got))
	}

	// LIKE wildcards in the query match literally: the decoy would match an
	// unescaped "%100%_%" pattern.
	if _, _, err := jobs.UpsertListing(ctx, IngestedListing{
		SourceID: f.source.ID, ExternalID: uniq("ext"), URL: "https://jobs.example.com/d",
		DedupeKey: uniq("dk"), Title: "Decoy", CompanyName: "Decoy 1000 Corp",
	}); err != nil {
		t.Fatal(err)
	}
	got, _, err = jobs.SearchJobs(ctx, JobFilter{Query: "100%_", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range got {
		if j.CompanyName != company {
			t.Fatalf("wildcard query matched %q", j.CompanyName)
		}
	}

	// Paginate two at a time through all four jobs.
	var all []string
	var after *JobCursor
	for page := 0; ; page++ {
		got, more, err := jobs.SearchJobs(ctx, JobFilter{Query: company, Limit: 2, After: after})
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, titles(got))
		if !more {
			break
		}
		last := got[len(got)-1]
		after = &JobCursor{LastSeenAt: last.LastSeenAt, ID: last.ID}
		if page > 3 {
			t.Fatal("pagination did not terminate")
		}
	}
	if strings.Join(all, "|") != "Data Analyst,Platform Engineer|Frontend Engineer,Go Engineer" {
		t.Fatalf("pages = %v", all)
	}
}
