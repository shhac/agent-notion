package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func tokenServer(t *testing.T, handler func(grant map[string]string, w http.ResponseWriter)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte("client-id:client-secret"))
		if r.Header.Get("Authorization") != wantBasic {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		var grant map[string]string
		if err := json.NewDecoder(r.Body).Decode(&grant); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		handler(grant, w)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestExchange(t *testing.T) {
	srv := tokenServer(t, func(grant map[string]string, w http.ResponseWriter) {
		if grant["grant_type"] != "authorization_code" || grant["code"] != "the-code" ||
			grant["redirect_uri"] != "http://localhost:9876/callback" {
			t.Errorf("grant = %v", grant)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":   "atk",
			"refresh_token":  "rtk",
			"bot_id":         "bot-1",
			"workspace_id":   "ws-1",
			"workspace_name": "Test Space",
			"owner": map[string]any{
				"type": "user",
				"user": map[string]any{
					"id":     "user-1",
					"name":   "Test User",
					"person": map[string]any{"email": "test@example.com"},
				},
			},
		})
	})

	tok, err := TokenClient{URL: srv.URL}.Exchange(context.Background(),
		"client-id", "client-secret", "the-code", "http://localhost:9876/callback")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "atk" || tok.RefreshToken != "rtk" || tok.WorkspaceName != "Test Space" {
		t.Errorf("token = %+v", tok)
	}
	if tok.Owner == nil || tok.Owner.User.Person.Email != "test@example.com" {
		t.Errorf("owner = %+v", tok.Owner)
	}
}

func TestRefresh(t *testing.T) {
	srv := tokenServer(t, func(grant map[string]string, w http.ResponseWriter) {
		if grant["grant_type"] != "refresh_token" || grant["refresh_token"] != "old-refresh" {
			t.Errorf("grant = %v", grant)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
		})
	})

	tok, err := TokenClient{URL: srv.URL}.Refresh(context.Background(),
		"client-id", "client-secret", "old-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "new-access" || tok.RefreshToken != "new-refresh" {
		t.Errorf("token = %+v", tok)
	}
}

func TestExchangeErrorSurfacesAPIMessage(t *testing.T) {
	srv := tokenServer(t, func(_ map[string]string, w http.ResponseWriter) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_grant",
			"message": "the code has expired",
		})
	})

	_, err := TokenClient{URL: srv.URL}.Exchange(context.Background(),
		"client-id", "client-secret", "stale", "http://localhost:9876/callback")
	if err == nil || !strings.Contains(err.Error(), "the code has expired") {
		t.Errorf("err = %v, want API message surfaced", err)
	}
}

func TestAuthorizeURL(t *testing.T) {
	got := AuthorizeURL("cid", "http://localhost:9880/callback", "st4te")
	for _, want := range []string{
		"https://api.notion.com/v1/oauth/authorize?",
		"client_id=cid",
		"redirect_uri=http%3A%2F%2Flocalhost%3A9880%2Fcallback",
		"response_type=code",
		"owner=user",
		"state=st4te",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("AuthorizeURL missing %q: %s", want, got)
		}
	}
}
