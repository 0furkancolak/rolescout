package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	c := summarizer(t, `Acme arar: React ve Node.js ile ürün geliştirecek bir mühendis.`)
	got, err := Summarize(context.Background(), c, "Engineer", "Acme", "We build with React and Node.js every day.", "tr", nil)
	if err != nil || got == "" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestSummarizeRejectsWhatThePostingDoesNotSay(t *testing.T) {
	for name, reply := range map[string]string{
		"technology": `The role uses Kafka and React.`,
		"number":     `The team has 250 engineers.`,
		"link":       `See https://acme.example for more.`,
		"empty":      ` `,
	} {
		c := summarizer(t, reply)
		if _, err := Summarize(context.Background(), c, "Engineer", "Acme", "We build with React.", "en", nil); !errors.Is(err, ErrInvalidOutput) {
			t.Errorf("%s: err = %v, want ErrInvalidOutput", name, err)
		}
	}
}

func TestSummarizeStreamsPiecesAndChecksTheWholeText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, piece := range []string{"Acme arar: ", "React ve Node.js ", "ile çalışacak biri."} {
			b, _ := json.Marshal(piece)
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", b)
			w.(http.Flusher).Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	c := NewClient(Options{BaseURL: srv.URL, Model: "m", Timeout: 2 * time.Second})
	var pieces []string
	got, err := Summarize(context.Background(), c, "Engineer", "Acme", "We build with React and Node.js every day.", "tr", func(p string) { pieces = append(pieces, p) })
	if err != nil || got != "Acme arar: React ve Node.js ile çalışacak biri." || len(pieces) != 3 {
		t.Fatalf("got %q, %v, pieces %v", got, err, pieces)
	}
	// The same stream with a technology the posting lacks is rejected at the end.
	_, err = Summarize(context.Background(), c, "Engineer", "Acme", "We build with Go.", "tr", func(string) {})
	if !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("err = %v, want ErrInvalidOutput", err)
	}
}

func TestSummarizeCutsARunOnReplyAtTheThirdSentence(t *testing.T) {
	c := summarizer(t, "Acme builds React tools. They hire a developer. They offer a team. They are nice.")
	got, err := Summarize(context.Background(), c, "Engineer", "Acme", "We build with React.", "en", nil)
	if err != nil || got != "Acme builds React tools. They hire a developer. They offer a team." {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestFactsSummaryNamesWhatTheSourceGave(t *testing.T) {
	got := FactsSummary("Çevre Mühendisi", "Mitto", "Ankara", "onsite", "full_time", "tr")
	want := "Mitto şirketinin Çevre Mühendisi pozisyonu (Ankara, iş yerinde, tam zamanlı). İlan kaynağında ayrıntılı açıklama yayımlanmıyor; ayrıntılar için ilan bağlantısına bakın."
	if got != want {
		t.Fatalf("got %q", got)
	}
	if en := FactsSummary("Engineer", "Acme", "", "", "", "en"); en != "Engineer role at Acme. The source publishes no detailed description; see the listing link for details." {
		t.Fatalf("got %q", en)
	}
	if FactsSummary("", "Acme", "", "", "", "en") != "" {
		t.Fatal("a posting without a title has nothing to say")
	}
}

func TestExtractiveSummaryTakesTheFirstSentences(t *testing.T) {
	desc := "We build payment tools for small shops across Europe. You will design the APIs they use. Benefits include snacks. Apply today."
	if got := ExtractiveSummary(desc); got != "We build payment tools for small shops across Europe. You will design the APIs they use." {
		t.Fatalf("got %q", got)
	}
	if ExtractiveSummary("Too short.") != "" {
		t.Fatal("short text is not a summary")
	}
}
