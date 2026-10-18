package httptransport

import (
	"context"
	"encoding/base64"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/discovery"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

type CompanyCatalog interface {
	List(ctx context.Context, f database.CompanyFilter) ([]domain.Company, bool, error)
	UpsertDiscovered(ctx context.Context, p database.CompanyParams) (domain.Company, bool, error)
	Block(ctx context.Context, workspaceID, companyID string) error
	Unblock(ctx context.Context, workspaceID, companyID string) error
	SetDisabled(ctx context.Context, id string, disabled bool) (domain.Company, error)
	AdapterReviews(ctx context.Context) ([]domain.AdapterReview, error)
	ReviewAdapter(ctx context.Context, adapter, note string, enabled bool) (domain.AdapterReview, error)
}

const maxURLLength = 2000

var (
	companyStatuses = []string{"pending", "active", "empty", "unreachable", "blocked_by_robots", "disabled"}
	atsPattern      = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)
)

// Company cursors are opaque to clients: base64url("<name>\x00<uuid>").
func encodeCompanyCursor(c domain.Company) string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.Name + "\x00" + c.ID))
}

func decodeCompanyCursor(s string) (name, id string, ok bool) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", "", false
	}
	name, id, ok = strings.Cut(string(raw), "\x00")
	return name, id, ok && isUUID(id)
}

func (h *handlers) listCompanies(c fiber.Ctx) error {
	var v domain.ValidationError
	f := database.CompanyFilter{
		Query:       strings.TrimSpace(c.Query("q")),
		ATS:         c.Query("ats"),
		Status:      domain.CompanyStatus(c.Query("status")),
		WorkspaceID: workspaceID(c),
		Limit:       parseLimit(c.Query("limit"), &v),
	}
	if len([]rune(f.Query)) > maxQueryLength {
		v.Add("q", "must be at most %d characters", maxQueryLength)
	}
	if f.ATS != "" && !atsPattern.MatchString(f.ATS) {
		v.Add("ats", "is not a job board system name")
	}
	if f.Status != "" && !domain.OneOf(string(f.Status), companyStatuses) {
		v.Add("status", "must be one of %s", strings.Join(companyStatuses, ", "))
	}
	if raw := c.Query("cursor"); raw != "" {
		name, id, ok := decodeCompanyCursor(raw)
		if !ok {
			v.Add("cursor", "is not a valid cursor")
		}
		f.AfterName, f.AfterID = name, id
	}
	if err := v.Err(); err != nil {
		return err
	}
	list, more, err := h.Companies.List(c.Context(), f)
	if err != nil {
		return err
	}
	resp := pageResponse[companyDTO]{Data: mapSlice(list, toCompany)}
	if more {
		next := encodeCompanyCursor(list[len(list)-1])
		resp.NextCursor = &next
	}
	return c.JSON(resp)
}

type addCompanyRequest struct {
	CareersURL string `json:"careers_url"`
	Name       string `json:"name"`
}

// addCompany puts a career page into the catalog; the next research run
// identifies its job board. Known board URLs are recognised at once.
func (h *handlers) addCompany(c fiber.Ctx) error {
	var req addCompanyRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	var v domain.ValidationError
	raw := strings.TrimSpace(req.CareersURL)
	u, err := url.Parse(raw)
	switch {
	case raw == "":
		v.Add("careers_url", "is required")
	case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "":
		v.Add("careers_url", "must be an absolute http(s) URL")
	case len(raw) > maxURLLength:
		v.Add("careers_url", "must be at most %d characters", maxURLLength)
	case !publicHost(u.Hostname()):
		v.Add("careers_url", "must be a public internet address")
	}
	name := strings.TrimSpace(req.Name)
	if len([]rune(name)) > maxProfileName {
		v.Add("name", "must be at most %d characters", maxProfileName)
	}
	if err := v.Err(); err != nil {
		return err
	}
	params := database.CompanyParams{Name: name, CareersURL: raw, ATS: "unknown", Origin: "manual"}
	if b, ok := discovery.ParseBoardURL(raw); ok {
		params.ATS, params.ATSID = b.ATS, &b.ID
		if params.Name == "" {
			params.Name = b.ID
		}
	}
	if params.Name == "" {
		params.Name = strings.TrimPrefix(u.Hostname(), "www.")
	}
	co, _, err := h.Companies.UpsertDiscovered(c.Context(), params)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(dataResponse[companyDTO]{Data: toCompany(co)})
}

func (h *handlers) blockCompany(c fiber.Ctx) error {
	id, err := pathID(c, "companyID")
	if err != nil {
		return err
	}
	if err := h.Companies.Block(c.Context(), workspaceID(c), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *handlers) unblockCompany(c fiber.Ctx) error {
	id, err := pathID(c, "companyID")
	if err != nil {
		return err
	}
	if err := h.Companies.Unblock(c.Context(), workspaceID(c), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// publicHost rejects names and addresses that point into a local network.
// The crawler also refuses them when it connects; this gives early feedback.
func publicHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if ip := net.ParseIP(host); ip != nil {
		return crawl.IsPublicIP(ip)
	}
	if !strings.Contains(host, ".") {
		return false
	}
	for _, suffix := range []string{".localhost", ".local", ".internal", ".lan", ".home", ".corp"} {
		if strings.HasSuffix(host, suffix) {
			return false
		}
	}
	return true
}
