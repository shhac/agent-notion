// Package auth extracts a Notion token_v2 session cookie from the Notion
// Desktop app or a web browser's cookie store.
//
// It reads Chromium, Firefox, and Safari cookie databases directly on disk —
// no running browser required — decrypting Chromium values with the platform
// Safe Storage password (macOS keychain / Linux secret-tool / Windows DPAPI).
// The design mirrors agent-slack's internal/auth, adapted for Notion: the
// token is a plain cookie value with no distinctive prefix, so the caller
// strips the Chromium meta-version ≥24 SHA-256 domain-hash prefix explicitly
// rather than scanning for a token signature.
package auth

// Session is an extracted Notion desktop session.
type Session struct {
	TokenV2 string            `json:"token_v2"`
	Source  map[string]string `json:"source,omitempty"`
}

// cookieName is the Notion session cookie.
const cookieName = "token_v2"

// hostLike matches Notion cookie host_key / host values in SQL LIKE form.
const hostLike = "%notion.so"
