// Package client implements a typed client for the self-hosted Mem0 OSS
// REST API (github.com/mem0ai/mem0/server).
//
// Key differences from the Mem0 Platform API:
//   - paths have no /v1/ prefix
//   - auth uses the `X-API-Key` header (m0sk_...) rather than a Bearer token
//   - GET /memories returns {"results":[...]} for unscoped (admin) listing and
//     a bare array when scoped by user/agent/run — both are normalized here
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultTimeout bounds every request unless overridden.
const DefaultTimeout = 30 * time.Second

const (
	// HeaderAPIKey is the auth header Mem0 OSS expects for API keys.
	HeaderAPIKey = "X-API-Key"
	// HeaderAuthorization carries a bearer token for /auth and /api-keys.
	HeaderAuthorization = "Authorization"
)

// Message is a single chat message fed into POST /memories.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Memory is one stored memory as returned by the OSS API.
type Memory struct {
	ID             string         `json:"id"`
	Memory         string         `json:"memory"`
	UserID         *string        `json:"user_id"`
	AgentID        *string        `json:"agent_id"`
	RunID          *string        `json:"run_id"`
	Hash           string         `json:"hash"`
	ExpirationDate *string        `json:"expiration_date"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      *Time          `json:"created_at"`
	UpdatedAt      *Time          `json:"updated_at"`
}

// ScopeOwner pretty-prints which entity owns a memory.
func (m Memory) ScopeOwner() string {
	switch {
	case m.UserID != nil && *m.UserID != "":
		return "user:" + *m.UserID
	case m.AgentID != nil && *m.AgentID != "":
		return "agent:" + *m.AgentID
	case m.RunID != nil && *m.RunID != "":
		return "run:" + *m.RunID
	default:
		return "—"
	}
}

// SearchResult is one hit from POST /search.
type SearchResult struct {
	ID        string         `json:"id"`
	Memory    string         `json:"memory"`
	Score     float64        `json:"score"`
	Hash      string         `json:"hash"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt *Time          `json:"created_at"`
	UpdatedAt *Time          `json:"updated_at"`
}

// AddResult is one memory created by POST /memories.
type AddResult struct {
	ID     string `json:"id"`
	Memory string `json:"memory"`
	Event  string `json:"event"`
}

// Entity is a user/agent/run aggregate from GET /entities.
type Entity struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	TotalMemories int    `json:"total_memories"`
	CreatedAt     *Time  `json:"created_at"`
	UpdatedAt     *Time  `json:"updated_at"`
}

// HistoryEvent is one entry of a memory's change history.
type HistoryEvent struct {
	ID        string `json:"id"`
	Memory    string `json:"memory"`
	Event     string `json:"event"`
	CreatedAt *Time  `json:"created_at"`
	// Raw holds any additional fields the server returned.
	Raw map[string]any `json:"-"`
}

// Currently created API key (POST /api-keys).
type CreatedKey struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	Label     string `json:"label"`
	KeyPrefix string `json:"key_prefix"`
	CreatedAt *Time  `json:"created_at"`
}

// KeyListItem is a row from GET /api-keys.
type KeyListItem struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	KeyPrefix  string `json:"key_prefix"`
	CreatedAt  *Time  `json:"created_at"`
	LastUsedAt *Time  `json:"last_used_at"`
}

// TokenResponse is the result of /auth/login and /auth/register.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

// AddOptions are the optional knobs on POST /memories.
type AddOptions struct {
	UserID         string
	AgentID        string
	RunID          string
	Metadata       map[string]any
	ExpirationDate string
	Infer          *bool
	MemoryType     string
	Prompt         string
}

// SearchOptions are the optional knobs on POST /search.
type SearchOptions struct {
	Filters      map[string]any
	UserID       string
	AgentID      string
	RunID        string
	TopK         int
	Threshold    float64
	Explain      bool
	ShowExpired  bool
	HasThreshold bool
}

// UpdateParams are the optional fields accepted by PUT /memories/{id}.
// Pointer fields distinguish "not provided" from an explicit null.
type UpdateParams struct {
	Text            *string
	Metadata        map[string]any
	SetMetadata     bool
	ExpirationDate  *string
	ClearExpiration bool
}

// Scope identifies a user/agent/run for DeleteAll.
type Scope struct {
	UserID  string
	AgentID string
	RunID   string
}

// Error is a structured error from the Mem0 OSS API.
type Error struct {
	Method string
	Path   string
	Status int
	Detail string
}

func (e *Error) Error() string {
	kind := http.StatusText(e.Status)
	if kind == "" {
		kind = strconv.Itoa(e.Status)
	}
	if e.Detail != "" {
		return fmt.Sprintf("%s %s: %s (HTTP %d %s)", e.Method, e.Path, e.Detail, e.Status, kind)
	}
	return fmt.Sprintf("%s %s: HTTP %d %s", e.Method, e.Path, e.Status, kind)
}

// Client talks to one Mem0 OSS server.
type Client struct {
	BaseURL string // e.g. http://localhost:8888
	APIKey  string // m0sk_...
	Token   string // bearer token for /auth and /api-keys
	HTTP    *http.Client
}

// New builds a client. baseURL and apiKey are used verbatim. Give apiKey ""
// for unauthenticated endpoints (AUTH_DISABLED=true instances).
func New(baseURL, apiKey string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: timeout},
	}
}

// ---- create / search / read ----

// Add stores messages and returns the created memories.
func (c *Client) Add(ctx context.Context, messages []Message, opts AddOptions) ([]AddResult, error) {
	body := map[string]any{"messages": messages}
	setScope(body, opts.UserID, opts.AgentID, opts.RunID)
	if len(opts.Metadata) > 0 {
		body["metadata"] = opts.Metadata
	}
	if opts.ExpirationDate != "" {
		body["expiration_date"] = opts.ExpirationDate
	}
	if opts.Infer != nil {
		body["infer"] = *opts.Infer
	}
	if opts.MemoryType != "" {
		body["memory_type"] = opts.MemoryType
	}
	if opts.Prompt != "" {
		body["prompt"] = opts.Prompt
	}
	var resp struct {
		Results []AddResult `json:"results"`
	}
	if err := c.do(ctx, http.MethodPost, "memories", nil, body, authHeader, &resp); err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// Search runs a natural-language search and returns ranked results.
func (c *Client) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	filters := map[string]any{}
	for k, v := range opts.Filters {
		filters[k] = v
	}
	setScopeMap(filters, opts.UserID, opts.AgentID, opts.RunID)

	body := map[string]any{"query": query}
	if len(filters) > 0 {
		body["filters"] = filters
	}
	if opts.TopK > 0 {
		body["top_k"] = opts.TopK
	}
	if opts.HasThreshold {
		body["threshold"] = opts.Threshold
	}
	if opts.Explain {
		body["explain"] = true
	}
	if opts.ShowExpired {
		body["show_expired"] = true
	}
	var results []SearchResult
	if err := c.do(ctx, http.MethodPost, "search", nil, body, authHeader, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// List returns memories, optionally scoped. Unscoped (admin) listing comes
// back wrapped in {"results":[...]}; scoped listing is a bare array. Both are
// normalized to a slice.
func (c *Client) List(ctx context.Context, s Scope, topK int, showExpired bool) ([]Memory, error) {
	q := url.Values{}
	setScopeQuery(q, s)
	if topK > 0 {
		q.Set("top_k", strconv.Itoa(topK))
	}
	if showExpired {
		q.Set("show_expired", "true")
	}
	raw, err := c.doRaw(ctx, http.MethodGet, "memories", q, nil, authHeader)
	if err != nil {
		return nil, err
	}
	var out []Memory
	if err := json.Unmarshal(raw, &out); err == nil {
		return out, nil
	}
	var wrapped struct {
		Results []Memory `json:"results"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("decode memories response: %w", err)
	}
	return wrapped.Results, nil
}

// Get returns a single memory by ID.
func (c *Client) Get(ctx context.Context, id string) (*Memory, error) {
	var m Memory
	if err := c.do(ctx, http.MethodGet, "memories/"+url.PathEscape(id), nil, nil, authHeader, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// History returns the change history of a memory.
func (c *Client) History(ctx context.Context, id string) ([]HistoryEvent, error) {
	var raw []json.RawMessage
	if err := c.do(ctx, http.MethodGet, "memories/"+url.PathEscape(id)+"/history", nil, nil, authHeader, &raw); err != nil {
		return nil, err
	}
	events := make([]HistoryEvent, 0, len(raw))
	for _, item := range raw {
		ev := HistoryEvent{Raw: map[string]any{}}
		_ = json.Unmarshal(item, &ev)
		_ = json.Unmarshal(item, &ev.Raw)
		events = append(events, ev)
	}
	return events, nil
}

// ---- update / delete ----

// Update changes the text, metadata, or expiration date of a memory.
func (c *Client) Update(ctx context.Context, id string, p UpdateParams) (*Memory, error) {
	body := map[string]any{}
	if p.Text != nil {
		body["text"] = *p.Text
	}
	if p.SetMetadata {
		body["metadata"] = p.Metadata
	}
	switch {
	case p.ClearExpiration:
		body["expiration_date"] = nil
	case p.ExpirationDate != nil:
		body["expiration_date"] = *p.ExpirationDate
	}
	var m Memory
	if err := c.do(ctx, http.MethodPut, "memories/"+url.PathEscape(id), nil, body, authHeader, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Delete removes a single memory by ID.
func (c *Client) Delete(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "memories/"+url.PathEscape(id), nil, nil, authHeader, nil)
}

// DeleteAll removes every memory in the given scope. The scope must be
// non-empty.
func (c *Client) DeleteAll(ctx context.Context, s Scope) error {
	q := url.Values{}
	setScopeQuery(q, s)
	if q.Encode() == "" {
		return fmt.Errorf("delete-all requires at least one of user_id, agent_id, run_id")
	}
	return c.do(ctx, http.MethodDelete, "memories", q, nil, authHeader, nil)
}

// ---- entities ----

// ListEntities aggregates the distinct users/agents/runs and their memory
// counts.
func (c *Client) ListEntities(ctx context.Context) ([]Entity, error) {
	var entities []Entity
	if err := c.do(ctx, http.MethodGet, "entities", nil, nil, authHeader, &entities); err != nil {
		return nil, err
	}
	return entities, nil
}

// DeleteEntity removes all memories for an entity (requires an admin key).
func (c *Client) DeleteEntity(ctx context.Context, entityType, entityID string) error {
	path := "entities/" + url.PathEscape(entityType) + "/" + url.PathEscape(entityID)
	return c.do(ctx, http.MethodDelete, path, nil, nil, authHeader, nil)
}

// ---- server introspection ----

// ServerConfig returns the current (redacted) server configuration.
func (c *Client) ServerConfig(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "configure", nil, nil, authHeader, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Providers lists the bundled LLM and embedder providers.
func (c *Client) Providers(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "configure/providers", nil, nil, authHeader, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---- auth & api keys (used by `mem0 init`) ----

// SetupStatus reports whether the server still needs its first admin.
func (c *Client) SetupStatus(ctx context.Context) (bool, error) {
	var out struct {
		NeedsSetup bool `json:"needsSetup"`
	}
	if err := c.do(ctx, http.MethodGet, "auth/setup-status", nil, nil, noAuth, &out); err != nil {
		return false, err
	}
	return out.NeedsSetup, nil
}

// Register creates the first admin account and returns tokens.
func (c *Client) Register(ctx context.Context, name, email, password string) (*TokenResponse, error) {
	body := map[string]any{"name": name, "email": email, "password": password}
	var out TokenResponse
	if err := c.do(ctx, http.MethodPost, "auth/register", nil, body, noAuth, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Login exchanges email/password for tokens.
func (c *Client) Login(ctx context.Context, email, password string) (*TokenResponse, error) {
	body := map[string]any{"email": email, "password": password}
	var out TokenResponse
	if err := c.do(ctx, http.MethodPost, "auth/login", nil, body, noAuth, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateAPIKey issues a new API key for the current user (bearer auth).
func (c *Client) CreateAPIKey(ctx context.Context, label string) (*CreatedKey, error) {
	body := map[string]any{"label": label}
	var out CreatedKey
	if err := c.do(ctx, http.MethodPost, "api-keys", nil, body, bearerAuth, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAPIKeys lists the current user's active API keys (bearer auth).
func (c *Client) ListAPIKeys(ctx context.Context) ([]KeyListItem, error) {
	var out []KeyListItem
	if err := c.do(ctx, http.MethodGet, "api-keys", nil, nil, bearerAuth, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---- plumbing ----

type authMode int

const (
	authHeader authMode = iota // X-API-Key
	bearerAuth                 // Bearer <token>
	noAuth                     // none
)

// doRaw performs a request and returns the raw response body.
func (c *Client) doRaw(ctx context.Context, method, path string, query url.Values, body any, auth authMode) ([]byte, error) {
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mem0-cli/0.1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch auth {
	case bearerAuth:
		if c.Token != "" {
			req.Header.Set(HeaderAuthorization, "Bearer "+c.Token)
		}
	case authHeader:
		if c.APIKey != "" {
			req.Header.Set(HeaderAPIKey, c.APIKey)
		}
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &Error{
			Method: method,
			Path:   path,
			Status: resp.StatusCode,
			Detail: extractDetail(data),
		}
	}
	return data, nil
}

// do performs a request and decodes the JSON response into out (if non-nil).
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, auth authMode, out any) error {
	data, err := c.doRaw(ctx, method, path, query, body, auth)
	if err != nil {
		return err
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response from %s %s: %w", method, path, err)
	}
	return nil
}

// extractDetail pulls the human-readable error message out of an OSS error
// body, which is usually {"detail": "..."} but may be a plain string.
func extractDetail(data []byte) string {
	var obj struct {
		Detail any `json:"detail"`
	}
	if err := json.Unmarshal(data, &obj); err == nil && obj.Detail != nil {
		switch v := obj.Detail.(type) {
		case string:
			if v != "" {
				return v
			}
		default:
			if b, err := json.Marshal(v); err == nil && string(b) != "null" {
				return string(b)
			}
		}
		return ""
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil && s != "" {
		return s
	}
	return ""
}

// setScope writes the (non-empty) identity keys into a body map.
func setScope(body map[string]any, userID, agentID, runID string) {
	if userID != "" {
		body["user_id"] = userID
	}
	if agentID != "" {
		body["agent_id"] = agentID
	}
	if runID != "" {
		body["run_id"] = runID
	}
}

// setScopeMap is setScope for search filters (identity goes in filters).
func setScopeMap(filters map[string]any, userID, agentID, runID string) {
	if userID != "" {
		filters["user_id"] = userID
	}
	if agentID != "" {
		filters["agent_id"] = agentID
	}
	if runID != "" {
		filters["run_id"] = runID
	}
}

// setScopeQuery writes identity keys onto a query string.
func setScopeQuery(q url.Values, s Scope) {
	if s.UserID != "" {
		q.Set("user_id", s.UserID)
	}
	if s.AgentID != "" {
		q.Set("agent_id", s.AgentID)
	}
	if s.RunID != "" {
		q.Set("run_id", s.RunID)
	}
}
