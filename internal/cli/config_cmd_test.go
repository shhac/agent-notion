package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/config"
)

// runConfig runs a config subcommand, capturing os.Stdout — the lib's
// ConfigCommand writes its NDJSON records there directly rather than through
// cobra's output writer.
func runConfig(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	orig := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatal(pipeErr)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	_, _, err = runCLI(t, "", args...)

	_ = w.Close()
	os.Stdout = orig
	return <-done, err
}

func TestConfigSetGetRoundTrip(t *testing.T) {
	isolateState(t)

	out, err := runConfig(t, "config", "set", "page_size", "20")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["set"] != "page_size" || item["value"] != "20" {
		t.Errorf("set output = %v", item)
	}
	if got := config.ReadSettings().PageSize; got != 20 {
		t.Errorf("persisted page_size = %d", got)
	}

	out, err = runConfig(t, "config", "get", "page_size")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["key"] != "page_size" || item["value"] != "20" || item["set"] != true {
		t.Errorf("get output = %v", item)
	}
}

func TestConfigGetUnsetKey(t *testing.T) {
	isolateState(t)

	out, err := runConfig(t, "config", "get", "page_size")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["value"] != "" || item["set"] != false {
		t.Errorf("get on unset key = %v", item)
	}
}

func TestConfigSetValidation(t *testing.T) {
	isolateState(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"page_size zero", []string{"config", "set", "page_size", "0"}, "between 1 and 100"},
		{"page_size too big", []string{"config", "set", "page_size", "101"}, "between 1 and 100"},
		{"page_size non-int", []string{"config", "set", "page_size", "abc"}, "between 1 and 100"},
		{"max_depth non-int", []string{"config", "set", "max_depth", "abc"}, "positive integer"},
		{"max_depth negative", []string{"config", "set", "max_depth", "--", "-1"}, "positive integer"},
		{"truncation negative", []string{"config", "set", "truncation.max_length", "--", "-5"}, "positive integer"},
		{"empty model", []string{"config", "set", "ai.default_model", "   "}, "cannot be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runConfig(t, tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want contains %q", err, tt.wantErr)
			}
		})
	}

	// A rejected set must not persist anything.
	if config.Read().Settings != nil {
		t.Errorf("failed sets should not write settings: %+v", config.Read().Settings)
	}
}

func TestConfigNestedKeysRoundTrip(t *testing.T) {
	isolateState(t)

	if _, err := runConfig(t, "config", "set", "truncation.max_length", "500"); err != nil {
		t.Fatal(err)
	}
	if _, err := runConfig(t, "config", "set", "ai.default_model", "oatmeal-cookie"); err != nil {
		t.Fatal(err)
	}

	s := config.ReadSettings()
	if s.Truncation == nil || s.Truncation.MaxLength != 500 {
		t.Errorf("truncation = %+v", s.Truncation)
	}
	if s.AI == nil || s.AI.DefaultModel != "oatmeal-cookie" {
		t.Errorf("ai = %+v", s.AI)
	}

	out, err := runConfig(t, "config", "get", "ai.default_model")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["value"] != "oatmeal-cookie" || item["set"] != true {
		t.Errorf("get ai.default_model = %v", item)
	}
}

func TestConfigUnsetPrunesNested(t *testing.T) {
	isolateState(t)

	if _, err := runConfig(t, "config", "set", "truncation.max_length", "500"); err != nil {
		t.Fatal(err)
	}
	out, err := runConfig(t, "config", "unset", "truncation.max_length")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["unset"] != "truncation.max_length" {
		t.Errorf("unset output = %v", item)
	}
	// It was the only setting, so the whole settings object is gone.
	if config.Read().Settings != nil {
		t.Errorf("settings should be pruned after unsetting the last value: %+v", config.Read().Settings)
	}
}

func TestConfigUnsetAllPrunesSettingsObject(t *testing.T) {
	isolateState(t)

	for _, kv := range [][2]string{{"page_size", "20"}, {"max_depth", "3"}, {"ai.default_model", "oatmeal-cookie"}} {
		if _, err := runConfig(t, "config", "set", kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
	}
	if config.Read().Settings == nil {
		t.Fatal("settings should exist after sets")
	}

	for _, key := range []string{"page_size", "max_depth", "ai.default_model"} {
		if _, err := runConfig(t, "config", "unset", key); err != nil {
			t.Fatal(err)
		}
	}
	if config.Read().Settings != nil {
		t.Errorf("settings object should be dropped once all keys are unset: %+v", config.Read().Settings)
	}
}

func TestConfigList(t *testing.T) {
	isolateState(t)

	if _, err := runConfig(t, "config", "set", "page_size", "20"); err != nil {
		t.Fatal(err)
	}
	out, err := runConfig(t, "config", "list")
	if err != nil {
		t.Fatal(err)
	}
	items := decodeLines(t, out)
	if len(items) != 4 {
		t.Fatalf("expected 4 keys, got %d:\n%s", len(items), out)
	}
	byKey := map[string]map[string]any{}
	for _, it := range items {
		key, _ := it["key"].(string)
		byKey[key] = it
	}
	for _, want := range []string{"page_size", "max_depth", "truncation.max_length", "ai.default_model"} {
		if byKey[want] == nil {
			t.Errorf("list missing key %q", want)
		}
	}
	if byKey["page_size"]["value"] != "20" || byKey["page_size"]["set"] != true {
		t.Errorf("page_size row = %v", byKey["page_size"])
	}
	if byKey["max_depth"]["set"] != false {
		t.Errorf("max_depth should be unset: %v", byKey["max_depth"])
	}
}

func TestConfigUnknownKey(t *testing.T) {
	isolateState(t)

	_, err := runConfig(t, "config", "get", "bogus")
	if err == nil || !strings.Contains(err.Error(), "unknown config key") {
		t.Errorf("err = %v, want unknown-key error", err)
	}
}

func TestConfigUsageCard(t *testing.T) {
	isolateState(t)

	// The usage subcommand writes through cobra's output writer, so read it
	// from runCLI's buffer rather than os.Stdout.
	out, _, err := runCLI(t, "", "config", "usage")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"page_size", "truncation.max_length", "ai.default_model", "get/set/unset/list"} {
		if !strings.Contains(out, want) {
			t.Errorf("config usage missing %q", want)
		}
	}
}
