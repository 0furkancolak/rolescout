package htmlpage

import (
	"os"
	"strings"
	"testing"
)

func TestEadamKariyerPage(t *testing.T) {
	body, err := os.ReadFile("testdata/eadam_kariyer.html")
	if err != nil {
		t.Fatal(err)
	}
	const page = "https://e-adam.com/kariyer"
	if !LooksLikeJobList(body, page) {
		t.Fatal("the career page is not seen as a job list")
	}
	cards := Extract(body, page)
	var titles []string
	for _, c := range cards {
		titles = append(titles, c.Title)
	}
	want := "Grafik Tasarım|Inf & UGC Marketing Uzmanı|Dijital Pazarlama Uzmanı|SEO Uzmanı|Müşteri Hizmetleri|Front-end Developer|Back-end Developer|Metin Yazarı"
	if got := strings.Join(titles, "|"); got != want {
		t.Fatalf("titles = %s", got)
	}
	for _, c := range cards {
		if c.Location != "" {
			t.Errorf("%s: location %q is an icon, not a place", c.Title, c.Location)
		}
	}
	c := cards[5]
	if c.EmploymentType != "full-time" || c.Arrangement != "onsite" || !strings.Contains(c.Description, "framework") {
		t.Fatalf("card = %+v", c)
	}
	if !strings.HasPrefix(c.URL, "https://e-adam.com/kariyer#") || c.ID == "" {
		t.Fatalf("a card without a page of its own should point at the list: %+v", c)
	}
}

func TestMarketingPageIsNotAJobList(t *testing.T) {
	page := `<html><body><h1>Kredi</h1><div><h2>İhtiyaç Kredisi</h2><p>Faiz oranları düşük.</p><a href="/basvuru">Hemen Başvur</a></div>
		<div><h2>Konut Kredisi</h2><a href="/konut">Başvur</a></div></body></html>`
	if LooksLikeJobList([]byte(page), "https://bank.example/krediler") {
		t.Fatal("a loan page with apply buttons must not be read as jobs")
	}
}

func TestJobLinksListUnderCareerPath(t *testing.T) {
	page := `<html><body><ul>
		<li><a href="/kariyer/backend-developer">Backend Developer</a></li>
		<li><a href="/kariyer/ui-designer">UI Designer</a></li>
		<li><a href="/hakkimizda">Hakkımızda</a></li></ul></body></html>`
	cards := Extract([]byte(page), "https://acme.example/kariyer")
	if len(cards) != 2 || cards[0].URL != "https://acme.example/kariyer/backend-developer" {
		t.Fatalf("cards = %+v", cards)
	}
	if got := Extract([]byte(page), "https://acme.example/hakkimizda"); len(got) != 0 {
		t.Fatalf("links off a career path are not jobs: %+v", got)
	}
}

func TestCardWithOwnPageAndApplyLink(t *testing.T) {
	page := `<html><body>
		<article><h3><a href="/jobs/1">Data Engineer</a></h3><span>Remote</span><span>Full time</span><a href="mailto:hr@x.example">Apply</a></article>
		<article><h3><a href="/jobs/2">Designer</a></h3><span>Hybrid</span><a href="mailto:hr@x.example">Apply</a></article></body></html>`
	cards := Extract([]byte(page), "https://x.example/careers")
	if len(cards) != 2 || cards[0].URL != "https://x.example/jobs/1" || cards[0].Arrangement != "remote" || cards[1].Arrangement != "hybrid" {
		t.Fatalf("cards = %+v", cards)
	}
}
