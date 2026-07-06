// Package official talks to Notion's public REST API
// (https://developers.notion.com). Currently only the users/me probe used to
// validate integration tokens; the full client lands in Phase 4.
package official

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DefaultBaseURL is the public Notion API host.
const DefaultBaseURL = "https://api.notion.com"

// APIVersion is sent as Notion-Version on every request.
const APIVersion = "2022-06-28"

// Client is a bearer-token API client. Zero-value HTTP/BaseURL fall back to
// http.DefaultClient and the real API.
type Client struct {
	HTTP    *http.Client
	BaseURL string
	Token   string
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
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/users/me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Notion-Version", APIVersion)

	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("users/me failed: HTTP %d", resp.StatusCode)
	}

	var body struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Bot  struct {
			WorkspaceName string `json:"workspace_name"`
		} `json:"bot"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("could not parse users/me response: %w", err)
	}
	return &Bot{ID: body.ID, Name: body.Name, WorkspaceName: body.Bot.WorkspaceName}, nil
}
