package httptransport

import (
	"context"
	"testing"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/sysstat"
)

type stubSystem struct{}

func (stubSystem) Report(context.Context) (sysstat.Report, error) {
	used := uint64(100)
	return sysstat.Report{HostTotal: 1000, Services: []sysstat.Service{{Name: "api", Used: &used}}}, nil
}

func TestSystemStatusIsForAdministrators(t *testing.T) {
	admin := newTestApp(Deps{Auth: fakeAuthAs(domain.User{ID: "a", IsAdmin: true}), System: stubSystem{}})
	r := client{token: testToken}.call(t, admin, "GET", "/api/v1/admin/system", nil)
	expectStatus(t, r, 200)
	if r.data()["host_total_bytes"] != float64(1000) {
		t.Fatalf("data = %v", r.data())
	}
	member := newTestApp(Deps{Auth: fakeAuthAs(domain.User{ID: "m"}), System: stubSystem{}})
	expectStatus(t, client{token: testToken}.call(t, member, "GET", "/api/v1/admin/system", nil), 403)
}
