package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zipServer serves fixed bytes as a stand-in for the export download URL.
func zipServer(t *testing.T, body string) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}

func TestExportPage(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	dl := zipServer(t, "PK-ZIPDATA")
	s.HandleBody("enqueueTask", map[string]any{"taskId": "task-1"})
	s.HandleBody("getTasks", map[string]any{"results": []any{map[string]any{
		"id": "task-1", "eventName": "exportBlock", "state": "success",
		"status": map[string]any{"pagesExported": 10, "exportURL": dl},
	}}})

	outPath := filepath.Join(t.TempDir(), "export.zip")
	stdout, _, err := runCLI(t, "", "--base-url", url, "export", "page", "pg1", "--output", outPath)
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, stdout)[0]
	if item["format"] != "markdown" || item["pages_exported"] != float64(10) || item["recursive"] != false {
		t.Errorf("export output = %v", item)
	}
	if !strings.HasSuffix(item["exported"].(string), "export.zip") {
		t.Errorf("exported path = %v", item["exported"])
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "PK-ZIPDATA" {
		t.Errorf("downloaded bytes = %q", data)
	}

	// The enqueued task carries the exportBlock event.
	var body struct {
		Task struct {
			EventName string `json:"eventName"`
		} `json:"task"`
	}
	if err := json.Unmarshal(s.CallsFor("enqueueTask")[0].Body, &body); err != nil {
		t.Fatal(err)
	}
	if body.Task.EventName != "exportBlock" {
		t.Errorf("eventName = %q", body.Task.EventName)
	}
}

func TestExportPollResumes(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	dl := zipServer(t, "ZIP")
	s.HandleBody("getTasks", map[string]any{"results": []any{map[string]any{
		"id": "task-9", "eventName": "exportSpace", "state": "success",
		"status": map[string]any{"pagesExported": 3, "exportURL": dl},
	}}})

	outPath := filepath.Join(t.TempDir(), "resume.zip")
	stdout, _, err := runCLI(t, "", "--base-url", url, "export", "poll", "task-9", "--output", outPath)
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, stdout)[0]
	if item["pages_exported"] != float64(3) {
		t.Errorf("poll output = %v", item)
	}
	if len(s.CallsFor("enqueueTask")) != 0 {
		t.Error("poll must not enqueue a new task")
	}
}

func TestExportPageFailureState(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	s.HandleBody("enqueueTask", map[string]any{"taskId": "task-1"})
	s.HandleBody("getTasks", map[string]any{"results": []any{map[string]any{
		"id": "task-1", "eventName": "exportBlock", "state": "failure", "error": "boom",
	}}})

	_, _, err := runCLI(t, "", "--base-url", url, "export", "page", "pg1", "--output", filepath.Join(t.TempDir(), "x.zip"))
	if err == nil || !strings.Contains(err.Error(), "Export failed") {
		t.Errorf("err = %v", err)
	}
}

func TestExportInvalidFormat(t *testing.T) {
	isolateState(t)
	seedV3Session(t)

	_, _, err := runCLI(t, "", "export", "page", "pg1", "--format", "pdf")
	if err == nil || !strings.Contains(err.Error(), "Invalid format") {
		t.Errorf("err = %v", err)
	}
}
