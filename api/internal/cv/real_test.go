package cv

import (
	"os"
	"strings"
	"testing"
)

// TestRealFiles reads files named in CV_SAMPLE_DIR; it is skipped without it.
func TestRealFiles(t *testing.T) {
	dir := os.Getenv("CV_SAMPLE_DIR")
	if dir == "" {
		t.Skip("CV_SAMPLE_DIR not set")
	}
	for _, name := range []string{"cv.pdf", "cv.docx", "cv.txt"} {
		data, err := os.ReadFile(dir + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		text, err := Extract(name, data)
		if err != nil || !strings.Contains(text, "PostgreSQL") {
			t.Errorf("%s: %q %v", name, text, err)
		}
	}
}
