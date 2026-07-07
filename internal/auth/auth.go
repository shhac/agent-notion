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

import "strings"

// Session is an extracted Notion desktop session.
type Session struct {
	TokenV2 string            `json:"token_v2"`
	Source  map[string]string `json:"source,omitempty"`
}

// cookieName is the Notion session cookie.
const cookieName = "token_v2"

// hostClause is a SQL predicate on the given host column matching Notion's
// cookie hosts. Notion serves the session across two domains — www.notion.so
// (legacy) and app.notion.com (current; the Desktop app uses it) — so both
// must match, or the token is missed entirely on one of them.
func hostClause(col string) string {
	return "(" + col + " like '%notion.so' or " + col + " like '%notion.com')"
}

// isNotionDomain reports whether a cookie domain belongs to Notion (either
// domain). The string form of hostClause, for the Safari path.
func isNotionDomain(domain string) bool {
	return strings.Contains(domain, "notion.so") || strings.Contains(domain, "notion.com")
}
