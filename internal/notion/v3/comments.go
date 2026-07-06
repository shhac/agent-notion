// V3 comments — the pure discussion/comment logic shared by `comment list`
// and `comment add`. The HTTP orchestration (loadPageChunk +
// syncRecordValues + saveTransactions) lands with the client in Phase 4.

package v3

import (
	"strings"
	"unicode/utf8"
)

// CollectDiscussionIDs walks a page block and its descendants (via content
// lists) and returns the discussion IDs they carry, in discovery order,
// deduplicated.
func CollectDiscussionIDs(rm RecordMap, pageID string) []string {
	var discussionIDs []string
	seenDiscussions := map[string]bool{}
	visited := map[string]bool{}
	queue := []string{pageID}

	for len(queue) > 0 {
		blockID := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if visited[blockID] {
			continue
		}
		visited[blockID] = true

		block, ok := rm.GetBlock(blockID)
		if !ok {
			continue
		}
		for _, discID := range block.Discussions {
			if !seenDiscussions[discID] {
				seenDiscussions[discID] = true
				discussionIDs = append(discussionIDs, discID)
			}
		}
		queue = append(queue, block.Content...)
	}

	return discussionIDs
}

// BuildAnchorTextMap maps discussion ID → the anchor text its inline comment
// is attached to, derived from the ["m", discussionID] decorations on the
// parent block's title. Discussions without a block parent or without anchor
// decorations are absent from the map.
func BuildAnchorTextMap(rm RecordMap, discussionIDs []string) map[string]string {
	anchors := map[string]string{}
	for _, discID := range discussionIDs {
		disc, ok := rm.GetDiscussion(discID)
		if !ok || disc.ParentTable != "block" {
			continue
		}
		parent, ok := rm.GetBlock(disc.ParentID)
		if !ok {
			continue
		}
		title := parent.Property("title")
		if len(title) == 0 {
			continue
		}
		if anchor := ExtractAnchorText(title, discID); anchor != "" {
			anchors[discID] = anchor
		}
	}
	return anchors
}

// FindOccurrence locates the nth occurrence (1-based) of text in plain,
// returning its rune offset. found reports how many occurrences exist up to
// the requested one, so callers can build precise error messages: start is
// -1 when the requested occurrence does not exist.
func FindOccurrence(plain, text string, occurrence int) (start, found int) {
	if occurrence < 1 || text == "" {
		return -1, 0
	}
	searchFrom := 0
	runesBefore := 0
	for found < occurrence {
		idx := strings.Index(plain[searchFrom:], text)
		if idx == -1 {
			return -1, found
		}
		found++
		runesBefore += len([]rune(plain[searchFrom : searchFrom+idx]))
		if found == occurrence {
			return runesBefore, found
		}
		// Advance one rune past the match start, mirroring the TS idx+1 scan.
		_, size := utf8.DecodeRuneInString(plain[searchFrom+idx:])
		searchFrom += idx + size
		runesBefore++
	}
	return -1, found
}
