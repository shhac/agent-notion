package oauth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// DefaultTokenURL is Notion's OAuth token endpoint.
const DefaultTokenURL = "https://api.notion.com/v1/oauth/token"

// DefaultAuthorizeURL is Notion's OAuth authorization page.
const DefaultAuthorizeURL = "https://api.notion.com/v1/oauth/authorize"

// Token is Notion's OAuth token response (both grant types).
type Token struct {
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	BotID         string `json:"bot_id"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	WorkspaceIcon string `json:"workspace_icon"`
	Owner         *Owner `json:"owner"`
}

// Owner identifies the authorizing user in a token response.
type Owner struct {
	Type string `json:"type"`
	User struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Person struct {
			Email string `json:"email"`
		} `json:"person"`
	} `json:"user"`
}

// TokenClient calls the OAuth token endpoint. The zero value targets the real
// Notion API with http.DefaultClient; tests override both fields.
type TokenClient struct {
	HTTP *http.Client
	URL  string
}

// Exchange trades an authorization code for tokens. redirectURI must match
// the one used in the authorize request.
func (c TokenClient) Exchange(ctx context.Context, clientID, clientSecret, code, redirectURI string) (*Token, error) {
	return c.post(ctx, clientID, clientSecret, map[string]string{
		"grant_type":   "authorization_code",
		"code":         code,
		"redirect_uri": redirectURI,
	}, "exchange authorization code for token")
}

// Refresh trades a refresh token for a fresh token pair.
func (c TokenClient) Refresh(ctx context.Context, clientID, clientSecret, refreshToken string) (*Token, error) {
	return c.post(ctx, clientID, clientSecret, map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	}, "refresh access token")
}

func (c TokenClient) post(ctx context.Context, clientID, clientSecret string, body map[string]string, action string) (*Token, error) {
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	endpoint := c.URL
	if endpoint == "" {
		endpoint = DefaultTokenURL
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	basic := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Set("Authorization", "Basic "+basic)
	req.Header.Set("Content-Type", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &apiErr)
		detail := apiErr.Message
		if detail == "" {
			detail = apiErr.Error
		}
		if detail == "" {
			detail = resp.Status
		}
		return nil, fmt.Errorf("failed to %s: %s", action, detail)
	}

	var tok Token
	if err := json.Unmarshal(raw, &tok); err != nil {
		return nil, fmt.Errorf("failed to %s: could not parse response: %w", action, err)
	}
	return &tok, nil
}

// AuthorizeURL builds the browser URL that starts the OAuth consent flow.
func AuthorizeURL(clientID, redirectURI, state string) string {
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("owner", "user")
	q.Set("state", state)
	return DefaultAuthorizeURL + "?" + q.Encode()
}
