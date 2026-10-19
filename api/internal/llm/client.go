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
}

// NewClient builds a client. The timeout applies to each request.
func NewClient(o Options) *Client {
	if o.Timeout <= 0 {
		o.Timeout = 60 * time.Second
	}
	return &Client{
		base:   strings.TrimRight(o.BaseURL, "/"),
		model:  o.Model,
		key:    o.APIKey,
		budget: o.Timeout,
		http:   &http.Client{Timeout: o.Timeout},
	}
}

// Model returns the configured model name.
func (c *Client) Model() string { return c.model }

type chatBody struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	ResponseFormat *respFormat   `json:"response_format,omitempty"`
	Temperature    float64       `json:"temperature"`
	MaxTokens      int           `json:"max_tokens,omitempty"`
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
	body := chatBody{
		Model:       model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Messages:    []chatMessage{{Role: "system", Content: req.System}, {Role: "user", Content: req.User}},
	}
	if len(req.Schema) > 0 {
		body.ResponseFormat = &respFormat{Type: "json_schema", JSONSchema: jsonSchema{Name: "job_enrichment", Strict: true, Schema: req.Schema}}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Response{}, fmt.Errorf("%w: encode request", ErrBadResponse)
	}
	start := time.Now()
	data, err := c.do(ctx, http.MethodPost, "/chat/completions", raw)
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
	raw, err := json.Marshal(chatBody{
		Model: model, Temperature: req.Temperature, MaxTokens: req.MaxTokens, Stream: true,
		Messages: []chatMessage{{Role: "system", Content: req.System}, {Role: "user", Content: req.User}},
	})
	if err != nil {
		return Response{}, fmt.Errorf("%w: encode request", ErrBadResponse)
	}
	start := time.Now()
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return Response{}, fmt.Errorf("%w: invalid base URL", ErrBadResponse)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "text/event-stream")
	if c.key != "" {
		hreq.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(hreq)
	if err != nil {
		return Response{}, classify(ctx, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode >= 500, resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode == http.StatusUnauthorized,
		resp.StatusCode == http.StatusForbidden, resp.StatusCode == http.StatusNotFound, resp.StatusCode == http.StatusRequestTimeout:
		return Response{}, fmt.Errorf("%w: HTTP %d", ErrUnavailable, resp.StatusCode)
	case resp.StatusCode >= 400:
		return Response{}, fmt.Errorf("%w: HTTP %d", ErrBadResponse, resp.StatusCode)
	}
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
	switch {
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("%w: HTTP %d", ErrUnavailable, resp.StatusCode)
	case resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode == http.StatusUnauthorized,
		resp.StatusCode == http.StatusForbidden, resp.StatusCode == http.StatusNotFound, resp.StatusCode == http.StatusRequestTimeout:
		// A missing model or bad key is a server-side problem, not this job's.
		return nil, fmt.Errorf("%w: HTTP %d", ErrUnavailable, resp.StatusCode)
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("%w: HTTP %d", ErrBadResponse, resp.StatusCode)
	}
	return data, nil
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
