// Package truncation implements convention-based field truncation for
// LLM-facing output: fields named "description", "body", or "content" are
// capped, and every string-valued truncatable field gets a companion
// "{field}Length" carrying the full size so an agent knows to re-fetch with
// --expand/--full.
package truncation

import "strings"

// DefaultMaxLength caps truncatable fields unless overridden.
const DefaultMaxLength = 200

const ellipsis = "…"

var truncatableFields = map[string]bool{
	"description": true,
	"body":        true,
	"content":     true,
}

// Options configures a Truncator, typically from the --expand/--full/
// settings.truncation.max_length CLI surface.
type Options struct {
	// Expand is a comma-separated list of field names left untruncated.
	Expand string
	// Full leaves every field untruncated (wins over Expand).
	Full bool
	// MaxLength overrides DefaultMaxLength when > 0.
	MaxLength int
}

// Truncator applies one CLI invocation's truncation policy.
type Truncator struct {
	expandAll bool
	expanded  map[string]bool
	maxLength int
}

// New builds a Truncator from opts.
func New(opts Options) *Truncator {
	t := &Truncator{expanded: map[string]bool{}, maxLength: opts.MaxLength}
	if t.maxLength <= 0 {
		t.maxLength = DefaultMaxLength
	}
	if opts.Full {
		t.expandAll = true
		return t
	}
	for _, f := range strings.Split(opts.Expand, ",") {
		if f = strings.ToLower(strings.TrimSpace(f)); f != "" {
			t.expanded[f] = true
		}
	}
	return t
}

// Apply walks data (maps, slices, and primitives — the shapes JSON
// marshaling handles), truncating truncatable string fields and adding the
// companion length fields. Lengths count runes, not bytes.
func (t *Truncator) Apply(data any) any {
	switch v := data.(type) {
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = t.Apply(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, value := range v {
			s, isString := value.(string)
			switch {
			case truncatableFields[key] && isString:
				runes := []rune(s)
				out[key+"Length"] = len(runes)
				if t.expandAll || t.expanded[key] || len(runes) <= t.maxLength {
					out[key] = s
				} else {
					out[key] = string(runes[:t.maxLength]) + ellipsis
				}
			case value != nil && !isString:
				out[key] = t.Apply(value)
			default:
				out[key] = value
			}
		}
		return out
	default:
		return data
	}
}
