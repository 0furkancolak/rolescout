// Package llmtest is a fake OpenAI-compatible chat endpoint for tests, so no
// test ever needs a real model.
package llmtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Scripted is one reply of the fake server.
type Scripted struct {
	// Content is the assistant message (usually a JSON document).
	Content string
	// Delay is waited before answering (or until the request is cancelled).
	Delay time.Duration
	// Status is the HTTP status; 0 means 200.
	Status int
	// Raw replaces the whole response body, to test malformed envelopes.
	Raw string
	// PromptTokens and CompletionTokens fill the usage block.
	PromptTokens, CompletionTokens int
}

// Request is a recorded call.
type Request struct {
	Path   string
	Header http.Header
	Body   []byte
}

// Server records requests and answers from a script. When the script runs out
// the last reply repeats.
type Server struct {
	*httptest.Server
	mu       sync.Mutex
	script   []Scripted
	requests []Request
	models   []string
	inFlight int
	maxSeen  int
}

// New starts a server that is closed when the test ends. Its models list holds
// the given names.
func New(t testing.TB, models ...string) *Server {
	t.Helper()
	s := &Server{models: models}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.Server.Close)
	return s
}

// BaseURL is what LLM_BASE_URL should be (it ends in /v1).
func (s *Server) BaseURL() string { return s.URL + "/v1" }

// Script replaces the queued replies.
func (s *Server) Script(replies ...Scripted) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.script = replies
}

// Requests returns the calls received so far.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// Chats counts chat completion calls.
func (s *Server) Chats() int {
	n := 0
	for _, r := range s.Requests() {
		if r.Path == "/v1/chat/completions" {
			n++
		}
	}
	return n
}

// MaxConcurrent is the most chat requests that were in flight at once.
func (s *Server) MaxConcurrent() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxSeen
}

func (s *Server) next() Scripted {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.script) == 0 {
		return Scripted{Content: "{}"}
	}
	r := s.script[0]
	if len(s.script) > 1 {
		s.script = s.script[1:]
	}
	return r
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.requests = append(s.requests, Request{Path: r.URL.Path, Header: r.Header.Clone(), Body: body})
	s.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
		type model struct {
			ID string `json:"id"`
		}
		list := make([]model, 0, len(s.models))
		for _, m := range s.models {
			list = append(list, model{ID: m})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": list})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/chat/completions":
		s.mu.Lock()
		s.inFlight++
		if s.inFlight > s.maxSeen {
			s.maxSeen = s.inFlight
		}
		s.mu.Unlock()
		defer func() {
			s.mu.Lock()
			s.inFlight--
			s.mu.Unlock()
		}()
		reply := s.next()
		if reply.Delay > 0 {
			select {
			case <-time.After(reply.Delay):
			case <-r.Context().Done():
				return
			}
		}
		if reply.Status != 0 && reply.Status != http.StatusOK {
			w.WriteHeader(reply.Status)
			_, _ = w.Write([]byte(`{"error":{"message":"scripted failure"}}`))
			return
		}
		if reply.Raw != "" {
			_, _ = w.Write([]byte(reply.Raw))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": reply.Content}, "finish_reason": "stop"}},
			"usage":   map[string]int{"prompt_tokens": reply.PromptTokens, "completion_tokens": reply.CompletionTokens},
		})
	default:
		http.NotFound(w, r)
	}
}
