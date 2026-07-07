package v3

import (
	"encoding/json"
	"testing"
)

func parse(t *testing.T, raw string) SessionInfo {
	t.Helper()
	var data map[string]map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	info, err := ParseGetSpacesSession(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return info
}

func TestParseRoleWrappedRecords(t *testing.T) {
	info := parse(t, `{
      "user-map-id": {
        "notion_user": {
          "u1": {"value": {"value": {"id": "u1", "email": "alice@example.com", "name": "Alice"}, "role": "reader"}}
        },
        "space": {
          "s1": {"value": {"value": {"id": "s1", "name": "Acme", "plan_type": "team"}, "role": "editor"}}
        },
        "space_view": {
          "v1": {"value": {"value": {"id": "v1", "space_id": "s1"}, "role": "reader"}}
        }
      }
    }`)

	if info.UserID != "user-map-id" || info.UserEmail != "alice@example.com" || info.UserName != "Alice" {
		t.Errorf("user = %+v", info)
	}
	if info.SpaceID != "s1" || info.SpaceName != "Acme" || info.SpaceViewID != "v1" {
		t.Errorf("space = %+v", info)
	}
}

func TestParseShallowRecords(t *testing.T) {
	info := parse(t, `{
      "user-map-id": {
        "notion_user": {"u1": {"value": {"id": "u1", "email": "bob@example.com", "name": "Bob"}}},
        "space": {"s1": {"value": {"id": "s1", "name": "Personal"}}}
      }
    }`)

	if info.UserEmail != "bob@example.com" || info.SpaceName != "Personal" {
		t.Errorf("got %+v", info)
	}
	if info.SpaceViewID != "" {
		t.Errorf("space_view_id should be empty, got %q", info.SpaceViewID)
	}
}

func TestPreferTeamSpaceOverFree(t *testing.T) {
	info := parse(t, `{
      "u": {
        "space": {
          "free": {"value": {"id": "free", "name": "Personal", "plan_type": "personal"}},
          "team": {"value": {"id": "team", "name": "Work", "plan_type": "enterprise"}}
        }
      }
    }`)
	if info.SpaceID != "team" || info.SpaceName != "Work" {
		t.Errorf("expected enterprise space to win, got %+v", info)
	}
}

func TestParseEmptyResponseErrors(t *testing.T) {
	var data map[string]map[string]json.RawMessage
	if err := json.Unmarshal([]byte(`{}`), &data); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseGetSpacesSession(data); err == nil {
		t.Error("expected error for empty response")
	}
}

// TestParseTolueratesVersionNumber pins the getSpaces fix: the API now includes
// a "__version__" number alongside the record tables inside each user entry,
// which must not fail the parse.
func TestParseToleratesVersionNumber(t *testing.T) {
	info := parse(t, `{
      "u": {
        "__version__": 5,
        "notion_user": {"u1": {"value": {"id": "u1", "email": "cy@example.com", "name": "Cy"}}},
        "space": {"s1": {"value": {"id": "s1", "name": "Space"}}}
      }
    }`)
	if info.UserEmail != "cy@example.com" || info.SpaceName != "Space" {
		t.Errorf("got %+v", info)
	}
}
