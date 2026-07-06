package v3

import (
	"reflect"
	"sort"
	"testing"
)

func TestCollectDiscussionIDs(t *testing.T) {
	rm := decodeRecordMap(t, `{
		"block": {
			"page-1": {"value": {"id": "page-1", "type": "page", "alive": true, "content": ["child-1", "child-2"], "discussions": ["disc-a"]}, "role": "reader"},
			"child-1": {"value": {"id": "child-1", "type": "text", "alive": true, "parent_id": "page-1", "discussions": ["disc-b", "disc-a"]}, "role": "reader"},
			"child-2": {"value": {"id": "child-2", "type": "text", "alive": true, "parent_id": "page-1"}, "role": "reader"}
		}
	}`)

	ids := CollectDiscussionIDs(rm, "page-1")
	sort.Strings(ids)
	if !reflect.DeepEqual(ids, []string{"disc-a", "disc-b"}) {
		t.Errorf("ids = %v", ids)
	}
}

func TestCollectDiscussionIDsIgnoresBlocksOutsideTree(t *testing.T) {
	rm := decodeRecordMap(t, `{
		"block": {
			"page-1": {"value": {"id": "page-1", "type": "page", "alive": true, "content": []}, "role": "reader"},
			"stranger": {"value": {"id": "stranger", "type": "text", "alive": true, "discussions": ["disc-x"]}, "role": "reader"}
		}
	}`)

	if ids := CollectDiscussionIDs(rm, "page-1"); len(ids) != 0 {
		t.Errorf("ids = %v, want none", ids)
	}
	if ids := CollectDiscussionIDs(RecordMap{}, "nope"); len(ids) != 0 {
		t.Errorf("missing page ids = %v, want none", ids)
	}
}

func TestBuildAnchorTextMap(t *testing.T) {
	rm := decodeRecordMap(t, `{
		"block": {
			"b1": {"value": {"id": "b1", "type": "text", "alive": true, "properties": {"title": [["Hello "], ["world", [["m", "disc-1"]]]]}}, "role": "reader"}
		},
		"discussion": {
			"disc-1": {"value": {"id": "disc-1", "version": 1, "parent_id": "b1", "parent_table": "block", "resolved": false, "comments": []}, "role": "reader"}
		}
	}`)

	anchors := BuildAnchorTextMap(rm, []string{"disc-1"})
	if anchors["disc-1"] != "world" {
		t.Errorf("anchors = %v", anchors)
	}
}

func TestBuildAnchorTextMapSkipsNonBlockParents(t *testing.T) {
	rm := decodeRecordMap(t, `{
		"discussion": {
			"disc-1": {"value": {"id": "disc-1", "version": 1, "parent_id": "page-1", "parent_table": "space", "resolved": false, "comments": []}, "role": "reader"}
		}
	}`)

	if anchors := BuildAnchorTextMap(rm, []string{"disc-1"}); len(anchors) != 0 {
		t.Errorf("anchors = %v, want empty", anchors)
	}
}

func TestFindOccurrence(t *testing.T) {
	cases := []struct {
		plain, text          string
		occurrence           int
		wantStart, wantFound int
	}{
		{"hello world hello", "hello", 1, 0, 1},
		{"hello world hello", "hello", 2, 12, 2},
		{"hello world hello", "hello", 3, -1, 2},
		{"hello", "missing", 1, -1, 0},
		{"aaaa", "aa", 2, 1, 2}, // overlapping matches advance one rune at a time
		{"héllo wörld", "wörld", 1, 6, 1},
		{"hello", "", 1, -1, 0},
		{"hello", "hello", 0, -1, 0},
	}
	for _, c := range cases {
		start, found := FindOccurrence(c.plain, c.text, c.occurrence)
		if start != c.wantStart || found != c.wantFound {
			t.Errorf("FindOccurrence(%q, %q, %d) = (%d, %d), want (%d, %d)",
				c.plain, c.text, c.occurrence, start, found, c.wantStart, c.wantFound)
		}
	}
}
