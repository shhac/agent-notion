// Package official talks to Notion's public REST API
// (https://developers.notion.com). It is a hand-rolled bearer-token client —
// no SDK dependency, so the binary stays zero-dep and static. Responses are
// mapped to the normalized types in package notion by transforms.go.
package official

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DefaultBaseURL is the public Notion API host.
const DefaultBaseURL = "https://api.notion.com"

// APIVersion is sent as Notion-Version on every request. It matches the
// default baked into @notionhq/client v2.3.0 (the TS reference), so both
// binaries speak the same wire format.
const APIVersion = "2022-06-28"

// Client is a bearer-token API client. Zero-value HTTP/BaseURL fall back to
// http.DefaultClient and the real API.
type Client struct {
	HTTP    *http.Client
	BaseURL string
	Token   string
}

// APIError is a decoded Notion error response
// ({object:"error", status, code, message}).
type APIError struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error renders an LLM-useful message: the API's own code and message when
// present, otherwise the HTTP status.
func (e *APIError) Error() string {
	switch {
	case e.Message != "":
		return fmt.Sprintf("notion API error (%s): %s", e.Code, e.Message)
	case e.Code != "":
		return fmt.Sprintf("notion API error (%s): HTTP %d", e.Code, e.Status)
	default:
		return fmt.Sprintf("notion API error: HTTP %d", e.Status)
	}
}

// paginatedRaw is the common list envelope: raw result objects plus the cursor
// contract. next_cursor is nullable, so it is a pointer.
type paginatedRaw struct {
	Results    []map[string]any `json:"results"`
	HasMore    bool             `json:"has_more"`
	NextCursor *string          `json:"next_cursor"`
}

func (p paginatedRaw) cursor() string {
	if p.NextCursor == nil {
		return ""
	}
	return *p.NextCursor
}

// do performs one API call: it attaches Bearer auth and the version header,
// marshals body as JSON when non-nil, decodes the response into out when
// non-nil, and turns any 4xx/5xx into an *APIError.
func (c Client) do(ctx context.Context, method, path string, body, out any) error {
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Notion-Version", APIVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return parseAPIError(resp.StatusCode, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("could not parse %s response: %w", path, err)
		}
	}
	return nil
}

// parseAPIError decodes the Notion error body, falling back to the HTTP status
// when the body is not the expected shape.
func parseAPIError(status int, raw []byte) error {
	e := &APIError{Status: status}
	var body APIError
	if json.Unmarshal(raw, &body) == nil {
		e.Code, e.Message = body.Code, body.Message
		if body.Status != 0 {
			e.Status = body.Status
		}
	}
	return e
}

// Bot is the integration identity returned by users/me.
type Bot struct {
	ID            string
	Name          string
	WorkspaceName string
}

// Me fetches the bot user behind the token — the cheapest way to check a
// pasted integration token actually works.
func (c Client) Me(ctx context.Context) (*Bot, error) {
	var body struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Bot  struct {
			WorkspaceName string `json:"workspace_name"`
		} `json:"bot"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/users/me", nil, &body); err != nil {
		return nil, err
	}
	return &Bot{ID: body.ID, Name: body.Name, WorkspaceName: body.Bot.WorkspaceName}, nil
}
