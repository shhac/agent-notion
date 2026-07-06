// Package mocknotion is a fixture-driven fake of Notion's APIs for tests. It
// is not a Notion clone: it answers requests from queued fixture responses
// and records calls for assertions. It serves both API surfaces:
//
//   - v3 internal API: POST /api/v3/<endpoint>, keyed by endpoint name
//     (e.g. "loadPageChunk")
//   - official REST API: keyed by "<METHOD> <path>" (e.g. "GET /v1/users/me")
package mocknotion

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Call records one request for test assertions.
type Call struct {
	// Key is the routing key the request matched: a v3 endpoint name or
	// "<METHOD> <path>" for official-API requests.
	Key    string
	Body   json.RawMessage
	Header http.Header
}

// Response is one canned reply. Zero-value fields default to status 200 and
// an empty JSON object body.
type Response struct {
	Status int
	Header map[string]string
	Body   any
	// RawBody, when set, is written verbatim (e.g. NDJSON streams). Wins
	// over Body.
	RawBody []byte
}

// conditional is a body-matched response queue, checked before the plain
// queue so fixtures can model per-entity answers ("syncRecordValues for the
// discussion table returns X") instead of relying on call order.
type conditional struct {
	match func(body json.RawMessage) bool
	queue []Response
}

// Server implements http.Handler. Responses queue per key: each call pops
// the next queued response, and the final one is sticky (repeats forever) so
// single-fixture setups behave like a steady-state API. Body-conditional
// handlers (HandleWhen) take precedence over the plain queue.
type Server struct {
	mu           sync.Mutex
	queues       map[string][]Response
	conditionals map[string][]*conditional
	calls        []Call

	// ExpectTokenV2, when set, rejects v3 calls whose token_v2 cookie
	// differs, with a 401 — exercises re-auth paths.
	ExpectTokenV2 string
	// ExpectBearer, when set, rejects official-API calls whose Bearer token
	// differs, with the official 401 error body — exercises token refresh.
	ExpectBearer string
}

// New returns an empty fixture server.
func New() *Server {
	return &Server{
		queues:       map[string][]Response{},
		conditionals: map[string][]*conditional{},
	}
}

// Handle queues responses for a key. Multiple calls append.
func (s *Server) Handle(key string, responses ...Response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queues[key] = append(s.queues[key], responses...)
}

// HandleWhen queues responses served only when match(body) is true, checked
// (in registration order) before the plain queue. The last response in the
// queue is sticky, like Handle.
func (s *Server) HandleWhen(key string, match func(body json.RawMessage) bool, responses ...Response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conditionals[key] = append(s.conditionals[key], &conditional{match: match, queue: responses})
}

// HandleBody queues a single 200 response with the given JSON body.
func (s *Server) HandleBody(key string, body any) {
	s.Handle(key, Response{Body: body})
}

// Calls returns all recorded calls in order.
func (s *Server) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Call(nil), s.calls...)
}

// CallsFor returns recorded calls for one key.
func (s *Server) CallsFor(key string) []Call {
	var out []Call
	for _, c := range s.Calls() {
		if c.Key == key {
			out = append(out, c)
		}
	}
	return out
}

// Reset clears queues, conditional handlers, and recorded calls.
func (s *Server) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queues = map[string][]Response{}
	s.conditionals = map[string][]*conditional{}
	s.calls = nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key, isV3 := routingKey(r)
	body, _ := io.ReadAll(r.Body)

	s.mu.Lock()
	s.calls = append(s.calls, Call{Key: key, Body: body, Header: r.Header.Clone()})
	expectTokenV2, expectBearer := s.ExpectTokenV2, s.ExpectBearer
	s.mu.Unlock()

	// Reject before consuming a fixture so a re-authed retry still gets it.
	if isV3 && expectTokenV2 != "" && cookieTokenV2(r) != expectTokenV2 {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"errorId": "mock", "name": "UnauthorizedError"})
		return
	}
	if !isV3 && expectBearer != "" && bearerToken(r) != expectBearer {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"object": "error", "status": 401, "code": "unauthorized",
			"message": "API token is invalid.",
		})
		return
	}

	s.mu.Lock()
	resp, ok := s.popResponse(key, body)
	s.mu.Unlock()

	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"object": "error", "status": 404, "code": "mock_unhandled",
			"message": "mocknotion: no fixture queued for " + key,
		})
		return
	}

	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	for k, v := range resp.Header {
		w.Header().Set(k, v)
	}
	if resp.RawBody != nil {
		w.WriteHeader(status)
		_, _ = w.Write(resp.RawBody)
		return
	}
	body2 := resp.Body
	if body2 == nil {
		body2 = map[string]any{}
	}
	writeJSON(w, status, body2)
}

// routingKey derives the fixture key: v3 endpoints route by name, everything
// else by "<METHOD> <path>".
func routingKey(r *http.Request) (key string, isV3 bool) {
	if endpoint, ok := strings.CutPrefix(r.URL.Path, "/api/v3/"); ok && endpoint != "" {
		return endpoint, true
	}
	return r.Method + " " + r.URL.Path, false
}

// popResponse pops the next queued response — from the first matching
// conditional handler, else the plain queue — keeping the last one sticky.
// Caller holds s.mu.
func (s *Server) popResponse(key string, body json.RawMessage) (Response, bool) {
	for _, cond := range s.conditionals[key] {
		if len(cond.queue) == 0 || !cond.match(body) {
			continue
		}
		resp := cond.queue[0]
		if len(cond.queue) > 1 {
			cond.queue = cond.queue[1:]
		}
		return resp, true
	}
	queue := s.queues[key]
	if len(queue) == 0 {
		return Response{}, false
	}
	resp := queue[0]
	if len(queue) > 1 {
		s.queues[key] = queue[1:]
	}
	return resp, true
}

func cookieTokenV2(r *http.Request) string {
	c, err := r.Cookie("token_v2")
	if err != nil {
		return ""
	}
	return c.Value
}

func bearerToken(r *http.Request) string {
	bearer, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return bearer
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
