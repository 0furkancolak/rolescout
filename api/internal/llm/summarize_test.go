package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/rolescout/api/internal/llm/llmtest"
)

func summarizer(t *testing.T, reply string) *Client {
	t.Helper()
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: reply})
	return NewClient(Options{BaseURL: srv.BaseURL(), Model: "m", Timeout: 2 * time.Second})
}

func TestSummarizeKeepsAGroundedSummary(t *testing.T) {
	c := summarizer(t, `{"summary":"Acme arar: React ve Node.js ile ürün geliştirecek bir mühendis."}`)
	got, err := Summarize(context.Background(), c, "Engineer", "Acme", "We build with React and Node.js every day.", "tr")
	if err != nil || got == "" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestSummarizeRejectsWhatThePostingDoesNotSay(t *testing.T) {
	for name, reply := range map[string]string{
		"technology": `{"summary":"The role uses Kafka and React."}`,
		"number":     `{"summary":"The team has 250 engineers."}`,
		"link":       `{"summary":"See https://acme.example for more."}`,
		"too long":   `{"summary":"One. Two. Three. Four."}`,
		"empty":      `{"summary":" "}`,
		"not json":   `nope`,
	} {
		c := summarizer(t, reply)
		if _, err := Summarize(context.Background(), c, "Engineer", "Acme", "We build with React.", "en"); !errors.Is(err, ErrInvalidOutput) {
			t.Errorf("%s: err = %v, want ErrInvalidOutput", name, err)
		}
	}
}
