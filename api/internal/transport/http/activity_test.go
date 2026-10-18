package httptransport

import (
	"context"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
)

type fakeActivity struct {
	ActivityStore
	gotWorkspace string
	gotDays      int
	result       domain.Activity
}

func (f *fakeActivity) Activity(_ context.Context, workspaceID string, days int, _ time.Time) (domain.Activity, error) {
	f.gotWorkspace, f.gotDays = workspaceID, days
	return f.result, nil
}

func TestActivitySeries(t *testing.T) {
	fa := &fakeActivity{result: domain.Activity{
		Days:                []string{"2026-10-16", "2026-10-17", "2026-10-18"},
		NewMatches:          []int{0, 2, 5},
		JobsCollected:       []int{10, 0, 3},
		ApplicationsCreated: []int{0, 0, 1},
		UpcomingInterviews:  2,
	}}
	app := newTestApp(Deps{Activity: fa, Workspaces: allowAll})

	r := call(t, app, "GET", wsPath+"/activity", nil)
	expectStatus(t, r, 200)
	if fa.gotDays != 14 || fa.gotWorkspace == "" {
		t.Fatalf("defaults: days=%d workspace=%q", fa.gotDays, fa.gotWorkspace)
	}
	d := r.data()
	if got := d["new_matches"].([]any); len(got) != 3 || got[2].(float64) != 5 {
		t.Fatalf("new_matches = %v", d["new_matches"])
	}
	if d["days"].([]any)[0] != "2026-10-16" || d["upcoming_interviews"].(float64) != 2 {
		t.Fatalf("body = %s", r.Raw)
	}
	for _, key := range []string{"jobs_collected", "applications_created"} {
		if len(d[key].([]any)) != 3 {
			t.Fatalf("%s = %v", key, d[key])
		}
	}

	expectStatus(t, call(t, app, "GET", wsPath+"/activity?days=90", nil), 200)
	if fa.gotDays != 90 {
		t.Fatalf("days = %d", fa.gotDays)
	}
	for _, q := range []string{"days=0", "days=91", "days=x", "days=-3"} {
		expectError(t, call(t, app, "GET", wsPath+"/activity?"+q, nil), 422, "invalid_input")
	}
}

func TestActivityHiddenFromOtherWorkspaces(t *testing.T) {
	denied := authorizer(func(string) (domain.MembershipRole, error) { return "", domain.ErrNotFound })
	app := newTestApp(Deps{Activity: &fakeActivity{}, Workspaces: denied})
	expectError(t, call(t, app, "GET", wsPath+"/activity", nil), 404, "not_found")
}
