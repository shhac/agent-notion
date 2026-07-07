// V3 operation builders for saveTransactions: the pointer + path + command +
// args shapes the Notion v3 internal API expects. JSON field names here are
// API wire format — do not rename.

package v3

import (
	"sort"
	"strings"
	"time"
)

// Pointer addresses a record in a saveTransactions operation.
type Pointer struct {
	Table   string `json:"table"`
	ID      string `json:"id"`
	SpaceID string `json:"spaceId"`
}

// Operation is one saveTransactions command.
type Operation struct {
	Pointer Pointer  `json:"pointer"`
	Path    []string `json:"path"`
	Command string   `json:"command"`
	Args    any      `json:"args"`
}

func ptr(table, id, spaceID string) Pointer { return Pointer{Table: table, ID: id, SpaceID: spaceID} }

func blockPtr(id, spaceID string) Pointer { return ptr("block", id, spaceID) }

// setOp sets a value at a specific path (or the entire record when path is
// empty).
func setOp(pointer Pointer, path []string, args any) Operation {
	if path == nil {
		path = []string{}
	}
	return Operation{Pointer: pointer, Path: path, Command: "set", Args: args}
}

// updateOp shallow-merges args into the record root.
func updateOp(pointer Pointer, args any) Operation {
	return Operation{Pointer: pointer, Path: []string{}, Command: "update", Args: args}
}

// listAfterOp appends a child ID to a list (after a specific sibling when
// afterID is set).
func listAfterOp(pointer Pointer, listPath, childID, afterID string) Operation {
	args := map[string]string{"id": childID}
	if afterID != "" {
		args["after"] = afterID
	}
	return Operation{Pointer: pointer, Path: []string{listPath}, Command: "listAfter", Args: args}
}

// listBeforeOp prepends a child ID to a list (before a specific sibling when
// beforeID is set).
func listBeforeOp(pointer Pointer, listPath, childID, beforeID string) Operation {
	args := map[string]string{"id": childID}
	if beforeID != "" {
		args["before"] = beforeID
	}
	return Operation{Pointer: pointer, Path: []string{listPath}, Command: "listBefore", Args: args}
}

// listRemoveOp removes a child ID from a list.
func listRemoveOp(pointer Pointer, listPath, childID string) Operation {
	return Operation{Pointer: pointer, Path: []string{listPath}, Command: "listRemove", Args: map[string]string{"id": childID}}
}

// editMetaOp updates a record's last-edited metadata.
func editMetaOp(pointer Pointer, userID string, now time.Time) Operation {
	return updateOp(pointer, map[string]any{
		"last_edited_time":     now.UnixMilli(),
		"last_edited_by_table": "notion_user",
		"last_edited_by_id":    userID,
	})
}

// CreateBlockParams describes a block to create. The optional fields express
// caller intent directly so no caller ever patches the emitted ops after the
// fact — the builder is the single owner of the op shapes.
type CreateBlockParams struct {
	ID          string
	Type        string
	ParentID    string
	ParentTable string
	SpaceID     string
	UserID      string
	Properties  map[string]any
	Format      map[string]any
	// ListParent overrides the pointer that receives the content listAfter
	// and the parent editMeta — e.g. a database page's collection_view_page
	// block, while the block args still name the collection as parent.
	ListParent *Pointer
	// AfterID inserts the new block after a specific sibling.
	AfterID string
	// SkipParentEditMeta omits the parent editMeta op, for callers batching
	// many creates that add one trailing editMeta themselves.
	SkipParentEditMeta bool
}

// CreateBlockOps builds the operations that create a new block and add it to
// its parent's content list.
func CreateBlockOps(p CreateBlockParams, now time.Time) []Operation {
	bp := blockPtr(p.ID, p.SpaceID)
	parentTable := p.ParentTable
	if parentTable == "collection" {
		parentTable = "block"
	}
	pp := ptr(parentTable, p.ParentID, p.SpaceID)
	if p.ListParent != nil {
		pp = *p.ListParent
	}

	blockArgs := map[string]any{
		"type":                 p.Type,
		"id":                   p.ID,
		"version":              0,
		"created_time":         now.UnixMilli(),
		"last_edited_time":     now.UnixMilli(),
		"parent_id":            p.ParentID,
		"parent_table":         p.ParentTable,
		"alive":                true,
		"created_by_table":     "notion_user",
		"created_by_id":        p.UserID,
		"last_edited_by_table": "notion_user",
		"last_edited_by_id":    p.UserID,
		"space_id":             p.SpaceID,
	}
	if p.Properties != nil {
		blockArgs["properties"] = p.Properties
	}
	if p.Format != nil {
		blockArgs["format"] = p.Format
	}

	ops := []Operation{
		setOp(bp, nil, blockArgs),
		listAfterOp(pp, "content", p.ID, p.AfterID),
	}
	if !p.SkipParentEditMeta {
		ops = append(ops, editMetaOp(pp, p.UserID, now))
	}
	return ops
}

// TrashBlockParams identifies a block to move to Trash.
type TrashBlockParams struct {
	ID          string
	ParentID    string
	ParentTable string
	SpaceID     string
	UserID      string
}

// TrashBlockOps builds the operations that move a block to Trash (Notion's
// recoverable delete).
//
// Uses the legacy alive:false + listRemove shape, which the v3 server still
// accepts. The current desktop client uses a different two-step flow
// (saveTransactionsFanout removeChild + REST deleteContentRecords). See
// design-docs/notion-page-lifecycle-har.md for the modern shape if we ever
// need to migrate.
func TrashBlockOps(p TrashBlockParams, now time.Time) []Operation {
	bp := blockPtr(p.ID, p.SpaceID)
	pp := ptr(p.ParentTable, p.ParentID, p.SpaceID)

	return []Operation{
		updateOp(bp, map[string]any{
			"alive":                false,
			"last_edited_time":     now.UnixMilli(),
			"last_edited_by_table": "notion_user",
			"last_edited_by_id":    p.UserID,
		}),
		listRemoveOp(pp, "content", p.ID),
		editMetaOp(pp, p.UserID, now),
	}
}

// ArchivePageParams identifies a page to (un)archive.
type ArchivePageParams struct {
	ID      string
	SpaceID string
	UserID  string
	Archive bool
}

// ArchivePageOps builds the operations that set or clear the real Archive
// state on a page. Archive is distinct from Trash: it keeps the page alive
// but hides it from search and shows an "archived by" banner.
func ArchivePageOps(p ArchivePageParams, now time.Time) []Operation {
	bp := blockPtr(p.ID, p.SpaceID)
	var args map[string]any
	if p.Archive {
		args = map[string]any{
			"archived_by_id":    p.UserID,
			"archived_by_table": "notion_user",
			"archived_time":     now.UnixMilli(),
		}
	} else {
		// Explicit nulls — the server clears the fields only when they are
		// present in args.
		args = map[string]any{
			"archived_by_id":    nil,
			"archived_by_table": nil,
			"archived_time":     nil,
		}
	}
	return []Operation{updateOp(bp, args), editMetaOp(bp, p.UserID, now)}
}

// MoveBlockParams describes a block move within or across parents.
type MoveBlockParams struct {
	ID             string
	OldParentID    string
	OldParentTable string
	NewParentID    string
	NewParentTable string
	SpaceID        string
	UserID         string
	AfterID        string
}

// MoveBlockOps builds the operations that move a block within or across
// parents. With AfterID empty the block is prepended to the new parent.
func MoveBlockOps(p MoveBlockParams, now time.Time) []Operation {
	bp := blockPtr(p.ID, p.SpaceID)
	oldPP := ptr(p.OldParentTable, p.OldParentID, p.SpaceID)
	newPP := ptr(p.NewParentTable, p.NewParentID, p.SpaceID)

	insert := listBeforeOp(newPP, "content", p.ID, "")
	if p.AfterID != "" {
		insert = listAfterOp(newPP, "content", p.ID, p.AfterID)
	}

	ops := []Operation{
		listRemoveOp(oldPP, "content", p.ID),
		insert,
	}

	if p.OldParentID != p.NewParentID {
		ops = append(ops,
			updateOp(bp, map[string]any{
				"parent_id":            p.NewParentID,
				"parent_table":         p.NewParentTable,
				"last_edited_time":     now.UnixMilli(),
				"last_edited_by_table": "notion_user",
				"last_edited_by_id":    p.UserID,
			}),
			editMetaOp(oldPP, p.UserID, now),
		)
	}

	return append(ops, editMetaOp(newPP, p.UserID, now))
}

// UpdatePropertyParams describes property/format updates on a block.
type UpdatePropertyParams struct {
	ID         string
	SpaceID    string
	UserID     string
	Properties map[string]any
	Format     map[string]any
}

// UpdatePropertyOps builds path-based set operations for each property and
// format key (in sorted key order — the TS relied on object insertion order).
func UpdatePropertyOps(p UpdatePropertyParams, now time.Time) []Operation {
	bp := blockPtr(p.ID, p.SpaceID)
	var ops []Operation

	for _, key := range sortedKeys(p.Properties) {
		ops = append(ops, setOp(bp, []string{"properties", key}, p.Properties[key]))
	}
	for _, key := range sortedKeys(p.Format) {
		ops = append(ops, setOp(bp, []string{"format", key}, p.Format[key]))
	}

	return append(ops, editMetaOp(bp, p.UserID, now))
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// CreateCommentParams describes a new page-level discussion + comment.
type CreateCommentParams struct {
	DiscussionID string
	CommentID    string
	PageID       string
	SpaceID      string
	UserID       string
	Text         string
}

// CreateCommentOps builds the operations that create a new discussion and its
// first comment on a page block.
func CreateCommentOps(p CreateCommentParams, now time.Time) []Operation {
	dp := ptr("discussion", p.DiscussionID, p.SpaceID)
	cp := ptr("comment", p.CommentID, p.SpaceID)
	bp := blockPtr(p.PageID, p.SpaceID)

	return []Operation{
		setOp(dp, nil, map[string]any{
			"id":           p.DiscussionID,
			"version":      0,
			"parent_id":    p.PageID,
			"parent_table": "block",
			"resolved":     false,
			"comments":     []string{},
			"space_id":     p.SpaceID,
			"alive":        true,
		}),
		listAfterOp(bp, "discussions", p.DiscussionID, ""),
		setOp(cp, nil, map[string]any{
			"id":               p.CommentID,
			"version":          0,
			"parent_id":        p.DiscussionID,
			"parent_table":     "discussion",
			"text":             NewRichText(p.Text),
			"created_by_table": "notion_user",
			"created_by_id":    p.UserID,
			"alive":            true,
			"space_id":         p.SpaceID,
		}),
		listAfterOp(dp, "comments", p.CommentID, ""),
		setOp(cp, []string{"created_time"}, now.UnixMilli()),
		setOp(cp, []string{"last_edited_time"}, now.UnixMilli()),
	}
}

// CreateInlineCommentParams describes a new discussion anchored to text
// within a block. UpdatedTitle is the block's rich text with the
// ["m", discussionID] decoration already injected (see AddDecorationToRange).
type CreateInlineCommentParams struct {
	DiscussionID string
	CommentID    string
	BlockID      string
	SpaceID      string
	UserID       string
	Text         string
	UpdatedTitle RichText
}

// extractContextRichText returns only the segments (with all their
// decorations) carrying the given discussion marker.
func extractContextRichText(rt RichText, discussionID string) RichText {
	context := RichText{}
	for _, segment := range rt {
		for _, d := range segment.Decorations {
			if d.Type == "m" && d.StringArg(0) == discussionID {
				context = append(context, segment)
				break
			}
		}
	}
	return context
}

// CreateInlineCommentOps builds the operations that create an inline comment
// anchored to specific text within a block.
func CreateInlineCommentOps(p CreateInlineCommentParams, now time.Time) []Operation {
	dp := ptr("discussion", p.DiscussionID, p.SpaceID)
	cp := ptr("comment", p.CommentID, p.SpaceID)
	bp := blockPtr(p.BlockID, p.SpaceID)

	return []Operation{
		setOp(dp, nil, map[string]any{
			"id":           p.DiscussionID,
			"version":      0,
			"parent_id":    p.BlockID,
			"parent_table": "block",
			"resolved":     false,
			"comments":     []string{},
			"context":      extractContextRichText(p.UpdatedTitle, p.DiscussionID),
			"type":         "default",
			"space_id":     p.SpaceID,
			"alive":        true,
		}),
		listAfterOp(bp, "discussions", p.DiscussionID, ""),
		setOp(cp, nil, map[string]any{
			"id":               p.CommentID,
			"version":          0,
			"parent_id":        p.DiscussionID,
			"parent_table":     "discussion",
			"text":             NewRichText(p.Text),
			"created_by_table": "notion_user",
			"created_by_id":    p.UserID,
			"alive":            true,
			"space_id":         p.SpaceID,
		}),
		listAfterOp(dp, "comments", p.CommentID, ""),
		setOp(cp, []string{"created_time"}, now.UnixMilli()),
		setOp(cp, []string{"last_edited_time"}, now.UnixMilli()),
		setOp(bp, []string{"properties", "title"}, p.UpdatedTitle),
		editMetaOp(bp, p.UserID, now),
	}
}

// officialToV3Type maps official API block types to v3 types.
var officialToV3Type = map[string]string{
	"paragraph":          "text",
	"heading_1":          "header",
	"heading_2":          "sub_header",
	"heading_3":          "sub_sub_header",
	"bulleted_list_item": "bulleted_list",
	"numbered_list_item": "numbered_list",
	"to_do":              "to_do",
	"toggle":             "toggle",
	"code":               "code",
	"quote":              "quote",
	"callout":            "callout",
	"divider":            "divider",
	"image":              "image",
	"bookmark":           "bookmark",
	"equation":           "equation",
	"embed":              "embed",
	"video":              "video",
	"pdf":                "pdf",
	"audio":              "audio",
	"file":               "file",
}

// BlockArgs is the v3 creation shape derived from an official API block.
type BlockArgs struct {
	Type       string
	Properties map[string]any
	Format     map[string]any
}

// OfficialBlockToV3Args converts an official API block object (as produced by
// the markdown → blocks conversion) into v3 block creation args.
func OfficialBlockToV3Args(block map[string]any) BlockArgs {
	officialType, _ := block["type"].(string)
	v3Type := officialType
	if mapped, ok := officialToV3Type[officialType]; ok {
		v3Type = mapped
	}

	typeData, _ := block[officialType].(map[string]any)
	if typeData == nil {
		return BlockArgs{Type: v3Type}
	}

	properties := map[string]any{}
	format := map[string]any{}

	if richText, ok := typeData["rich_text"].([]any); ok && len(richText) > 0 {
		var text strings.Builder
		for _, rt := range richText {
			if m, ok := rt.(map[string]any); ok {
				if inner, ok := m["text"].(map[string]any); ok {
					if content, ok := inner["content"].(string); ok {
						text.WriteString(content)
					}
				}
			}
		}
		properties["title"] = NewRichText(text.String())
	}

	switch officialType {
	case "code":
		if lang, ok := typeData["language"].(string); ok && lang != "" {
			properties["language"] = NewRichText(lang)
		}
	case "to_do":
		if checked, ok := typeData["checked"].(bool); ok && checked {
			properties["checked"] = NewRichText("Yes")
		}
	case "image", "video", "pdf", "audio", "file", "embed":
		url, _ := typeData["url"].(string)
		if url == "" {
			if external, ok := typeData["external"].(map[string]any); ok {
				url, _ = external["url"].(string)
			}
		}
		if url != "" {
			properties["source"] = NewRichText(url)
		}
	case "bookmark":
		if url, ok := typeData["url"].(string); ok && url != "" {
			properties["link"] = NewRichText(url)
		}
	case "equation":
		if expression, ok := typeData["expression"].(string); ok && expression != "" {
			properties["title"] = NewRichText(expression)
		}
	case "callout":
		if icon, ok := typeData["icon"].(map[string]any); ok {
			if emoji, ok := icon["emoji"].(string); ok && emoji != "" {
				format["page_icon"] = emoji
			}
		}
	}

	args := BlockArgs{Type: v3Type}
	if len(properties) > 0 {
		args.Properties = properties
	}
	if len(format) > 0 {
		args.Format = format
	}
	return args
}
