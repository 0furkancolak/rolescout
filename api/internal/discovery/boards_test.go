package discovery

import (
	"reflect"
	"testing"
)

func TestParseBoardURL(t *testing.T) {
	tests := []struct {
		url    string
		ats    string
		id     string
		config map[string]any
	}{
		{"https://boards.greenhouse.io/acme", "greenhouse", "acme", map[string]any{"board_token": "acme"}},
		{"https://boards.greenhouse.io/acme/jobs/123", "greenhouse", "acme", map[string]any{"board_token": "acme"}},
		{"https://job-boards.greenhouse.io/acme", "greenhouse", "acme", map[string]any{"board_token": "acme"}},
		{"https://job-boards.eu.greenhouse.io/acme/jobs/1", "greenhouse", "acme", map[string]any{"board_token": "acme"}},
		{"https://boards.greenhouse.io/embed/job_board?for=acme&b=x", "greenhouse", "acme", map[string]any{"board_token": "acme"}},
		{"https://boards.greenhouse.io/embed/job_board/js?for=acme", "greenhouse", "acme", map[string]any{"board_token": "acme"}},
		{"http://boards.greenhouse.io/Acme", "greenhouse", "Acme", map[string]any{"board_token": "Acme"}},
		{"https://jobs.lever.co/acme", "lever", "acme", map[string]any{"site": "acme"}},
		{"https://jobs.lever.co/acme/5b1c-uuid/apply", "lever", "acme", map[string]any{"site": "acme"}},
		{"https://jobs.eu.lever.co/acme", "lever", "acme", map[string]any{"site": "acme", "region": "eu"}},
		{"https://jobs.ashbyhq.com/acme", "ashby", "acme", map[string]any{"board": "acme"}},
		{"https://jobs.ashbyhq.com/Acme.Inc/abc", "ashby", "Acme.Inc", map[string]any{"board": "Acme.Inc"}},
		{"https://apply.workable.com/acme/", "workable", "acme", map[string]any{"account": "acme"}},
		{"https://apply.workable.com/acme/j/ABC123/", "workable", "acme", map[string]any{"account": "acme"}},
		{"https://acme.workable.com/", "workable", "acme", map[string]any{"account": "acme"}},
		{"https://acme.recruitee.com/", "recruitee", "acme", map[string]any{"subdomain": "acme"}},
		{"https://acme.recruitee.com/o/backend-dev", "recruitee", "acme", map[string]any{"subdomain": "acme"}},
		{"https://acme.jobs.personio.de/", "personio", "acme", map[string]any{"subdomain": "acme", "tld": "de"}},
		{"https://acme.jobs.personio.com/job/1", "personio", "acme", map[string]any{"subdomain": "acme", "tld": "com"}},
		{"https://acme.bamboohr.com/careers", "bamboohr", "acme", map[string]any{"subdomain": "acme"}},
		{"https://acme.bamboohr.com/jobs/", "bamboohr", "acme", map[string]any{"subdomain": "acme"}},
		{"https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite", "workday", "nvidia/NVIDIAExternalCareerSite",
			map[string]any{"host": "nvidia.wd5.myworkdayjobs.com", "tenant": "nvidia", "site": "NVIDIAExternalCareerSite"}},
		{"https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite/job/x", "workday", "nvidia/NVIDIAExternalCareerSite",
			map[string]any{"host": "nvidia.wd5.myworkdayjobs.com", "tenant": "nvidia", "site": "NVIDIAExternalCareerSite"}},
	}
	for _, tt := range tests {
		b, ok := ParseBoardURL(tt.url)
		if !ok || b.ATS != tt.ats || b.ID != tt.id || !reflect.DeepEqual(b.Config, tt.config) {
			t.Errorf("ParseBoardURL(%q) = %+v, %v; want %s %s %v", tt.url, b, ok, tt.ats, tt.id, tt.config)
		}
	}
}

func TestParseBoardURLRejects(t *testing.T) {
	for _, u := range []string{
		"", "not a url", "ftp://boards.greenhouse.io/acme",
		"https://boards.greenhouse.io/", "https://boards.greenhouse.io/embed/job_board",
		"https://boards.greenhouse.io/v1/boards", "https://jobs.lever.co/", "https://jobs.ashbyhq.com/api/x",
		"https://apply.workable.com/j/ABC123", "https://apply.workable.com/", "https://www.workable.com/",
		"https://www.recruitee.com/", "https://app.recruitee.com/x", "https://www.bamboohr.com/careers",
		"https://acme.bamboohr.com/login", "https://nvidia.wd5.myworkdayjobs.com/", "https://nvidia.wd5.myworkdayjobs.com/wday/cxs/x",
		"https://example.com/careers", "https://jobs.smartrecruiters.com/Acme",
		"https://evil.com/boards.greenhouse.io/acme",
	} {
		if b, ok := ParseBoardURL(u); ok {
			t.Errorf("ParseBoardURL(%q) = %+v, want no board", u, b)
		}
	}
}

func TestBoardSourceKey(t *testing.T) {
	for _, tt := range []struct {
		b    Board
		want string
	}{
		{Board{ATS: "greenhouse", ID: "Acme"}, "company-greenhouse-acme"},
		{Board{ATS: "workday", ID: "nvidia/External"}, "company-workday-nvidia-external"},
		{Board{ATS: "ashby", ID: "Acme.Inc"}, "company-ashby-acme-inc"},
		{Board{ATS: "jsonld", ID: "https://a.example/x"}, "company-jsonld-https-a-example-x"},
	} {
		if got := tt.b.SourceKey(); got != tt.want {
			t.Errorf("SourceKey(%+v) = %q, want %q", tt.b, got, tt.want)
		}
	}
}
