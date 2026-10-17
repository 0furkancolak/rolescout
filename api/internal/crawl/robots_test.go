package crawl

import "testing"

func TestParseRobots(t *testing.T) {
	const body = `
# comment
User-agent: *
Disallow: /private
Allow: /private/open

User-agent: RoleScout
Disallow: /only-us
Allow: /only-us/fine$
Disallow: /*.pdf$

User-agent: OtherBot
Disallow: /
`
	r := parseRobots([]byte(body), "RoleScout")
	tests := []struct {
		path string
		want bool
	}{
		// The RoleScout group applies, so the * group is ignored.
		{"/private", true},
		{"/only-us", false},
		{"/only-us/page", false},
		{"/only-us/fine", true},
		{"/only-us/fine/more", false},
		{"/doc.pdf", false},
		{"/doc.pdf?x=1", true},
		{"/", true},
	}
	for _, tt := range tests {
		if got := r.allowed(tt.path); got != tt.want {
			t.Errorf("allowed(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestParseRobotsFallsBackToStarGroup(t *testing.T) {
	r := parseRobots([]byte("User-agent: *\nDisallow: /private\nAllow: /private/open\n"), "RoleScout")
	for path, want := range map[string]bool{
		"/private/x":    false,
		"/private/open": true,
		"/public":       true,
	} {
		if got := r.allowed(path); got != want {
			t.Errorf("allowed(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestParseRobotsSharedGroupAndEmptyDisallow(t *testing.T) {
	body := "User-agent: a\nUser-agent: rolescout\nDisallow:\n\nUser-agent: *\nDisallow: /\n"
	r := parseRobots([]byte(body), "RoleScout")
	if !r.allowed("/anything") {
		t.Error("empty Disallow in the matching group should allow everything")
	}
}
