package httptransport

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func newTestApp(d Deps) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	RegisterAPIRoutes(app, d)
	return app
}

type testResponse struct {
	Status int
	Body   map[string]any
	Raw    string
}

func (r testResponse) errorCode() string {
	e, _ := r.Body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func (r testResponse) data() map[string]any {
	d, _ := r.Body["data"].(map[string]any)
	return d
}

func (r testResponse) list() []any {
	d, _ := r.Body["data"].([]any)
	return d
}

// call sends a request; body may be nil, a string (sent verbatim) or a value
// encoded as JSON.
func call(t *testing.T, app *fiber.App, method, path string, body any) testResponse {
	t.Helper()
	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		reader = bytes.NewBufferString(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := testResponse{Status: resp.StatusCode, Raw: string(raw)}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.Body)
	}
	return out
}

func expectStatus(t *testing.T, r testResponse, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("status = %d, want %d; body: %s", r.Status, want, r.Raw)
	}
}
