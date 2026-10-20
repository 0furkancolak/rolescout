package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStrictParams(t *testing.T) {
	for model, want := range map[string]bool{
		"gpt-4o-mini": false, "gpt-4.1": false, "qwen3:4b": false, "llama3.1": false,
		"gpt-5": true, "gpt-5-mini": true, "gpt-6-luna": true, "o3-mini": true, "o1": true, "": false,
	} {
		if got := strictParams(model); got != want {
			t.Errorf("strictParams(%q) = %v, want %v", model, got, want)
		}
	}
}

func TestNewModelsGetCompletionTokensAndNoTemperature(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()
	c := NewClient(Options{BaseURL: srv.URL, Model: "gpt-6-luna"})
	if _, err := c.Complete(context.Background(), Request{System: "s", User: "u", MaxTokens: 400}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"max_completion_tokens":2448`) || strings.Contains(got, `"max_tokens"`) || strings.Contains(got, `"temperature"`) {
		t.Fatalf("body = %s", got)
	}
}

func TestLearnsFromTheServersComplaint(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if strings.Contains(string(b), `"max_tokens"`) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported parameter: 'max_tokens'. Use 'max_completion_tokens' instead."}}`))
			return
		}
		if strings.Contains(string(b), `"temperature"`) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported value: 'temperature' does not support 0."}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()
	// An unfamiliar name: nothing is known up front.
	c := NewClient(Options{BaseURL: srv.URL, Model: "acme-reasoner"})
	for i := 0; i < 2; i++ {
		if _, err := c.Complete(context.Background(), Request{System: "s", User: "u", MaxTokens: 100}); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	// First call: tried legacy, then fixed tokens, then fixed temperature. The
	// second call goes straight through.
	if len(bodies) != 4 {
		t.Fatalf("requests = %d, want 4: %v", len(bodies), bodies)
	}
}

func TestAPlainBadRequestIsNotRetried(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad json"}}`))
	}))
	defer srv.Close()
	c := NewClient(Options{BaseURL: srv.URL, Model: "gpt-4o-mini"})
	_, err := c.Complete(context.Background(), Request{System: "s", User: "u"})
	if err == nil || n != 1 || strings.Contains(err.Error(), "bad json") {
		t.Fatalf("err = %v after %d calls", err, n)
	}
}
