// NDJSON (newline-delimited JSON) stream parsing for the v3 streaming
// endpoints. Notion occasionally emits blank or partial lines, so malformed
// lines are skipped rather than surfaced as errors.

package v3

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
)

// maxNDJSONLine caps a single NDJSON line. Inference events can carry large
// accumulated content, so this is generous.
const maxNDJSONLine = 10 << 20 // 10 MiB

// ParseNDJSON reads newline-delimited JSON from r and calls fn once per valid
// JSON line. Blank and malformed lines are skipped (matching the TS parser,
// which tolerates the empty/partial lines Notion occasionally sends). fn
// returning a non-nil error stops parsing and is returned to the caller.
//
// The json.RawMessage passed to fn is a fresh copy, safe to retain.
func ParseNDJSON(r io.Reader, fn func(json.RawMessage) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxNDJSONLine)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if !json.Valid(line) {
			continue
		}
		raw := make(json.RawMessage, len(line))
		copy(raw, line)
		if err := fn(raw); err != nil {
			return err
		}
	}
	return scanner.Err()
}
