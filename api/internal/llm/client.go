// Package llm talks to a local OpenAI-compatible chat endpoint (Ollama,
// llama.cpp server, LM Studio) and turns its replies into validated job data.
// Nothing here is on a request path: callers are background loops.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

var (
	// ErrUnavailable means the model server could not be reached or failed (5xx).
	ErrUnavailable = errors.New("llm: server unavailable")
	// ErrTimeout means a request took longer than the configured timeout.
	ErrTimeout = errors.New("llm: request timed out")
	// ErrBadResponse means the server answered with something that is not a
	// usable chat completion.
	ErrBadResponse = errors.New("llm: bad response")
)

// Options configures a Client.
type Options struct {
	BaseURL string
	Model   string
	APIKey  string
	Timeout time.Duration
}

// Request is one chat completion. Schema, when set, constrains the output.
type Request struct {
	// Model overrides the client's model (used for the fallback model).
	Model       string
	System      string
	User        string
	Schema      json.RawMessage
	MaxTokens   int
	Temperature float64
}

// Response is the assistant message with usage and timing.
type Response struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
	Latency          time.Duration
}

// Completer is what the enricher depends on; tests use the fake server.
type Completer interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

// Streamer is a Completer that can hand over the reply piece by piece.
type Streamer interface {
	Completer
	Stream(ctx context.Context, req Request, onDelta func(string)) (Response, error)
}

// Health is the result of a cheap reachability check.
type Health struct {
	Reachable    bool
	ModelPresent bool
	Latency      time.Duration
}

// Client is an OpenAI-compatible chat client.
type Client struct {
	base   string
	model  string
	key    string
	http   *http.Client
	budget time.Duration

	// Newer OpenAI models reject max_tokens and a custom temperature. The
	// flags start from the model name and flip once if the server says so.
	newTokens atomic.Bool
	noTemp    atomic.Bool
}

// NewClient builds a client. The timeout applies to each request.
func NewClient(o Options) *Client {
	if o.Timeout <= 0 {
		o.Timeout = 60 * time.Second
	}
	c := &Client{
		base:   strings.TrimRight(o.BaseURL, "/"),
		model:  o.Model,
		key:    o.APIKey,
		budget: o.Timeout,
		http:   &http.Client{Timeout: o.Timeout},
	}
	if strictParams(o.Model) {
		c.newTokens.Store(true)
		c.noTemp.Store(true)
	}
	return c
}

// strictParams reports models of the OpenAI generation that takes
// max_completion_tokens and only the default temperature: gpt-5 and later, and
// the o-series. Anything else is learned from the server's first complaint.
func strictParams(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if rest, ok := strings.CutPrefix(m, "gpt-"); ok && rest != "" {
		return rest[0] >= '5' && rest[0] <= '9'
	}
	return len(m) >= 2 && m[0] == 'o' && m[1] >= '1' && m[1] <= '9'
}

// reasoningHeadroom is added to a completion cap for strict models, whose
// hidden reasoning counts against it.
const reasoningHeadroom = 2048

// adapt flips the parameter style when a 400 names the one it refused. It
// returns false when nothing changed, so a retry would fail the same way.
func (c *Client) adapt(err error) bool {
	var he *httpError
	if !errors.As(err, &he) || he.status != http.StatusBadRequest {
		return false
	}
	msg := strings.ToLower(he.body)
	changed := false
	switch {
	case strings.Contains(msg, "max_completion_tokens") && !c.newTokens.Load():
		c.newTokens.Store(true)
		changed = true
	case strings.Contains(msg, "max_tokens") && c.newTokens.Load() && !strings.Contains(msg, "max_completion_tokens"):
		c.newTokens.Store(false)
		changed = true
	}
	if strings.Contains(msg, "temperature") && !c.noTemp.Load() {
		c.noTemp.Store(true)
		changed = true
	}
	return changed
}

// httpError is a 4xx the server explained. Error() omits the body.
type httpError struct {
	status int
	body   string
}

func (e *httpError) Error() string { return fmt.Sprintf("%v: HTTP %d", ErrBadResponse, e.status) }
func (e *httpError) Unwrap() error { return ErrBadResponse }

func (c *Client) chatBody(req Request, model string, stream bool) chatBody {
	b := chatBody{
		Model:    model,
		Stream:   stream,
		Messages: []chatMessage{{Role: "system", Content: req.System}, {Role: "user", Content: req.User}},
	}
	if !c.noTemp.Load() {
		t := req.Temperature
		b.Temperature = &t
	}
	switch {
	case req.MaxTokens <= 0:
	case c.newTokens.Load():
		b.MaxCompletion = req.MaxTokens
		if req.MaxTokens > 1 {
			b.MaxCompletion += reasoningHeadroom
		}
	default:
		b.MaxTokens = req.MaxTokens
	}
	return b
}

// Model returns the configured model name.
func (c *Client) Model() string { return c.model }

type chatBody struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	ResponseFormat *respFormat   `json:"response_format,omitempty"`
	Temperature    *float64      `json:"temperature,omitempty"`
	MaxTokens      int           `json:"max_tokens,omitempty"`
	MaxCompletion  int           `json:"max_completion_tokens,omitempty"`
	Stream         bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type respFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

type jsonSchema struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

// Complete sends one chat completion. Error messages never contain the request
// text, because descriptions can be long and private.
func (c *Client) Complete(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	send := func() ([]byte, error) {
		body := c.chatBody(req, model, false)
		if len(req.Schema) > 0 {
			body.ResponseFormat = &respFormat{Type: "json_schema", JSONSchema: jsonSchema{Name: "job_enrichment", Strict: true, Schema: req.Schema}}
		}
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("%w: encode request", ErrBadResponse)
		}
		return c.do(ctx, http.MethodPost, "/chat/completions", raw)
	}
	start := time.Now()
	data, err := send()
	for i := 0; i < 2 && err != nil && c.adapt(err); i++ {
		data, err = send()
	}
	if err != nil {
		return Response{}, err
	}
	var env struct {
		Choices []struct {
			Message struct {
				Content *string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &env); err != nil || len(env.Choices) == 0 || env.Choices[0].Message.Content == nil {
		return Response{}, fmt.Errorf("%w: unexpected response shape", ErrBadResponse)
	}
	return Response{
		Content:          *env.Choices[0].Message.Content,
		PromptTokens:     env.Usage.PromptTokens,
		CompletionTokens: env.Usage.CompletionTokens,
		Latency:          time.Since(start),
	}, nil
}

// Stream is Complete with "stream": true: onDelta gets each piece of text as
// the model writes it, and the joined reply is returned at the end.
func (c *Client) Stream(ctx context.Context, req Request, onDelta func(string)) (Response, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	open := func() (*http.Response, error) {
		raw, err := json.Marshal(c.chatBody(req, model, true))
		if err != nil {
			return nil, fmt.Errorf("%w: encode request", ErrBadResponse)
		}
		hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/chat/completions", bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("%w: invalid base URL", ErrBadResponse)
		}
		hreq.Header.Set("Content-Type", "application/json")
		hreq.Header.Set("Accept", "text/event-stream")
		if c.key != "" {
			hreq.Header.Set("Authorization", "Bearer "+c.key)
		}
		resp, err := c.http.Do(hreq)
		if err != nil {
			return nil, classify(ctx, err)
		}
		if err := statusError(resp); err != nil {
			resp.Body.Close()
			return nil, err
		}
		return resp, nil
	}
	start := time.Now()
	resp, err := open()
	for i := 0; i < 2 && err != nil && c.adapt(err); i++ {
		resp, err = open()
	}
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	var all strings.Builder
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line, ok := strings.CutPrefix(sc.Text(), "data:")
		if !ok {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "[DONE]" {
			break
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(line), &ev) != nil || len(ev.Choices) == 0 || ev.Choices[0].Delta.Content == "" {
			continue
		}
		piece := ev.Choices[0].Delta.Content
		all.WriteString(piece)
		if onDelta != nil {
			onDelta(piece)
		}
	}
	if err := sc.Err(); err != nil {
		return Response{}, classify(ctx, err)
	}
	if all.Len() == 0 {
		return Response{}, fmt.Errorf("%w: empty stream", ErrBadResponse)
	}
	return Response{Content: all.String(), Latency: time.Since(start)}, nil
}

// Health lists the server's models and reports whether the configured one is
// there. It does not run the model.
func (c *Client) Health(ctx context.Context) (Health, error) {
	start := time.Now()
	data, err := c.do(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return Health{}, err
	}
	h := Health{Reachable: true, Latency: time.Since(start)}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &list) != nil {
		return h, nil
	}
	for _, m := range list.Data {
		if sameModel(m.ID, c.model) {
			h.ModelPresent = true
			break
		}
	}
	return h, nil
}

// Probe runs a one-token completion to prove the model loads and answers.
func (c *Client) Probe(ctx context.Context) (Response, error) {
	return c.Complete(ctx, Request{System: "Reply with one word.", User: "ping", MaxTokens: 1})
}

// sameModel compares names, treating a missing tag as ":latest".
func sameModel(listed, want string) bool {
	norm := func(s string) string {
		s = strings.TrimSpace(s)
		if !strings.Contains(s, ":") {
			s += ":latest"
		}
		return s
	}
	return norm(listed) == norm(want)
}

func (c *Client) do(ctx context.Context, method, path string, payload []byte) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid base URL", ErrBadResponse)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, classify(ctx, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, classify(ctx, err)
	}
	if err := statusError(&http.Response{StatusCode: resp.StatusCode, Body: io.NopCloser(bytes.NewReader(data))}); err != nil {
		return nil, err
	}
	return data, nil
}

// statusError maps a non-2xx reply to the package's errors. A 400 keeps the
// server's explanation (not shown in Error()) so the client can adapt.
func statusError(resp *http.Response) error {
	switch {
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: HTTP %d", ErrUnavailable, resp.StatusCode)
	case resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode == http.StatusUnauthorized,
		resp.StatusCode == http.StatusForbidden, resp.StatusCode == http.StatusNotFound, resp.StatusCode == http.StatusRequestTimeout:
		// A missing model or bad key is a server-side problem, not this job's.
		return fmt.Errorf("%w: HTTP %d", ErrUnavailable, resp.StatusCode)
	case resp.StatusCode >= 400:
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return &httpError{status: resp.StatusCode, body: string(b)}
	}
	return nil
}

func classify(ctx context.Context, err error) error {
	// A cancelled caller is not a server problem; hand the cancellation back.
	if ctx.Err() != nil && errors.Is(ctx.Err(), context.Canceled) {
		return context.Canceled
	}
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
		return ErrTimeout
	}
	return fmt.Errorf("%w: %s", ErrUnavailable, scrub(err))
}

// scrub keeps only the operation and the reason, dropping URLs with query
// strings or anything else the standard library might echo.
func scrub(err error) string {
	var op *net.OpError
	if errors.As(err, &op) {
		return op.Op + ": " + op.Err.Error()
	}
	return "request failed"
}
