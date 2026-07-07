package v3

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func collectNDJSON(t *testing.T, input string) []string {
	t.Helper()
	var lines []string
	err := ParseNDJSON(strings.NewReader(input), func(raw json.RawMessage) error {
		lines = append(lines, string(raw))
		return nil
	})
	if err != nil {
		t.Fatalf("ParseNDJSON: %v", err)
	}
	return lines
}

func TestParseNDJSONBasic(t *testing.T) {
	got := collectNDJSON(t, "{\"a\":1}\n{\"b\":2}\n")
	want := []string{`{"a":1}`, `{"b":2}`}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseNDJSONSkipsBlankAndMalformed(t *testing.T) {
	// Blank lines, whitespace-only lines, and non-JSON garbage are skipped.
	got := collectNDJSON(t, "\n  \n{\"ok\":true}\nnot json\n{\"also\":\"ok\"}\n")
	want := []string{`{"ok":true}`, `{"also":"ok"}`}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseNDJSONTrailingLineWithoutNewline(t *testing.T) {
	got := collectNDJSON(t, "{\"first\":1}\n{\"last\":2}")
	if len(got) != 2 || got[1] != `{"last":2}` {
		t.Fatalf("trailing line not flushed: %v", got)
	}
}

func TestParseNDJSONEmptyInput(t *testing.T) {
	if got := collectNDJSON(t, ""); len(got) != 0 {
		t.Fatalf("expected no lines, got %v", got)
	}
}

func TestParseNDJSONCallbackErrorStops(t *testing.T) {
	stop := errors.New("stop")
	var seen int
	err := ParseNDJSON(strings.NewReader("{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n"), func(raw json.RawMessage) error {
		seen++
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("err = %v, want stop", err)
	}
	if seen != 1 {
		t.Fatalf("callback ran %d times, want 1 (should stop on first error)", seen)
	}
}

func TestParseNDJSONDecodable(t *testing.T) {
	// The raw message handed to the callback is valid JSON that decodes.
	err := ParseNDJSON(strings.NewReader("{\"type\":\"text\",\"n\":42}\n"), func(raw json.RawMessage) error {
		var v struct {
			Type string `json:"type"`
			N    int    `json:"n"`
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		if v.Type != "text" || v.N != 42 {
			t.Errorf("decoded %+v", v)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
