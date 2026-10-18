package research

import (
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
)

var now = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

func defaultProfile() domain.ResearchProfile {
	return domain.ResearchProfile{
		Name: "Full Stack Developer", TitleKeywords: []string{"full stack", "fullstack", "full-stack"},
		Technologies: []string{"react", "typescript", "node.js", "javascript", "postgresql"}, Remote: domain.RemoteAny,
	}
}

func job(title, arrangement string, techs ...string) domain.Job {
	return domain.Job{Title: title, WorkArrangement: arrangement, Seniority: "unknown", Technologies: techs, FirstSeenAt: now.Add(-48 * time.Hour)}
}

func codes(rs []domain.MatchReason) map[string]domain.MatchReason {
	m := map[string]domain.MatchReason{}
	for _, r := range rs {
		m[r.Code] = r
	}
	return m
}

func TestScoreStrongFullStackMatch(t *testing.T) {
	j := job("Senior Full Stack Engineer", "remote", "react", "node.js", "typescript")
	j.Seniority = "senior"
	score, reasons, ok := Score(defaultProfile(), j, now)
	if !ok || score < 85 || score > 100 {
		t.Fatalf("score = %d ok = %v", score, ok)
	}
	c := codes(reasons)
	if c["title"].Params["value"] != "full stack" || c["tech"].Params["matched"] != "3" || c["tech"].Params["total"] != "5" {
		t.Errorf("reasons = %+v", reasons)
	}
	if _, ok := c["remote"]; !ok {
		t.Errorf("missing remote reason: %+v", reasons)
	}
	if _, ok := c["fresh"]; !ok {
		t.Errorf("missing fresh reason: %+v", reasons)
	}
}

func TestScoreTitleVariants(t *testing.T) {
	for _, title := range []string{"Fullstack Developer", "Full-Stack Engineer (React/Node)", "Software Engineer, Full Stack", "Product Engineer", "Full Stack Geliştirici"} {
		_, reasons, _ := Score(defaultProfile(), job(title, "unknown"), now)
		if _, ok := codes(reasons)["title"]; !ok {
			t.Errorf("%q: no title reason in %+v", title, reasons)
		}
	}
}

func TestScoreUnrelatedJobIsLow(t *testing.T) {
	score, _, _ := Score(defaultProfile(), job("Accountant", "onsite"), now)
	if score >= 40 {
		t.Fatalf("accountant score = %d, want < 40", score)
	}
}

func TestScoreExcludedKeyword(t *testing.T) {
	p := defaultProfile()
	p.ExcludedKeywords = []string{"php"}
	j := job("Full Stack Developer", "remote", "react")
	j.Description = "We use PHP and Laravel."
	if _, _, ok := Score(p, j, now); ok {
		t.Fatal("excluded keyword must remove the job")
	}
}

func TestScoreWithoutTechnologies(t *testing.T) {
	p := defaultProfile()
	p.Technologies = nil
	a, _, _ := Score(p, job("Full Stack Developer", "remote"), now)
	b, _, _ := Score(p, job("Full Stack Developer", "remote", "react", "go"), now)
	if a != b {
		t.Errorf("without profile technologies, job technologies must not matter: %d vs %d", a, b)
	}
}

func TestScoreRemoteOnly(t *testing.T) {
	p := defaultProfile()
	p.Remote = domain.RemoteOnly
	onsite := job("Full Stack Developer", "onsite")
	onsite.CountryCode = strp("DE")
	if _, _, ok := Score(p, onsite, now); ok {
		t.Fatal("remote-only profile must drop onsite jobs outside its countries")
	}
	p.Countries = []string{"DE"}
	score, reasons, ok := Score(p, onsite, now)
	if !ok || codes(reasons)["country"].Params["value"] != "DE" || score <= 0 {
		t.Fatalf("onsite in a profile country: score %d ok %v reasons %+v", score, ok, reasons)
	}
}

func TestScoreHybridAndSeniority(t *testing.T) {
	p := defaultProfile()
	p.Remote = domain.RemoteOrHybrid
	p.Seniority = []string{"senior", "lead"}
	h := job("Full Stack Developer", "hybrid")
	h.Seniority = "junior"
	_, reasons, _ := Score(p, h, now)
	c := codes(reasons)
	if _, ok := c["hybrid"]; !ok {
		t.Errorf("missing hybrid reason: %+v", reasons)
	}
	if _, ok := c["seniority"]; ok {
		t.Errorf("junior job must not get a seniority reason: %+v", reasons)
	}
	h.Seniority = "senior"
	_, reasons, _ = Score(p, h, now)
	if codes(reasons)["seniority"].Params["value"] != "senior" {
		t.Errorf("seniority reason = %+v", reasons)
	}
}

func TestScoreNeverExceeds100(t *testing.T) {
	p := defaultProfile()
	p.Countries = []string{"TR"}
	j := job("Full Stack Developer Full Stack", "remote", "react", "typescript", "node.js", "javascript", "postgresql")
	j.CountryCode = strp("TR")
	j.FirstSeenAt = now
	if score, _, _ := Score(p, j, now); score != 100 {
		t.Errorf("perfect score = %d, want 100", score)
	}
}

// Technologies found by the local model reach the score through the job's
// effective technology list. That can only add up to the technology weight,
// never more, never beyond 100, and without them the score is the old one.
func TestEnrichedTechnologiesRaiseTheScoreByAtMostTheTechnologyWeight(t *testing.T) {
	p := defaultProfile()
	rules := job("Full Stack Engineer", "remote", "react")
	enriched := job("Full Stack Engineer", "remote", "react", "typescript", "node.js", "javascript", "postgresql", "rust", "docker")

	base, _, _ := Score(p, rules, now)
	raised, reasons, _ := Score(p, enriched, now)
	if raised <= base {
		t.Fatalf("more matching technologies must raise the score: %d vs %d", raised, base)
	}
	if raised-base > techWeight {
		t.Fatalf("raised by %d, more than the technology weight %d", raised-base, techWeight)
	}
	if raised > 100 || codes(reasons)["tech"].Params["matched"] != "5" {
		t.Fatalf("score %d reasons %+v", raised, reasons)
	}
	// A job with no extra technologies scores exactly as before.
	same, _, _ := Score(p, job("Full Stack Engineer", "remote", "react"), now)
	if same != base {
		t.Fatalf("unchanged job scored %d, want %d", same, base)
	}
}
