// Notion AI patch engine — normalizes the JSON-patch stream used for thread
// replies into standard NdjsonEvents by maintaining slot state and applying
// JSON-pointer ops in place.

package v3

import (
	"encoding/json"
	"strconv"
	"strings"
)

// NormalizePatchStream converts a patch-format event slice (used for thread
// replies) into standard NdjsonEvents, in order.
func NormalizePatchStream(events []NdjsonEvent) []NdjsonEvent {
	var out []NdjsonEvent
	pn := &patchNormalizer{emit: func(e NdjsonEvent) error {
		out = append(out, e)
		return nil
	}}
	for _, e := range events {
		_ = pn.handle(e)
	}
	return out
}

// patchNormalizer tracks patch-start slot state and emits synthetic
// agent-inference events as patches apply.
type patchNormalizer struct {
	slots []map[string]any
	emit  func(NdjsonEvent) error
}

func (p *patchNormalizer) handle(e NdjsonEvent) error {
	switch e.Type {
	case "patch-start":
		var ps struct {
			Data struct {
				S []map[string]any `json:"s"`
			} `json:"data"`
		}
		_ = json.Unmarshal(e.Raw, &ps)
		p.slots = ps.Data.S
		if p.slots == nil {
			p.slots = []map[string]any{}
		}
		return nil

	case "patch":
		var pe struct {
			V []PatchOp `json:"v"`
		}
		if json.Unmarshal(e.Raw, &pe) != nil {
			return nil
		}
		for _, op := range pe.V {
			applyPatchOp(&p.slots, op.O, op.P, op.V)
		}
		for _, slot := range p.slots {
			if slot["type"] == "agent-inference" {
				raw, err := json.Marshal(slot)
				if err != nil {
					break
				}
				return p.emit(NdjsonEvent{Type: "agent-inference", Raw: raw})
			}
		}
		return nil

	default:
		return p.emit(e)
	}
}

// applyPatchOp applies one JSON-pointer patch op to the slots array in place.
// Supports "a" (add/append), "x" (string append), and "r" (replace/remove).
func applyPatchOp(slots *[]map[string]any, op, path string, value any) {
	parts := pathParts(path)
	if len(parts) < 2 || parts[0] != "s" {
		return
	}
	slotKey := parts[1]

	if slotKey == "-" && op == "a" {
		m, _ := value.(map[string]any)
		*slots = append(*slots, m)
		return
	}

	idx, err := strconv.Atoi(slotKey)
	if err != nil || idx < 0 || idx >= len(*slots) {
		return
	}
	s := *slots

	if len(parts) == 2 {
		if op == "r" {
			if m, ok := value.(map[string]any); ok {
				s[idx] = m
			}
		}
		return
	}

	fieldParts := parts[2:]
	var parent any = s
	parentKey := slotKey
	var target any = s[idx]
	for i := 0; i < len(fieldParts)-1; i++ {
		key := fieldParts[i]
		parent = target
		parentKey = key
		target = navigate(target, key)
		if target == nil {
			return
		}
	}
	applyLeaf(parent, parentKey, target, fieldParts[len(fieldParts)-1], op, value)
}

// navigate steps into a map key or array index, "" (nil) on any miss.
func navigate(target any, key string) any {
	switch t := target.(type) {
	case map[string]any:
		return t[key]
	case []any:
		if key == "-" {
			return nil
		}
		i, err := strconv.Atoi(key)
		if err != nil || i < 0 || i >= len(t) {
			return nil
		}
		return t[i]
	default:
		return nil
	}
}

// applyLeaf performs the op at the leaf. For array length changes (append,
// splice) the mutated slice is written back through parent[parentKey].
func applyLeaf(parent any, parentKey string, target any, lastKey, op string, value any) {
	switch t := target.(type) {
	case []any:
		if lastKey == "-" {
			if op == "a" {
				setChild(parent, parentKey, append(t, value))
			}
			return
		}
		i, err := strconv.Atoi(lastKey)
		if err != nil || i < 0 {
			return
		}
		switch op {
		case "a":
			switch {
			case i < len(t):
				t[i] = value
			case i == len(t):
				setChild(parent, parentKey, append(t, value))
			}
		case "r":
			if i < len(t) {
				setChild(parent, parentKey, append(t[:i], t[i+1:]...))
			}
		case "x":
			if i < len(t) {
				if s, ok := t[i].(string); ok {
					if add, ok := value.(string); ok {
						t[i] = s + add
					}
				}
			}
		}
	case map[string]any:
		switch op {
		case "a":
			t[lastKey] = value
		case "r":
			delete(t, lastKey)
		case "x":
			if s, ok := t[lastKey].(string); ok {
				if add, ok := value.(string); ok {
					t[lastKey] = s + add
				}
			}
		}
	}
}

// setChild writes val back into a map key or array index.
func setChild(parent any, key string, val []any) {
	switch p := parent.(type) {
	case map[string]any:
		p[key] = val
	case []any:
		if i, err := strconv.Atoi(key); err == nil && i >= 0 && i < len(p) {
			p[i] = val
		}
	}
}

// pathParts splits a JSON pointer into non-empty segments.
func pathParts(path string) []string {
	raw := strings.Split(path, "/")
	parts := make([]string, 0, len(raw))
	for _, p := range raw {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}
