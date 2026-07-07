package v3

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RichText is Notion v3's inline text format: an ordered list of
// [text, decorations?] tuples.
type RichText []Segment

// Segment is one rich-text run: literal text plus the decorations applied to
// every character of it.
type Segment struct {
	Text        string
	Decorations []Decoration
}

// Decoration is one v3 text decoration in wire form [type, ...args] — e.g.
// ["b"] (bold), ["a", url] (anchor), ["m", discussionID] (comment mark),
// ["u", userID] / ["p", pageID] / ["d", {start_date,…}] (mentions).
type Decoration struct {
	Type string
	Args []any
}

// Plain concatenates the segment texts, dropping decorations.
func (rt RichText) Plain() string {
	var b strings.Builder
	for _, s := range rt {
		b.WriteString(s.Text)
	}
	return b.String()
}

// Render concatenates the segment texts like Plain, but resolves inline mention
// decorations to readable text via the record map: person mentions become
// "@Given Family", page mentions become the target page's title, and date
// mentions become the date. Notion stores each mention as a "‣" placeholder
// segment with the real target in its decorations, so Plain alone loses them.
// Segments without a resolvable mention keep their literal text, so bold/link/
// etc. decorations pass through unchanged.
func (rt RichText) Render(rm RecordMap) string {
	var b strings.Builder
	for _, s := range rt {
		b.WriteString(s.render(rm))
	}
	return b.String()
}

// render resolves a single segment's inline mention to readable text, falling
// back to the literal text when there is no mention or it can't be resolved.
func (s Segment) render(rm RecordMap) string {
	for _, d := range s.Decorations {
		switch d.Type {
		case "u": // person mention ["u", userID]
			if u, ok := rm.GetUser(d.StringArg(0)); ok {
				if name := strings.TrimSpace(u.GivenName + " " + u.FamilyName); name != "" {
					return "@" + name
				}
				// The page chunk often ships a mentioned user as id+email only;
				// the email is a readable identity, so fall back to it verbatim.
				if u.Email != "" {
					return u.Email
				}
			}
		case "p": // page mention ["p", pageID]
			if blk, ok := rm.GetBlock(d.StringArg(0)); ok {
				if title := blk.Property("title").Plain(); title != "" {
					return title
				}
			}
		case "d": // date mention ["d", {start_date,…}]
			if txt := dateMentionText(d.ObjectArg(0)); txt != "" {
				return txt
			}
		}
	}
	return s.Text
}

// dateMentionText renders a date decoration's object as "start", "start time",
// or "start → end". Returns "" when there is no start date.
func dateMentionText(obj map[string]any) string {
	if obj == nil {
		return ""
	}
	start, _ := obj["start_date"].(string)
	if start == "" {
		return ""
	}
	if t, _ := obj["start_time"].(string); t != "" {
		start += " " + t
	}
	if end, _ := obj["end_date"].(string); end != "" {
		return start + " → " + end
	}
	return start
}

// NewRichText wraps plain text as a single undecorated segment.
func NewRichText(text string) RichText {
	return RichText{{Text: text}}
}

// MarshalJSON emits the [text] / [text, decorations] tuple wire form.
func (s Segment) MarshalJSON() ([]byte, error) {
	if len(s.Decorations) == 0 {
		return json.Marshal([]any{s.Text})
	}
	return json.Marshal([]any{s.Text, s.Decorations})
}

// UnmarshalJSON accepts the [text] / [text, decorations] tuple wire form.
func (s *Segment) UnmarshalJSON(data []byte) error {
	var parts []json.RawMessage
	if err := json.Unmarshal(data, &parts); err != nil {
		return fmt.Errorf("rich text segment is not an array: %w", err)
	}
	if len(parts) == 0 {
		return fmt.Errorf("rich text segment is empty")
	}
	if err := json.Unmarshal(parts[0], &s.Text); err != nil {
		return fmt.Errorf("rich text segment text: %w", err)
	}
	s.Decorations = nil
	if len(parts) > 1 {
		if err := json.Unmarshal(parts[1], &s.Decorations); err != nil {
			return fmt.Errorf("rich text segment decorations: %w", err)
		}
	}
	return nil
}

// MarshalJSON emits the [type, ...args] tuple wire form.
func (d Decoration) MarshalJSON() ([]byte, error) {
	tuple := make([]any, 0, 1+len(d.Args))
	tuple = append(tuple, d.Type)
	tuple = append(tuple, d.Args...)
	return json.Marshal(tuple)
}

// UnmarshalJSON accepts the [type, ...args] tuple wire form.
func (d *Decoration) UnmarshalJSON(data []byte) error {
	var tuple []any
	if err := json.Unmarshal(data, &tuple); err != nil {
		return fmt.Errorf("decoration is not an array: %w", err)
	}
	if len(tuple) == 0 {
		return fmt.Errorf("decoration is empty")
	}
	t, ok := tuple[0].(string)
	if !ok {
		return fmt.Errorf("decoration type is not a string")
	}
	d.Type = t
	d.Args = tuple[1:]
	return nil
}

// StringArg returns decoration argument i as a string, "" when absent or not
// a string.
func (d Decoration) StringArg(i int) string {
	if i >= len(d.Args) {
		return ""
	}
	s, _ := d.Args[i].(string)
	return s
}

// ObjectArg returns decoration argument i as an object, nil when absent or
// not an object.
func (d Decoration) ObjectArg(i int) map[string]any {
	if i >= len(d.Args) {
		return nil
	}
	m, _ := d.Args[i].(map[string]any)
	return m
}
