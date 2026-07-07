// Package v3 talks to Notion's unofficial (token_v2) API. This file covers the
// getSpaces call used to validate a desktop/browser token and derive session
// identity, ported from the TS validateDesktopToken/parseGetSpacesSession.
package v3

import (
	"bytes"
	"context"
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

	// Each user entry is structurally a RecordMap, so the canonical decode
	// path handles all wire-format layering (role-wrapped records, __version__
	// metadata at any level) in one place.
	data, err := decodeObjectMap[RecordMap](body)
	if err != nil {
		return SessionInfo{}, fmt.Errorf("could not parse getSpaces response: %w", err)
	}
	return ParseGetSpacesSession(data)
}

// sessionUser is the slice of a notion_user record the session needs. It is
// deliberately not the canonical User struct: getSpaces user records carry a
// display "name" field that User (given_name/family_name) does not.
type sessionUser struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type sessionSpace struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	PlanType string `json:"plan_type"`
}

type sessionSpaceView struct {
	ID      string `json:"id"`
	SpaceID string `json:"space_id"`
}

// ParseGetSpacesSession extracts session info from a decoded getSpaces
// response, shaped userId → RecordMap. Only the first user entry (by sorted
// ID, for determinism) is considered.
func ParseGetSpacesSession(data map[string]RecordMap) (SessionInfo, error) {
	for _, userID := range sortedKeys(data) {
		rm := data[userID]
		user := findSessionUser(rm["notion_user"])
		spaceID, spaceName := pickPreferredSpace(rm["space"])
		return SessionInfo{
			UserID:      userID,
			UserEmail:   user.Email,
			UserName:    user.Name,
			SpaceID:     spaceID,
			SpaceName:   spaceName,
			SpaceViewID: findSpaceViewID(rm["space_view"], spaceID),
		}, nil
	}
	return SessionInfo{}, fmt.Errorf("could not extract user info from token; it may be expired")
}

// findSessionUser returns the first user record carrying an email.
func findSessionUser(t Table) sessionUser {
	for _, id := range sortedKeys(t) {
		u, ok := decodeEntry[sessionUser](t, id)
		if !ok || u.Email == "" {
			continue
		}
		return sessionUser{Email: strings.TrimSpace(u.Email), Name: strings.TrimSpace(u.Name)}
	}
	return sessionUser{}
}

// pickPreferredSpace returns a space id/name, preferring team/enterprise plans.
func pickPreferredSpace(t Table) (id, name string) {
	var firstID, firstName string
	found := false
	for _, rid := range sortedKeys(t) {
		s, ok := decodeEntry[sessionSpace](t, rid)
		if !ok || s.Name == "" {
			continue
		}
		if !found {
			firstID, firstName = strings.TrimSpace(s.ID), strings.TrimSpace(s.Name)
			found = true
		}
		if s.PlanType == "team" || s.PlanType == "enterprise" {
			return strings.TrimSpace(s.ID), strings.TrimSpace(s.Name)
		}
	}
	return firstID, firstName
}

func findSpaceViewID(t Table, spaceID string) string {
	for _, rid := range sortedKeys(t) {
		v, ok := decodeEntry[sessionSpaceView](t, rid)
		if ok && strings.TrimSpace(v.SpaceID) == spaceID {
			return strings.TrimSpace(v.ID)
		}
	}
	return ""
}
