// Package v3 talks to Notion's unofficial (token_v2) API. This file covers the
// getSpaces call used to validate a desktop/browser token and derive session
// identity, ported from the TS validateDesktopToken/parseGetSpacesSession.
package v3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SessionInfo is the identity derived from a getSpaces response.
type SessionInfo struct {
	UserID      string `json:"user_id"`
	UserEmail   string `json:"user_email"`
	UserName    string `json:"user_name"`
	SpaceID     string `json:"space_id"`
	SpaceName   string `json:"space_name"`
	SpaceViewID string `json:"space_view_id,omitempty"`
}

// ValidateDesktopToken calls getSpaces with the token and derives session
// info. client and baseURL may be zero for the real API; tests inject both.
func ValidateDesktopToken(ctx context.Context, client *http.Client, baseURL, token string) (SessionInfo, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if baseURL == "" {
		baseURL = "https://www.notion.so/api/v3"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/getSpaces", bytes.NewReader([]byte("{}")))
	if err != nil {
		return SessionInfo{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "token_v2="+token)

	resp, err := client.Do(req)
	if err != nil {
		return SessionInfo{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return SessionInfo{}, fmt.Errorf("token validation failed: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SessionInfo{}, err
	}

	var data map[string]map[string]map[string]json.RawMessage
	if err := json.Unmarshal(body, &data); err != nil {
		return SessionInfo{}, fmt.Errorf("could not parse getSpaces response: %w", err)
	}
	return ParseGetSpacesSession(data)
}

// ParseGetSpacesSession extracts session info from a getSpaces response,
// shaped userId → table → recordId → record. Only the first user entry is
// considered. Records may be role-wrapped ({value:{value,role}}) or shallow.
func ParseGetSpacesSession(data map[string]map[string]map[string]json.RawMessage) (SessionInfo, error) {
	for userID, tables := range data {
		user := findUserInfo(tables["notion_user"])
		spaceID, spaceName := pickPreferredSpace(tables["space"])
		return SessionInfo{
			UserID:      userID,
			UserEmail:   user["email"],
			UserName:    user["name"],
			SpaceID:     spaceID,
			SpaceName:   spaceName,
			SpaceViewID: findSpaceViewID(tables["space_view"], spaceID),
		}, nil
	}
	return SessionInfo{}, fmt.Errorf("could not extract user info from token; it may be expired")
}

// entityOf returns the actual entity from a record's `value` field, unwrapping
// the role-wrapped {value: entity, role} shape when present.
func entityOf(record json.RawMessage) map[string]any {
	var wrapper struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(record, &wrapper); err != nil || len(wrapper.Value) == 0 {
		return nil
	}
	var entity map[string]any
	if err := json.Unmarshal(wrapper.Value, &entity); err != nil {
		return nil
	}
	// Role-wrapped: the entity is nested one level deeper under "value".
	if nested, ok := entity["value"].(map[string]any); ok {
		return nested
	}
	return entity
}

func findUserInfo(table map[string]json.RawMessage) map[string]string {
	for _, rec := range table {
		e := entityOf(rec)
		if e == nil {
			continue
		}
		if _, ok := e["email"]; ok {
			return map[string]string{
				"email": asString(e["email"]),
				"name":  asString(e["name"]),
			}
		}
	}
	return map[string]string{}
}

// pickPreferredSpace returns a space id/name, preferring team/enterprise plans.
func pickPreferredSpace(table map[string]json.RawMessage) (id, name string) {
	var firstID, firstName string
	found := false
	for _, rec := range table {
		e := entityOf(rec)
		if e == nil {
			continue
		}
		if _, ok := e["name"]; !ok {
			continue
		}
		if !found {
			firstID, firstName = asString(e["id"]), asString(e["name"])
			found = true
		}
		if plan := asString(e["plan_type"]); plan == "team" || plan == "enterprise" {
			return asString(e["id"]), asString(e["name"])
		}
	}
	return firstID, firstName
}

func findSpaceViewID(table map[string]json.RawMessage, spaceID string) string {
	for _, rec := range table {
		e := entityOf(rec)
		if e == nil {
			continue
		}
		if asString(e["space_id"]) == spaceID {
			return asString(e["id"])
		}
	}
	return ""
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}
