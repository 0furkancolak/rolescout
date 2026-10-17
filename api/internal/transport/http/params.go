package httptransport

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isUUID(s string) bool { return uuidPattern.MatchString(s) }

// pathID returns a UUID path parameter. A malformed ID cannot name an existing
// resource, so it is reported as not found.
func pathID(c fiber.Ctx, name string) (string, error) {
	id := c.Params(name)
	if !isUUID(id) {
		return "", fmt.Errorf("path %s %q: %w", name, id, domain.ErrNotFound)
	}
	return strings.ToLower(id), nil
}

// decodeJSON strictly decodes the request body: unknown fields and trailing
// data are rejected. The body must be declared as JSON, which browsers cannot
// send cross-origin without a CORS preflight; this backs up the SameSite
// cookie against forged requests.
func decodeJSON(c fiber.Ctx, dst any) error {
	if mt, _, _ := strings.Cut(c.Get(fiber.HeaderContentType), ";"); !strings.EqualFold(strings.TrimSpace(mt), fiber.MIMEApplicationJSON) {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "Content-Type must be application/json.")
	}
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if err == io.EOF {
			return badRequest("request body is required")
		}
		return badRequest("malformed JSON body: %v", err)
	}
	if dec.More() {
		return badRequest("request body must contain a single JSON object")
	}
	return nil
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

func parseLimit(raw string, v *domain.ValidationError) int {
	if raw == "" {
		return defaultPageSize
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxPageSize {
		v.Add("limit", "must be an integer between 1 and %d", maxPageSize)
		return defaultPageSize
	}
	return n
}

// Cursors are opaque to clients: base64url("<RFC3339Nano>|<uuid>").
func encodeCursor(c database.JobCursor) string {
	raw := c.LastSeenAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (*database.JobCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	ts, id, ok := strings.Cut(string(raw), "|")
	if !ok || !isUUID(id) {
		return nil, fmt.Errorf("malformed cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, err
	}
	return &database.JobCursor{LastSeenAt: t, ID: id}, nil
}

// requiredText trims s and checks it is non-empty and at most max characters.
func requiredText(v *domain.ValidationError, field, s string, max int) string {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		v.Add(field, "is required")
	case len([]rune(s)) > max:
		v.Add(field, "must be at most %d characters", max)
	}
	return s
}
