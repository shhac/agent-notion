// Package auth extracts a Notion token_v2 session cookie from the Notion
// Desktop app or a web browser's cookie store.
//
// The cross-platform cookie mechanism — locating and reading Chromium, Firefox,
// and Safari stores on disk and decrypting Chromium values — lives in the
// shared github.com/shhac/lib-agent-browsercookies library. This package is a
// thin Notion-policy adapter over it: which cookie (token_v2), which hosts
// (notion.so + notion.com), and where the Desktop app keeps its store.
package auth

import browsercookies "github.com/shhac/lib-agent-browsercookies"

// Session is an extracted Notion desktop session.
type Session struct {
	TokenV2 string            `json:"token_v2"`
	Source  map[string]string `json:"source,omitempty"`
}

// notionTarget is the extraction policy for Notion's session cookie. Notion
// serves the session across two domains — notion.so (legacy) and notion.com
// (current; the Desktop app uses app.notion.com) — so both must match, or the
// token is missed on one of them. The value is returned verbatim: browsers
// transmit cookie values byte-for-byte, and Notion's token_v2 is not
// percent-encoded, so no decode is wanted.
var notionTarget = browsercookies.Target{
	CookieName:   "token_v2",
	HostSuffixes: []string{"notion.so", "notion.com"},
}
