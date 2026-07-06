// Package ids normalizes Notion identifiers: URLs and copy-pasted IDs arrive
// dashless, the APIs want dashed UUIDs.
package ids

import "strings"

// Normalize accepts a dashless (from URLs) or dashed (standard UUID) Notion
// ID and returns the dashed form. Anything that is not 32 hex chars after
// stripping dashes is returned unchanged, letting the API reject it.
func Normalize(id string) string {
	cleaned := strings.ToLower(strings.ReplaceAll(id, "-", ""))
	if len(cleaned) != 32 || !isHex(cleaned) {
		return id
	}
	return cleaned[0:8] + "-" + cleaned[8:12] + "-" + cleaned[12:16] + "-" + cleaned[16:20] + "-" + cleaned[20:]
}

func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
