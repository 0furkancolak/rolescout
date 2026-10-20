package cv

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

func docx(t *testing.T, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("word/document.xml")
	w.Write([]byte(`<w:document xmlns:w="x"><w:body>` + body + `</w:body></w:document>`))
	zw.Close()
	return buf.Bytes()
}

func TestExtractDocxKeepsParagraphs(t *testing.T) {
	data := docx(t, `<w:p><w:r><w:t>Ayşe Yılmaz</w:t></w:r></w:p><w:p><w:r><w:t>Senior Backend Developer, Go ve PostgreSQL</w:t></w:r></w:p>`)
	got, err := Extract("cv.docx", data)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Ayşe Yılmaz\nSenior Backend Developer, Go ve PostgreSQL" {
		t.Fatalf("text = %q", got)
	}
}

func TestExtractText(t *testing.T) {
	got, err := Extract("cv.TXT", []byte("  Backend   developer\r\n\r\n\r\n\r\nGo, Docker, Kubernetes ve daha fazlası burada yazıyor "))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "\n\n\n") || !strings.HasPrefix(got, "Backend developer") {
		t.Fatalf("text = %q", got)
	}
}

func TestExtractRejects(t *testing.T) {
	if _, err := Extract("cv.exe", []byte("x")); !errors.Is(err, ErrUnsupported) {
		t.Errorf("exe: %v", err)
	}
	if _, err := Extract("cv.txt", []byte("kısa")); !errors.Is(err, ErrNoText) {
		t.Errorf("short: %v", err)
	}
	if _, err := Extract("cv.pdf", []byte("not a pdf at all, just text that is long enough to count")); err == nil {
		t.Error("a broken PDF must fail")
	}
	if _, err := Extract("cv.docx", []byte("not a zip")); err == nil {
		t.Error("a broken docx must fail")
	}
}
