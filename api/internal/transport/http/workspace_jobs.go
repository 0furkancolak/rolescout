package httptransport

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/ingest"
)

var adapterName = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

var postedWindows = map[string]time.Duration{"24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour}

// multiQuery returns every value of a repeated or comma-separated query
// parameter.
func multiQuery(c fiber.Ctx, name string) []string {
	var out []string
	for _, raw := range c.Request().URI().QueryArgs().PeekMulti(name) {
		for _, v := range strings.Split(string(raw), ",") {
			if v = strings.TrimSpace(v); v != "" {
				out = append(out, v)
			}
		}
	}
	return out
}

// Workspace job cursors are opaque to clients: base64url(JSON).
func encodeJobCursor(c database.JobCursor) string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeJobCursor(s string) (*database.JobCursor, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, false
	}
	var c database.JobCursor
	if err := json.Unmarshal(raw, &c); err != nil || !isUUID(c.ID) || c.Offset < 0 {
		return nil, false
	}
	return &c, true
}

// workspaceJobFilter reads and validates the job browsing parameters shared
// by the list and facets endpoints.
func (h *handlers) workspaceJobFilter(c fiber.Ctx, paging bool) (database.JobFilter, error) {
	var v domain.ValidationError
	f := database.JobFilter{
		WorkspaceID:     workspaceID(c),
		Query:           strings.TrimSpace(c.Query("q")),
		ProfileID:       c.Query("profile"),
		Sort:            c.Query("sort"),
		WorkArrangement: c.Query("arrangement"),
		EmploymentType:  c.Query("employment_type"),
		Seniority:       c.Query("seniority"),
		Technologies:    multiQuery(c, "technology"),
	}
	if len([]rune(f.Query)) > maxQueryLength {
		v.Add("q", "must be at most %d characters", maxQueryLength)
	}
	if f.ProfileID != "" && !isUUID(f.ProfileID) {
		v.Add("profile", "must be a profile ID")
	}
	switch f.Sort {
	case "", "recent":
		f.Sort = "recent"
	case "score":
		if f.ProfileID == "" {
			v.Add("sort", "score needs a profile")
		}
	case "relevance":
		if f.Query == "" {
			v.Add("sort", "relevance needs a search query")
		}
	default:
		v.Add("sort", "must be recent, score or relevance")
	}
	if raw := c.Query("posted"); raw != "" {
		d, ok := postedWindows[raw]
		if !ok {
			v.Add("posted", "must be 24h, 7d or 30d")
		}
		f.PostedWithin = d
	}
	for _, p := range []struct {
		field, value string
		allowed      []string
	}{
		{"arrangement", f.WorkArrangement, domain.WorkArrangements},
		{"employment_type", f.EmploymentType, domain.EmploymentTypes},
		{"seniority", f.Seniority, domain.SeniorityLevels},
	} {
		if p.value != "" && !domain.OneOf(p.value, p.allowed) {
			v.Add(p.field, "must be one of %s", strings.Join(p.allowed, ", "))
		}
	}
	if len(f.Technologies) > maxTechnologies {
		v.Add("technology", "must list at most %d technologies", maxTechnologies)
	}
	for _, cc := range multiQuery(c, "country") {
		cc = strings.ToUpper(cc)
		if !countryPattern.MatchString(cc) {
			v.Add("country", "must be ISO 3166-1 alpha-2 codes such as TR")
			break
		}
		f.Countries = append(f.Countries, cc)
	}
	for _, name := range multiQuery(c, "region") {
		codes := ingest.RegionCountries(name)
		if codes == nil {
			v.Add("region", "must be one of %s", strings.Join(ingest.RegionNames(), ", "))
			break
		}
		for _, cc := range codes {
			if !slices.Contains(f.Countries, cc) {
				f.Countries = append(f.Countries, cc)
			}
		}
	}
	if loc := strings.TrimSpace(c.Query("location")); loc != "" {
		if utf8.RuneCountInString(loc) > 80 {
			v.Add("location", "must be at most 80 characters")
		}
		f.Location = ingest.Fold(loc)
	}
	if ats := c.Query("ats"); ats != "" {
		if !adapterName.MatchString(ats) {
			v.Add("ats", "must be a job board system such as lever")
		}
		f.ATS = ats
	}
	for _, id := range multiQuery(c, "company") {
		if !isUUID(id) {
			v.Add("company", "must be company IDs")
			break
		}
		f.CompanyIDs = append(f.CompanyIDs, id)
	}
	switch c.Query("closed") {
	case "", "false":
	case "true":
		f.IncludeClosed = true
	default:
		v.Add("closed", "must be true or false")
	}
	if paging {
		f.Limit = parseLimit(c.Query("limit"), &v)
		if raw := c.Query("cursor"); raw != "" {
			cursor, ok := decodeJobCursor(raw)
			if !ok {
				v.Add("cursor", "is not a valid cursor")
			}
			f.After = cursor
		}
	}
	if err := v.Err(); err != nil {
		return f, err
	}
	if f.ProfileID != "" {
		if _, err := h.Research.GetProfile(c.Context(), f.WorkspaceID, f.ProfileID); err != nil {
			return f, err
		}
	}
	return f, nil
}

func (h *handlers) listWorkspaceJobs(c fiber.Ctx) error {
	f, err := h.workspaceJobFilter(c, true)
	if err != nil {
		return err
	}
	jobs, more, err := h.Jobs.WorkspaceJobs(c.Context(), f)
	if err != nil {
		return err
	}
	resp := pageResponse[workspaceJobDTO]{Data: mapSlice(jobs, toWorkspaceJob)}
	h.attachAI(c, f.ProfileID, resp.Data)
	h.attachFit(c, f.ProfileID, resp.Data)
	if more && (f.Sort != "relevance" || f.After == nil || f.After.Offset+f.Limit < database.MaxRelevanceOffset) {
		next := encodeJobCursor(database.CursorAfter(f, jobs[len(jobs)-1]))
		resp.NextCursor = &next
	}
	return c.JSON(resp)
}

func (h *handlers) workspaceJobFacets(c fiber.Ctx) error {
	f, err := h.workspaceJobFilter(c, false)
	if err != nil {
		return err
	}
	fc, err := h.Jobs.Facets(c.Context(), f)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[facetsDTO]{Data: toFacets(fc)})
}
