package v3

import (
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/mocknotion"
	"github.com/shhac/agent-notion/internal/notion"
)

// updateBlockParamsFor builds UpdateBlockParams with optional content.
func updateBlockParamsFor(id string, content *string) notion.UpdateBlockParams {
	return notion.UpdateBlockParams{ID: id, Content: content}
}

// opPathJoined renders an op's path slice for contains-checks.
func opPathJoined(op map[string]any) string {
	parts, _ := op["path"].([]any)
	joined := ""
	for _, p := range parts {
		if s, ok := p.(string); ok {
			joined += s + "/"
		}
	}
	return joined
}

// syncBlockBody answers syncRecordValuesMain with one block record.
func syncBlockBody(id string, overrides map[string]any) map[string]any {
	return mocknotion.RecordMapBody(map[string]map[string]any{
		"block": {id: mocknotion.BlockEntity(id, "text", overrides)},
	})
}

// TestBackendUpdateBlock covers the fetch-then-save wiring the CLI tests only
// exercise on the official backend: the block must exist, and a nil Content
// still saves edit-meta ops.
func TestBackendUpdateBlock(t *testing.T) {
	s := mocknotion.New()
	s.HandleBody("syncRecordValuesMain", syncBlockBody("block-1", nil))
	b := newBackend(t, s)

	content := "updated text"
	res, err := b.UpdateBlock(ctx(), updateBlockParamsFor("block-1", &content))
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "block-1" || res.LastEditedAt == "" {
		t.Errorf("res = %#v", res)
	}

	ops := opsFromSave(t, s)
	foundTitle := false
	for _, op := range ops {
		if opCmd(op) == "set" && strings.Contains(opPathJoined(op), "properties") {
			foundTitle = true
		}
	}
	if !foundTitle {
		t.Errorf("no properties set op: %#v", ops)
	}
}

func TestBackendUpdateBlockNilContentStillSavesEditMeta(t *testing.T) {
	s := mocknotion.New()
	s.HandleBody("syncRecordValuesMain", syncBlockBody("block-1", nil))
	b := newBackend(t, s)

	if _, err := b.UpdateBlock(ctx(), updateBlockParamsFor("block-1", nil)); err != nil {
		t.Fatal(err)
	}
	ops := opsFromSave(t, s)
	if len(ops) == 0 || opCmd(ops[len(ops)-1]) != "update" {
		t.Errorf("expected trailing editMeta op: %#v", ops)
	}
}

func TestBackendUpdateBlockNotFound(t *testing.T) {
	s := mocknotion.New()
	s.HandleBody("syncRecordValuesMain", mocknotion.RecordMapBody(map[string]map[string]any{"block": {}}))
	b := newBackend(t, s)

	if _, err := b.UpdateBlock(ctx(), updateBlockParamsFor("missing", nil)); err == nil ||
		!strings.Contains(err.Error(), "Block not found") {
		t.Errorf("err = %v", err)
	}
}

// TestBackendDeleteBlock pins the highest-stakes wiring detail: the trash ops
// must target the FETCHED parent, not a guess — a wrong parent table would
// emit listRemove against the wrong record.
func TestBackendDeleteBlock(t *testing.T) {
	s := mocknotion.New()
	s.HandleBody("syncRecordValuesMain", syncBlockBody("block-1", map[string]any{
		"parent_id": "the-real-parent", "parent_table": "block",
	}))
	b := newBackend(t, s)

	res, err := b.DeleteBlock(ctx(), "block-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "block-1" || !res.Deleted {
		t.Errorf("res = %#v", res)
	}

	ops := opsFromSave(t, s)
	if len(ops) != 3 {
		t.Fatalf("ops = %d, want 3", len(ops))
	}
	if opCmd(ops[1]) != "listRemove" {
		t.Fatalf("ops[1] = %#v", ops[1])
	}
	ptr := opPointer(ops[1])
	if ptr["id"] != "the-real-parent" || ptr["table"] != "block" {
		t.Errorf("listRemove pointer = %v, want the fetched parent", ptr)
	}
}

func TestBackendDeleteBlockNotFound(t *testing.T) {
	s := mocknotion.New()
	s.HandleBody("syncRecordValuesMain", mocknotion.RecordMapBody(map[string]map[string]any{"block": {}}))
	b := newBackend(t, s)

	if _, err := b.DeleteBlock(ctx(), "missing"); err == nil ||
		!strings.Contains(err.Error(), "Block not found") {
		t.Errorf("err = %v", err)
	}
}
