package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/enrich"
	"example.com/rolescout/api/internal/infrastructure/database"
)

type fakeMemory struct{ c domain.WorkspaceContext }

func (f *fakeMemory) Get(context.Context, string) (domain.WorkspaceContext, error) { return f.c, nil }
func (f *fakeMemory) SaveAbout(_ context.Context, _, about string) error {
	f.c.About = about
	return nil
}
func (f *fakeMemory) SaveCV(_ context.Context, _ string, p database.CVParams, at time.Time) error {
	f.c.CVFilename, f.c.CVText, f.c.CVSkills, f.c.CVTitles, f.c.CVSummary, f.c.CVUploadedAt = p.Filename, p.Text, p.Skills, p.Titles, p.Summary, &at
	return nil
}
func (f *fakeMemory) DeleteCV(context.Context, string) error {
	f.c = domain.WorkspaceContext{About: f.c.About}
	return nil
}

type fakeCVReader struct{}

func (fakeCVReader) Read(context.Context, string) enrich.CVReading {
	return enrich.CVReading{Skills: []string{"go", "postgresql"}, Titles: []string{"Backend Developer"}, Summary: "Backend developer."}
}

func upload(t *testing.T, app *fiber.App, filename, content string) testResponse {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	w, _ := mw.CreateFormFile("file", filename)
	io.WriteString(w, content)
	mw.Close()
	req := httptest.NewRequest("POST", wsPath+"/context/cv", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: testToken})
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := testResponse{Status: resp.StatusCode, Raw: string(raw)}
	_ = json.Unmarshal(raw, &out.Body)
	return out
}

func TestContextNoteAndCV(t *testing.T) {
	mem := &fakeMemory{}
	app := newTestApp(Deps{Memory: mem, CVReader: fakeCVReader{}, Workspaces: allowAll})

	r := call(t, app, "PUT", wsPath+"/context", map[string]string{"about": "  Backend işleri arıyorum, İstanbul veya uzaktan.  "})
	expectStatus(t, r, 200)
	if r.data()["about"] != "Backend işleri arıyorum, İstanbul veya uzaktan." || r.data()["cv"] != nil {
		t.Fatalf("context = %v", r.data())
	}
	expectError(t, call(t, app, "PUT", wsPath+"/context", map[string]string{"about": strings.Repeat("a", 4001)}), 422, "invalid_input")

	r = upload(t, app, "cv.txt", "Ayşe Yılmaz\nBackend developer. Go, PostgreSQL ve Docker ile servisler geliştirdim, 6 yıl deneyim.")
	expectStatus(t, r, 200)
	cv, _ := r.data()["cv"].(map[string]any)
	if cv["filename"] != "cv.txt" || cv["summary"] != "Backend developer." {
		t.Fatalf("cv = %v", cv)
	}
	if skills, _ := cv["skills"].([]any); len(skills) != 2 {
		t.Errorf("skills = %v", cv["skills"])
	}
	if r.data()["ai_read"] != true {
		t.Errorf("ai_read = %v", r.data()["ai_read"])
	}

	expectError(t, upload(t, app, "cv.exe", "MZ binary content that is long enough to pass any length check"), 422, "invalid_input")
	expectError(t, upload(t, app, "cv.txt", "kısa"), 422, "invalid_input")

	expectStatus(t, call(t, app, "DELETE", wsPath+"/context/cv", nil), 204)
	r = call(t, app, "GET", wsPath+"/context", nil)
	if r.data()["cv"] != nil || r.data()["about"] == "" {
		t.Fatalf("after delete = %v", r.data())
	}
}
