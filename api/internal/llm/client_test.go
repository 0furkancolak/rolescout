package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"example.com/rolescout/api/internal/llm/llmtest"
)

func newTestClient(t *testing.T, srv *llmtest.Server, mutate ...func(*Options)) *Client {
	t.Helper()
	o := Options{BaseURL: srv.BaseURL(), Model: "qwen3:4b-instruct-2507-q4_K_M", Timeout: 2 * time.Second}
	for _, m := range mutate {
		m(&o)
	}
	return NewClient(o)
}

var testSchema = json.RawMessage(`{"type":"object","properties":{"seniority":{"enum":["junior","senior","unknown"]}},"required":["seniority"],"additionalProperties":false}`)

func TestCompleteSendsConstrainedRequest(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: `{"seniority":"senior"}`, PromptTokens: 120, CompletionTokens: 9})
	c := newTestClient(t, srv)

	resp, err := c.Complete(context.Background(), Request{System: "be strict", User: "the job text", Schema: testSchema, MaxTokens: 400})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != `{"seniority":"senior"}` || resp.PromptTokens != 120 || resp.CompletionTokens != 9 || resp.Latency <= 0 {
		t.Fatalf("response = %+v", resp)
	}

	reqs := srv.Requests()
	if len(reqs) != 1 || reqs[0].Path != "/v1/chat/completions" {
		t.Fatalf("requests = %+v", reqs)
	}
	var body struct {
		Model          string `json:"model"`
		Stream         bool   `json:"stream"`
		Temperature    float64
		MaxTokens      int `json:"max_tokens"`
		Messages       []struct{ Role, Content string }
		ResponseFormat struct {
			Type       string `json:"type"`
			JSONSchema struct {
				Name   string          `json:"name"`
				Strict bool            `json:"strict"`
				Schema json.RawMessage `json:"schema"`
			} `json:"json_schema"`
		} `json:"response_format"`
	}
	if err := json.Unmarshal(reqs[0].Body, &body); err != nil {
		t.Fatal(err)
	}
	if body.Model != "qwen3:4b-instruct-2507-q4_K_M" || body.Stream || body.Temperature != 0 || body.MaxTokens != 400 {
		t.Errorf("body = %+v", body)
	}
	if len(body.Messages) != 2 || body.Messages[0].Role != "system" || body.Messages[1].Role != "user" || body.Messages[1].Content != "the job text" {
		t.Errorf("messages = %+v", body.Messages)
	}
	if body.ResponseFormat.Type != "json_schema" || !body.ResponseFormat.JSONSchema.Strict || body.ResponseFormat.JSONSchema.Name != "job_enrichment" ||
		!strings.Contains(string(body.ResponseFormat.JSONSchema.Schema), `"enum"`) {
		t.Errorf("response_format = %+v", body.ResponseFormat)
	}
}

func TestRequestModelOverridesClientModel(t *testing.T) {
	srv := llmtest.New(t)
	c := newTestClient(t, srv)
	if _, err := c.Complete(context.Background(), Request{Model: "qwen3:1.7b-q4_K_M", System: "s", User: "u"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(srv.Requests()[0].Body), `"model":"qwen3:1.7b-q4_K_M"`) {
		t.Fatalf("body = %s", srv.Requests()[0].Body)
	}
}

func TestOmitsSchemaWhenNone(t *testing.T) {
	srv := llmtest.New(t)
	c := newTestClient(t, srv)
	if _, err := c.Complete(context.Background(), Request{System: "s", User: "u"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(srv.Requests()[0].Body), "response_format") {
		t.Fatalf("body = %s", srv.Requests()[0].Body)
	}
}

func TestAPIKeyOnlyWhenSet(t *testing.T) {
	srv := llmtest.New(t)
	if _, err := newTestClient(t, srv).Complete(context.Background(), Request{User: "u"}); err != nil {
		t.Fatal(err)
	}
	if got := srv.Requests()[0].Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q without a key", got)
	}
	c := newTestClient(t, srv, func(o *Options) { o.APIKey = "sekret" })
	if _, err := c.Complete(context.Background(), Request{User: "u"}); err != nil {
		t.Fatal(err)
	}
	if got := srv.Requests()[1].Header.Get("Authorization"); got != "Bearer sekret" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestServerErrorIsUnavailable(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Status: 500})
	_, err := newTestClient(t, srv).Complete(context.Background(), Request{User: "private description text"})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if strings.Contains(err.Error(), "private description text") {
		t.Fatalf("error leaks the request: %v", err)
	}
}

func TestClientErrorIsBadResponse(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Status: 400})
	_, err := newTestClient(t, srv).Complete(context.Background(), Request{User: "u"})
	if !errors.Is(err, ErrBadResponse) {
		t.Fatalf("err = %v, want ErrBadResponse", err)
	}
}

func TestSlowServerTimesOut(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: "{}", Delay: 2 * time.Second})
	c := newTestClient(t, srv, func(o *Options) { o.Timeout = 100 * time.Millisecond })
	start := time.Now()
	_, err := c.Complete(context.Background(), Request{User: "u"})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("took %s", time.Since(start))
	}
}

func TestContextCancelIsReturnedAsIs(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: "{}", Delay: 2 * time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	_, err := newTestClient(t, srv).Complete(ctx, Request{User: "u"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestConnectionRefusedIsUnavailable(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	c := NewClient(Options{BaseURL: "http://" + addr + "/v1", Model: "m", Timeout: time.Second})
	if _, err := c.Complete(context.Background(), Request{User: "u"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestMalformedEnvelopeIsBadResponse(t *testing.T) {
	for name, raw := range map[string]string{
		"not json":   "<html>oops</html>",
		"no choices": `{"choices":[]}`,
		"no content": `{"choices":[{"message":{"role":"assistant"}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := llmtest.New(t)
			srv.Script(llmtest.Scripted{Raw: raw})
			if _, err := newTestClient(t, srv).Complete(context.Background(), Request{User: "u"}); !errors.Is(err, ErrBadResponse) {
				t.Fatalf("err = %v, want ErrBadResponse", err)
			}
		})
	}
}

func TestHealth(t *testing.T) {
	srv := llmtest.New(t, "qwen3:4b-instruct-2507-q4_K_M", "other:latest")
	h, err := newTestClient(t, srv).Health(context.Background())
	if err != nil || !h.Reachable || !h.ModelPresent || h.Latency <= 0 {
		t.Fatalf("health = %+v, %v", h, err)
	}

	missing := newTestClient(t, srv, func(o *Options) { o.Model = "not-pulled:1b" })
	h, err = missing.Health(context.Background())
	if err != nil || !h.Reachable || h.ModelPresent {
		t.Fatalf("missing model: %+v, %v", h, err)
	}

	// A model listed with the implicit :latest tag still counts.
	srv2 := llmtest.New(t, "mymodel:latest")
	h, _ = newTestClient(t, srv2, func(o *Options) { o.Model = "mymodel" }).Health(context.Background())
	if !h.ModelPresent {
		t.Fatalf("implicit latest tag: %+v", h)
	}

	down := NewClient(Options{BaseURL: "http://127.0.0.1:1/v1", Model: "m", Timeout: time.Second})
	h, err = down.Health(context.Background())
	if !errors.Is(err, ErrUnavailable) || h.Reachable {
		t.Fatalf("down: %+v, %v", h, err)
	}
}

func TestProbeSendsOneTokenRequest(t *testing.T) {
	srv := llmtest.New(t)
	if _, err := newTestClient(t, srv).Probe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(srv.Requests()[0].Body), `"max_tokens":1`) {
		t.Fatalf("body = %s", srv.Requests()[0].Body)
	}
}
