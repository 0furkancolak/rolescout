// Package hnhiring reads the job posts of the monthly "Ask HN: Who is
// hiring?" thread through the HN Search (Algolia) API. See
// docs/sources/hnhiring.md.
package hnhiring

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// maxPages bounds the pages of comments read per thread (1000 comments each).
const maxPages = 5

// Adapter reads the newest thread(s). Config: {} or {"threads": "2"}.
type Adapter struct {
	Client  *crawl.Client
	BaseURL string // overrides https://hn.algolia.com (for tests)
}

func New(c *crawl.Client) *Adapter { return &Adapter{Client: c} }

var threads = regexp.MustCompile(`^[1-3]$`)

func (a *Adapter) Validate(config map[string]any) error {
	return ingest.CheckConfig(config, ingest.Field{Name: "threads", Pattern: threads, Optional: true})
}

type stories struct {
	Hits []struct {
		ObjectID string `json:"objectID"`
		Title    string `json:"title"`
	} `json:"hits"`
}

type comments struct {
	NbPages int `json:"nbPages"`
	Hits    []struct {
		ObjectID    string `json:"objectID"`
		ParentID    int64  `json:"parent_id"`
		StoryID     int64  `json:"story_id"`
		CommentText string `json:"comment_text"`
		CreatedAtI  int64  `json:"created_at_i"`
	} `json:"hits"`
}

func (a *Adapter) Fetch(ctx context.Context, config map[string]any) ([]ingest.RawListing, error) {
	if err := a.Validate(config); err != nil {
		return nil, err
	}
	base := a.BaseURL
	if base == "" {
		base = "https://hn.algolia.com"
	}
	want := 1
	if n, err := strconv.Atoi(ingest.ConfigString(config, "threads")); err == nil {
		want = n
	}
	var st stories
	q := url.Values{"tags": {"story,author_whoishiring"}, "hitsPerPage": {"12"}}
	if err := ingest.FetchJSON(ctx, a.Client, base+"/api/v1/search_by_date?"+q.Encode(), &st); err != nil {
		return nil, err
	}
	var out []ingest.RawListing
	incomplete := ""
	read := 0
	for _, s := range st.Hits {
		if read >= want {
			break
		}
		if !strings.Contains(strings.ToLower(s.Title), "who is hiring") {
			continue
		}
		storyID, err := strconv.ParseInt(s.ObjectID, 10, 64)
		if err != nil {
			continue
		}
		read++
		for p := 0; p < maxPages; p++ {
			cq := url.Values{"tags": {"comment,story_" + s.ObjectID}, "hitsPerPage": {"1000"}, "page": {strconv.Itoa(p)}}
			var cs comments
			if err := ingest.FetchJSON(ctx, a.Client, base+"/api/v1/search_by_date?"+cq.Encode(), &cs); err != nil {
				if len(out) == 0 {
					return nil, err
				}
				incomplete = fmt.Sprintf("comments page %d of %q could not be read: %v", p+1, s.Title, err)
				break
			}
			for _, h := range cs.Hits {
				if h.ParentID != storyID { // replies are not job posts
					continue
				}
				if l, ok := parsePost(h.ObjectID, h.CommentText, h.CreatedAtI); ok {
					out = append(out, l)
				}
			}
			if p+1 >= cs.NbPages {
				break
			}
			if p+1 == maxPages {
				incomplete = fmt.Sprintf("thread %q has more than %d comments", s.Title, maxPages*1000)
			}
		}
	}
	if len(out) == 0 {
		return nil, ingest.Incomplete("no job posts found")
	}
	if incomplete != "" {
		return out, ingest.Incomplete("%s", incomplete)
	}
	// Posts of older threads are not read, so they are never closed.
	return out, ingest.Incomplete("newest %d hiring thread(s) only: older posts are not closed", want)
}

var (
	roleWords  = regexp.MustCompile(`(?i)\b(engineer|engineering|developer|designer|scientist|manager|analyst|architect|lead|sre|devops|researcher|product|sales|marketing|recruiter|cto|intern|programmer|administrator|specialist|consultant|director|head of|founder|support|writer|qa|full[- ]?stack|front[- ]?end|back[- ]?end|data|ml|ai)\b`)
	placeWords = regexp.MustCompile(`(?i)\b(remote|onsite|on-site|hybrid|worldwide|anywhere|us|usa|uk|eu|emea|europe|germany|turkey|türkiye|istanbul|london|berlin|paris|amsterdam|new york|nyc|san francisco|sf|bay area|seattle|boston|austin|toronto|vancouver|canada|india|singapore|australia|sydney)\b|,\s*[A-Z]{2}\b`)
	tag        = regexp.MustCompile(`<[^>]*>`)
	paragraph  = regexp.MustCompile(`(?i)<p>|<br\s*/?>`)
)

// parsePost reads one comment. Posts start with a header line such as
// "Acme | Senior Go Engineer | Berlin, Germany | Remote | Full-time"; a
// comment without such a line is not a job post.
func parsePost(id, text string, created int64) (ingest.RawListing, bool) {
	header := paragraph.Split(text, 2)[0]
	header = strings.TrimSpace(html.UnescapeString(tag.ReplaceAllString(header, "")))
	parts := strings.Split(header, "|")
	if len(parts) < 3 {
		return ingest.RawListing{}, false
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	company := parts[0]
	title, location := "", ""
	for _, p := range parts[1:] {
		switch {
		case title == "" && roleWords.MatchString(p) && len(p) <= 120:
			title = p
		case location == "" && placeWords.MatchString(p) && len(p) <= 100:
			location = p
		}
	}
	if title == "" && !strings.HasPrefix(strings.ToLower(parts[1]), "http") && !placeWords.MatchString(parts[1]) && len(parts[1]) <= 80 {
		title = parts[1]
	}
	if company == "" || title == "" || len(company) > 100 {
		return ingest.RawListing{}, false
	}
	company = ingest.CleanCompanyName(company)
	l := ingest.RawListing{
		ExternalID:  id,
		URL:         "https://news.ycombinator.com/item?id=" + id,
		Title:       title,
		Company:     company,
		Location:    location,
		Description: text,
	}
	if created > 0 {
		t := time.Unix(created, 0).UTC()
		l.PostedAt = &t
	}
	if strings.Contains(strings.ToLower(header), "remote") && !strings.Contains(strings.ToLower(header), "no remote") {
		l.Arrangement = "remote"
	}
	return l, true
}
