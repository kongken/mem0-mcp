package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestClient spins up an httptest server that routes by path/method and
// asserts the X-API-Key header.
func newTestClient(t *testing.T, apiKey string, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	return New(srv.URL, apiKey, 5*time.Second), srv
}

// route returns a handler keyed by "METHOD path-prefix".
type route struct{ method, prefix string }

func serve(t *testing.T, r *sync.Map, key string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if got := req.Header.Get(HeaderAPIKey); got != key {
			http.Error(w, `{"detail":"bad key"}`, http.StatusUnauthorized)
			return
		}
		h, _ := r.Load(route{req.Method, req.URL.Path})
		if h == nil {
			http.NotFound(w, req)
			return
		}
		h.(http.HandlerFunc)(w, req)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAdd(t *testing.T) {
	var gotBody map[string]any
	var mu sync.Map
	mu.Store(route{"POST", "/memories"}, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = json.NewDecoder(req.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"results":[{"id":"m1","memory":"likes Go","event":"ADD"}]}`)
	}))
	c := New(serve(t, &mu, "k").URL, "k", time.Second)

	results, err := c.Add(context.Background(), []Message{{Role: "user", Content: "likes Go"}}, AddOptions{UserID: "alice", ExpirationDate: "2026-12-31"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(results) != 1 || results[0].ID != "m1" || results[0].Memory != "likes Go" {
		t.Errorf("results = %+v", results)
	}
	if gotBody["user_id"] != "alice" {
		t.Errorf("body user_id = %v", gotBody["user_id"])
	}
	if gotBody["expiration_date"] != "2026-12-31" {
		t.Errorf("body expiration_date = %v", gotBody["expiration_date"])
	}
	messages := gotBody["messages"].([]any)
	if len(messages) != 1 {
		t.Errorf("messages = %v", messages)
	}
}

func TestAddRequiresScopeRefusedServerSide(t *testing.T) {
	var mu sync.Map
	mu.Store(route{"POST", "/memories"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"detail":"At least one identifier (user_id, agent_id, run_id) is required."}`)
	}))
	c := New(serve(t, &mu, "k").URL, "k", time.Second)

	_, err := c.Add(context.Background(), []Message{{Role: "user", Content: "x"}}, AddOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	ce, ok := err.(*Error)
	if !ok {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if ce.Status != 400 || !strings.Contains(ce.Detail, "identifier") {
		t.Errorf("error = %+v", ce)
	}
}

func TestListBareArray(t *testing.T) {
	var mu sync.Map
	mu.Store(route{"GET", "/memories"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `[{"id":"m1","memory":"a","created_at":"2026-01-01T00:00:00Z"},{"id":"m2","memory":"b"}]`)
	}))
	c := New(serve(t, &mu, "k").URL, "k", time.Second)

	ms, err := c.List(context.Background(), Scope{UserID: "alice"}, 0, false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ms) != 2 || ms[0].ID != "m1" {
		t.Errorf("memories = %+v", ms)
	}
	if ms[0].CreatedAt == nil || ms[0].CreatedAt.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("created_at parse failed: %+v", ms[0].CreatedAt)
	}
}

func TestListWrappedResults(t *testing.T) {
	var mu sync.Map
	mu.Store(route{"GET", "/memories"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"results":[{"id":"m1","memory":"a"},{"id":"m2","memory":"b"}]}`)
	}))
	c := New(serve(t, &mu, "k").URL, "k", time.Second)

	ms, err := c.List(context.Background(), Scope{}, 0, false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ms) != 2 {
		t.Errorf("expected 2 memories, got %d: %+v", len(ms), ms)
	}
}

func TestDeleteAllRequiresScope(t *testing.T) {
	c := New("http://x", "k", time.Second)
	if err := c.DeleteAll(context.Background(), Scope{}); err == nil {
		t.Fatal("expected error for empty scope")
	}
}

func TestDeleteAllSendsScopeQuery(t *testing.T) {
	var mu sync.Map
	mu.Store(route{"DELETE", "/memories"}, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if q := req.URL.Query().Get("user_id"); q != "alice" {
			t.Errorf("user_id query = %q", q)
		}
		fmt.Fprint(w, `{"message":"All relevant memories deleted"}`)
	}))
	c := New(serve(t, &mu, "k").URL, "k", time.Second)
	if err := c.DeleteAll(context.Background(), Scope{UserID: "alice"}); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
}

func TestSearchBuildsBody(t *testing.T) {
	var got map[string]any
	var mu sync.Map
	mu.Store(route{"POST", "/search"}, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = json.NewDecoder(req.Body).Decode(&got)
		fmt.Fprint(w, `[{"id":"m1","memory":"prefers vim","score":0.9}]`)
	}))
	c := New(serve(t, &mu, "k").URL, "k", time.Second)

	res, err := c.Search(context.Background(), "editor?", SearchOptions{UserID: "alice", TopK: 5, Threshold: 0.5, HasThreshold: true})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 1 || res[0].Score != 0.9 {
		t.Errorf("results = %+v", res)
	}
	filters := got["filters"].(map[string]any)
	if filters["user_id"] != "alice" {
		t.Errorf("filters = %v", filters)
	}
	if got["top_k"].(float64) != 5 {
		t.Errorf("top_k = %v", got["top_k"])
	}
	if got["threshold"].(float64) != 0.5 {
		t.Errorf("threshold = %v", got["threshold"])
	}
}

func TestUpdatePreservesUnsetFields(t *testing.T) {
	var got map[string]any
	var mu sync.Map
	mu.Store(route{"PUT", "/memories/m1"}, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = json.NewDecoder(req.Body).Decode(&got)
		fmt.Fprint(w, `{"id":"m1","memory":"new text"}`)
	}))
	c := New(serve(t, &mu, "k").URL, "k", time.Second)

	text := "new text"
	if _, err := c.Update(context.Background(), "m1", UpdateParams{Text: &text}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got["text"] != "new text" {
		t.Errorf("body = %v", got)
	}
	if _, present := got["metadata"]; present {
		t.Errorf("metadata should be absent when not set: %v", got)
	}
}

func TestErrorsCarryStatusAndDetail(t *testing.T) {
	var mu sync.Map
	mu.Store(route{"GET", "/memories/ghost"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"detail":"Memory not found"}`)
	}))
	c := New(serve(t, &mu, "k").URL, "k", time.Second)

	_, err := c.Get(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected 404 error")
	}
	ce, ok := err.(*Error)
	if !ok {
		t.Fatalf("type = %T, want *Error", err)
	}
	if ce.Status != 404 || ce.Detail != "Memory not found" {
		t.Errorf("error = %+v", ce)
	}
}

func TestAuthHeaderNotSentForNoAuth(t *testing.T) {
	var hdrKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hdrKey = req.Header.Get(HeaderAPIKey)
		fmt.Fprint(w, `{"access_token":"t","refresh_token":"r","token_type":"bearer"}`)
	}))
	defer srv.Close()
	c := New(srv.URL, "k", time.Second)
	if _, err := c.Login(context.Background(), "a@b.c", "pw"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if hdrKey != "" {
		t.Errorf("X-API-Key sent on /auth/login: %q", hdrKey)
	}
}

func TestTimeParsingVariants(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`"2026-01-02T03:04:05Z"`, "2026-01-02 03:04:05 +0000 UTC"},
		{`"2026-01-02 03:04:05"`, "2026-01-02 03:04:05 +0000 UTC"},
		{`"2026-01-02"`, "2026-01-02 00:00:00 +0000 UTC"},
		{`null`, ""},
	} {
		var tme Time
		if err := json.Unmarshal([]byte(tc.in), &tme); err != nil {
			t.Fatalf("unmarshal %s: %v", tc.in, err)
		}
		if tc.want == "" {
			if !tme.IsZero() {
				t.Errorf("unmarshal %s = %v, want zero", tc.in, tme.Time)
			}
			continue
		}
		if got := tme.UTC().String(); got != tc.want {
			t.Errorf("unmarshal %s = %q, want %q", tc.in, got, tc.want)
		}
	}
}
