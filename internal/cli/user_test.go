package cli

import "testing"

func TestUserList(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/users", map[string]any{
		"has_more":    false,
		"next_cursor": nil,
		"results": []any{map[string]any{
			"id": "u1", "name": "Al", "type": "person",
			"person": map[string]any{"email": "al@example.test"},
		}},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "user", "list")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["name"] != "Al" || item["email"] != "al@example.test" {
		t.Errorf("user list output = %v", item)
	}
}

func TestUserMe(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/users/me", map[string]any{
		"id": "bot1", "name": "Bot", "type": "bot",
		"bot": map[string]any{"workspace_name": "WS"},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "user", "me")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["id"] != "bot1" || item["workspace_name"] != "WS" {
		t.Errorf("user me output = %v", item)
	}
}
