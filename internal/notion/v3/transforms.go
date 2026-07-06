package v3

// Transform v3 RecordMap entities into the normalized notion types.
// Handles rich-text flattening, property ID→name resolution, timestamp
// conversion, and parent-format normalization.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shhac/agent-notion/internal/notion"
)

// --- Timestamps ---

// msToISO renders unix milliseconds as a JS Date.toISOString()-style string
// (UTC, millisecond precision, literal Z). Zero is treated as absent.
func msToISO(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format("2006-01-02T15:04:05.000Z")
}

// --- Notion URL ---

func notionURL(id string) string {
	return "https://www.notion.so/" + strings.ReplaceAll(id, "-", "")
}

// resolveID prefers an explicit collection-view-page ID, falling back to the
// collection's parent ID (matching the TS `viewPageId ?? parent_id`).
func resolveID(viewPageID, parentID string) string {
	if viewPageID != "" {
		return viewPageID
	}
	return parentID
}

// --- Parent ---

// ParentFor converts a v3 parent_table/parent_id pair into a normalized
// ParentRef, or nil for unrecognized tables.
func ParentFor(parentTable, parentID string) *notion.ParentRef {
	switch parentTable {
	case "collection":
		return &notion.ParentRef{Type: "database", ID: parentID}
	case "block":
		return &notion.ParentRef{Type: "page", ID: parentID}
	case "space":
		return &notion.ParentRef{Type: "workspace", ID: parentID}
	default:
		return nil
	}
}

// --- Property value flattening ---

// FlattenPropertyValue flattens one v3 property value using its schema type.
// Returns Go values whose JSON encoding matches the TS flattener: strings,
// float64 numbers, []string, nil, mention/date/file maps, etc.
func FlattenPropertyValue(value RichText, schema PropertySchema) any {
	text := value.Plain()

	switch schema.Type {
	case "title", "text":
		return text
	case "number":
		if text == "" {
			return nil
		}
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil
		}
		return n
	case "multi_select":
		if text == "" {
			return []string{}
		}
		return strings.Split(text, ",")
	case "date":
		return flattenDate(value, text)
	case "person", "people":
		return flattenMentions(value, "u")
	case "relation":
		return flattenMentions(value, "p")
	case "checkbox":
		return text == "Yes"
	case "created_by", "last_edited_by":
		if text == "" {
			return nil
		}
		return map[string]string{"id": text}
	case "files":
		if text == "" {
			return []map[string]any{}
		}
		return []map[string]any{{"name": text, "url": nil}}
	default:
		// select, status, url, email, phone_number, created_time,
		// last_edited_time, formula, rollup, unique_id, and unknown types all
		// coerce to text-or-null.
		return textOrNil(text)
	}
}

func textOrNil(text string) any {
	if text == "" {
		return nil
	}
	return text
}

// flattenDate extracts start/end from a v3 date decoration (["d", {…}]),
// falling back to the plain text as a start-only date.
func flattenDate(value RichText, text string) any {
	if len(value) == 0 {
		return nil
	}
	for _, seg := range value {
		for _, dec := range seg.Decorations {
			if dec.Type != "d" {
				continue
			}
			if obj := dec.ObjectArg(0); obj != nil {
				return map[string]any{"start": obj["start_date"], "end": obj["end_date"]}
			}
		}
	}
	if text != "" {
		return map[string]any{"start": text, "end": nil}
	}
	return nil
}

// flattenMentions collects mention IDs of the given decoration type ("u" for
// people, "p" for relations) into [{id}] maps.
func flattenMentions(value RichText, decType string) []map[string]string {
	result := []map[string]string{}
	for _, seg := range value {
		for _, dec := range seg.Decorations {
			if dec.Type != decType {
				continue
			}
			if id := dec.StringArg(0); id != "" {
				result = append(result, map[string]string{"id": id})
			}
		}
	}
	return result
}

// FlattenProperties resolves a block's schema-ID-keyed properties to
// human-readable names. Schema keys are iterated in sorted order for
// deterministic output (Go maps are unordered).
func FlattenProperties(properties map[string]RichText, schema map[string]PropertySchema) map[string]any {
	result := make(map[string]any)
	if properties == nil {
		return result
	}
	for _, propID := range sortedSchemaKeys(schema) {
		ps := schema[propID]
		result[ps.Name] = FlattenPropertyValue(properties[propID], ps)
	}
	return result
}

// --- Block normalization ---

// blockTypeMap maps v3 block types to the official API's block type names.
var blockTypeMap = map[string]string{
	"text":                 "paragraph",
	"header":               "heading_1",
	"sub_header":           "heading_2",
	"sub_sub_header":       "heading_3",
	"bulleted_list":        "bulleted_list_item",
	"numbered_list":        "numbered_list_item",
	"to_do":                "to_do",
	"toggle":               "toggle",
	"code":                 "code",
	"quote":                "quote",
	"callout":              "callout",
	"divider":              "divider",
	"image":                "image",
	"bookmark":             "bookmark",
	"equation":             "equation",
	"page":                 "child_page",
	"collection_view_page": "child_database",
	"collection_view":      "child_database",
	"table_of_contents":    "table_of_contents",
	"breadcrumb":           "breadcrumb",
	"column_list":          "column_list",
	"column":               "column",
	"synced_block":         "synced_block",
	"link_preview":         "link_preview",
	"embed":                "embed",
	"video":                "video",
	"pdf":                  "pdf",
	"audio":                "audio",
	"file":                 "file",
}

// NormalizeBlock converts a v3 block into a NormalizedBlock in the official
// API's type vocabulary, flattening type-specific extras alongside.
func NormalizeBlock(b *Block) notion.NormalizedBlock {
	typ := b.Type
	if mapped, ok := blockTypeMap[b.Type]; ok {
		typ = mapped
	}
	titleText := b.Property("title").Plain()

	nb := notion.NormalizedBlock{
		ID:          b.ID,
		Type:        typ,
		RichText:    titleText,
		HasChildren: len(b.Content) > 0,
	}

	switch b.Type {
	case "to_do":
		checked := b.Property("checked").Plain() == "Yes"
		nb.Checked = &checked
	case "code":
		nb.Language = b.Property("language").Plain()
	case "image":
		url := formatString(b.Format, "display_source")
		if url == "" {
			url = b.Property("source").Plain()
		}
		nb.URL = url
		nb.Caption = b.Property("caption").Plain()
	case "bookmark":
		nb.URL = b.Property("link").Plain()
		nb.Caption = b.Property("description").Plain()
	case "equation":
		nb.Expression = titleText
	case "page", "collection_view_page", "collection_view":
		nb.Title = titleText
	case "callout":
		nb.Emoji = formatString(b.Format, "page_icon")
	case "embed", "link_preview":
		nb.URL = b.Property("source").Plain()
	case "video", "pdf", "audio", "file":
		nb.URL = b.Property("source").Plain()
		nb.Caption = b.Property("caption").Plain()
		nb.Title = b.Property("title").Plain()
	}

	return nb
}

// formatString reads a string entry from a block's format map, "" when absent.
func formatString(f map[string]any, key string) string {
	if f == nil {
		return ""
	}
	s, _ := f[key].(string)
	return s
}

// --- High-level transforms ---

// ToSearchResult transforms a v3 block into a SearchResult.
func ToSearchResult(b *Block) notion.SearchResult {
	isCollection := b.Type == "collection_view_page" || b.Type == "collection_view"
	resultType := "page"
	if isCollection {
		resultType = "database"
	}
	return notion.SearchResult{
		ID:           b.ID,
		Type:         resultType,
		Title:        b.Property("title").Plain(),
		URL:          notionURL(b.ID),
		Parent:       ParentFor(b.ParentTable, b.ParentID),
		LastEditedAt: msToISO(b.LastEditedTime),
	}
}

// ToDatabaseListItem transforms a v3 collection into a DatabaseListItem. An
// empty collectionViewPageID falls back to the collection's parent ID.
func ToDatabaseListItem(c *Collection, collectionViewPageID string) notion.DatabaseListItem {
	id := resolveID(collectionViewPageID, c.ParentID)
	return notion.DatabaseListItem{
		ID:            id,
		Title:         c.Name.Plain(),
		URL:           notionURL(id),
		Parent:        ParentFor(c.ParentTable, c.ParentID),
		PropertyCount: len(c.Schema),
	}
}

// ToDatabaseDetail transforms a v3 collection into a DatabaseDetail.
func ToDatabaseDetail(c *Collection, collectionViewPageID string) notion.DatabaseDetail {
	id := resolveID(collectionViewPageID, c.ParentID)

	props := make(map[string]notion.PropertyDefinition, len(c.Schema))
	for _, propID := range sortedSchemaKeys(c.Schema) {
		ps := c.Schema[propID]
		def := notion.PropertyDefinition{ID: propID, Type: ps.Type}

		if len(ps.Options) > 0 {
			opts := make([]notion.PropertyOption, 0, len(ps.Options))
			for _, o := range ps.Options {
				opts = append(opts, notion.PropertyOption{Name: o.Value, Color: o.Color})
			}
			def.Options = opts
		}
		if len(ps.Groups) > 0 {
			groups := make([]notion.PropertyGroup, 0, len(ps.Groups))
			for _, g := range ps.Groups {
				groups = append(groups, notion.PropertyGroup{Name: g.Name, Options: optionValuesForGroup(ps, g)})
			}
			def.Groups = groups
		}
		if ps.CollectionID != "" {
			def.RelatedDatabase = ps.CollectionID
		}

		props[ps.Name] = def
	}

	return notion.DatabaseDetail{
		ID:          id,
		Title:       c.Name.Plain(),
		Description: c.Description.Plain(),
		URL:         notionURL(id),
		Parent:      ParentFor(c.ParentTable, c.ParentID),
		Properties:  props,
	}
}

// ToDatabaseSchema transforms a v3 collection into a DatabaseSchema. Properties
// are ordered by sorted schema key for deterministic output.
func ToDatabaseSchema(c *Collection, collectionViewPageID string) notion.DatabaseSchema {
	props := make([]notion.SchemaProperty, 0, len(c.Schema))
	for _, propID := range sortedSchemaKeys(c.Schema) {
		ps := c.Schema[propID]
		sp := notion.SchemaProperty{Name: ps.Name, ID: propID, Type: ps.Type}

		if len(ps.Options) > 0 {
			vals := make([]string, 0, len(ps.Options))
			for _, o := range ps.Options {
				vals = append(vals, o.Value)
			}
			sp.Options = vals
		}
		if len(ps.Groups) > 0 {
			groups := make(map[string][]string, len(ps.Groups))
			for _, g := range ps.Groups {
				groups[g.Name] = optionValuesForGroup(ps, g)
			}
			sp.Groups = groups
		}
		if ps.CollectionID != "" {
			sp.RelatedDatabase = ps.CollectionID
		}

		props = append(props, sp)
	}

	return notion.DatabaseSchema{
		ID:         resolveID(collectionViewPageID, c.ParentID),
		Title:      c.Name.Plain(),
		Properties: props,
	}
}

// optionValuesForGroup returns the option values referenced by a group's
// optionIds, preserving the schema's option order.
func optionValuesForGroup(ps PropertySchema, g PropertySchemaGroup) []string {
	values := []string{}
	if len(g.OptionIDs) == 0 {
		return values
	}
	wanted := make(map[string]bool, len(g.OptionIDs))
	for _, id := range g.OptionIDs {
		wanted[id] = true
	}
	for _, o := range ps.Options {
		if wanted[o.ID] {
			values = append(values, o.Value)
		}
	}
	return values
}

// ToQueryRow transforms a v3 page row + collection schema into a QueryRow.
func ToQueryRow(b *Block, schema map[string]PropertySchema) notion.QueryRow {
	return notion.QueryRow{
		ID:           b.ID,
		URL:          notionURL(b.ID),
		Properties:   FlattenProperties(b.Properties, schema),
		CreatedAt:    msToISO(b.CreatedTime),
		LastEditedAt: msToISO(b.LastEditedTime),
	}
}

// ToPageDetail transforms a v3 page block into a PageDetail. A nil schema
// yields a title-only properties map.
func ToPageDetail(b *Block, schema map[string]PropertySchema) notion.PageDetail {
	var props map[string]any
	if schema != nil {
		props = FlattenProperties(b.Properties, schema)
	} else {
		props = map[string]any{"title": b.Property("title").Plain()}
	}

	var icon *notion.Icon
	if emoji := formatString(b.Format, "page_icon"); emoji != "" {
		icon = &notion.Icon{Type: "emoji", Emoji: emoji}
	}

	return notion.PageDetail{
		ID:           b.ID,
		URL:          notionURL(b.ID),
		Parent:       ParentFor(b.ParentTable, b.ParentID),
		Properties:   props,
		Icon:         icon,
		CreatedAt:    msToISO(b.CreatedTime),
		LastEditedAt: msToISO(b.LastEditedTime),
		Archived:     !b.IsAlive(),
	}
}

// ToUserItem transforms a v3 user into a UserItem.
func ToUserItem(u *User) notion.UserItem {
	return notion.UserItem{
		ID:        u.ID,
		Name:      fullName(u),
		Type:      "person",
		Email:     u.Email,
		AvatarURL: u.ProfilePhoto,
	}
}

// ToUserMe transforms a v3 user (plus optional workspace name) into a UserMe.
func ToUserMe(u *User, spaceName string) notion.UserMe {
	return notion.UserMe{
		ID:            u.ID,
		Name:          fullName(u),
		Type:          "person",
		WorkspaceName: spaceName,
	}
}

// fullName joins a user's given and family names, skipping empty parts.
func fullName(u *User) string {
	parts := make([]string, 0, 2)
	if u.GivenName != "" {
		parts = append(parts, u.GivenName)
	}
	if u.FamilyName != "" {
		parts = append(parts, u.FamilyName)
	}
	return strings.Join(parts, " ")
}

// --- Rich text decoration injection ---

// AddDecorationToRange applies a decoration to a character range within a
// RichText, splitting segments at range boundaries and preserving existing
// decorations. Offsets are rune indices; the TS original used UTF-16 code
// units, which differ only for text outside the BMP.
func AddDecorationToRange(rt RichText, start, end int, d Decoration) (RichText, error) {
	if start < 0 || end <= start {
		return nil, fmt.Errorf("invalid range: [%d, %d)", start, end)
	}

	result := RichText{}
	offset := 0

	for _, seg := range rt {
		runes := []rune(seg.Text)
		segStart := offset
		segEnd := offset + len(runes)

		// No overlap with the target range — pass through unchanged.
		if segEnd <= start || segStart >= end {
			result = append(result, seg)
			offset = segEnd
			continue
		}

		overlapStart := max(segStart, start)
		overlapEnd := min(segEnd, end)

		// Before part keeps the original decorations, no new decoration.
		if overlapStart > segStart {
			before := string(runes[:overlapStart-segStart])
			result = append(result, Segment{Text: before, Decorations: cloneDecorations(seg.Decorations)})
		}

		// Overlap part adds the new decoration alongside any existing ones.
		overlap := string(runes[overlapStart-segStart : overlapEnd-segStart])
		result = append(result, Segment{Text: overlap, Decorations: append(cloneDecorations(seg.Decorations), d)})

		// After part keeps the original decorations, no new decoration.
		if overlapEnd < segEnd {
			after := string(runes[overlapEnd-segStart:])
			result = append(result, Segment{Text: after, Decorations: cloneDecorations(seg.Decorations)})
		}

		offset = segEnd
	}

	return result, nil
}

func cloneDecorations(decs []Decoration) []Decoration {
	if len(decs) == 0 {
		return nil
	}
	out := make([]Decoration, len(decs))
	copy(out, decs)
	return out
}

// --- Reverse transforms (write direction) ---

// BuildPropertyValue converts a property value to v3 rich text based on its
// schema type. Complex types that require v3 decoration formats return an
// error with an LLM-facing message rather than silently mis-encoding.
func BuildPropertyValue(value any, schemaType string) (RichText, error) {
	switch schemaType {
	case "title", "text", "url", "email", "phone_number", "select", "status":
		return NewRichText(jsStringOr(value, "")), nil
	case "number":
		return NewRichText(jsStringOr(value, "0")), nil
	case "multi_select":
		if arr, ok := toStringSlice(value); ok {
			return NewRichText(strings.Join(arr, ",")), nil
		}
		return NewRichText(jsStringOr(value, "")), nil
	case "checkbox":
		if isTruthy(value) {
			return NewRichText("Yes"), nil
		}
		return NewRichText("No"), nil
	case "date":
		return nil, fmt.Errorf(`Property type "date" requires v3 decoration format (‣ with "d" decoration). ` +
			`Use the official API backend for date properties, or pass pre-formatted v3 rich text.`)
	case "relation":
		return nil, fmt.Errorf(`Property type "relation" requires v3 decoration format (‣ with "p" decoration). ` +
			`Use the official API backend for relation properties, or pass pre-formatted v3 rich text.`)
	case "person", "people":
		return nil, fmt.Errorf(`Property type %q requires v3 decoration format (‣ with "u" decoration). `+
			`Use the official API backend for people properties, or pass pre-formatted v3 rich text.`, schemaType)
	case "files":
		return nil, fmt.Errorf(`Property type "files" requires complex v3 format. ` +
			`Use the official API backend for file properties.`)
	default:
		return NewRichText(jsStringOr(value, "")), nil
	}
}

// MapPropertiesToSchema maps human-readable property names to schema-ID-keyed
// v3 property values. The title property is skipped and names with no matching
// schema entry are dropped. Property keys and schema lookups are iterated in
// sorted order for deterministic output.
func MapPropertiesToSchema(properties map[string]any, schema map[string]PropertySchema) (map[string]RichText, error) {
	mapped := make(map[string]RichText)
	for _, name := range sortedAnyKeys(properties) {
		if name == "Name" || name == "title" {
			continue
		}
		schemaKey, entry, ok := findSchemaByName(schema, name)
		if !ok {
			continue
		}
		rt, err := BuildPropertyValue(properties[name], entry.Type)
		if err != nil {
			return nil, err
		}
		mapped[schemaKey] = rt
	}
	return mapped, nil
}

// findSchemaByName returns the schema key + entry whose Name matches, checking
// keys in sorted order for deterministic resolution.
func findSchemaByName(schema map[string]PropertySchema, name string) (string, PropertySchema, bool) {
	for _, key := range sortedSchemaKeys(schema) {
		if schema[key].Name == name {
			return key, schema[key], true
		}
	}
	return "", PropertySchema{}, false
}

// --- Comment transforms ---

// ToCommentItem transforms a v3 comment (with optional author user and anchor
// text) into a CommentItem. An empty anchorText is treated as absent.
func ToCommentItem(c *Comment, u *User, anchorText string) notion.CommentItem {
	item := notion.CommentItem{
		ID:        c.ID,
		Body:      c.Text.Plain(),
		Author:    commentAuthor(c, u),
		CreatedAt: msToISO(c.CreatedTime),
	}
	if anchorText != "" {
		item.AnchorText = anchorText
	}
	return item
}

func commentAuthor(c *Comment, u *User) *notion.UserRef {
	if u != nil {
		return &notion.UserRef{ID: u.ID, Name: fullName(u)}
	}
	if c.CreatedByID != "" {
		return &notion.UserRef{ID: c.CreatedByID}
	}
	return nil
}

// ExtractAnchorText concatenates the characters marked with the given
// discussion ID (["m", discussionID]), or "" when none are found.
func ExtractAnchorText(rt RichText, discussionID string) string {
	var anchor strings.Builder
	for _, seg := range rt {
		for _, dec := range seg.Decorations {
			if dec.Type == "m" && dec.StringArg(0) == discussionID {
				anchor.WriteString(seg.Text)
				break
			}
		}
	}
	return anchor.String()
}

// --- Sorted-key helpers ---

func sortedSchemaKeys(schema map[string]PropertySchema) []string {
	keys := make([]string, 0, len(schema))
	for k := range schema {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedAnyKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --- JS-style value coercion (write direction) ---

// jsStringOr mimics TS `String(value ?? def)`: nil yields def, otherwise the
// value is stringified JS-style.
func jsStringOr(value any, def string) string {
	if value == nil {
		return def
	}
	return jsString(value)
}

func jsString(value any) string {
	switch x := value.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func toStringSlice(value any) ([]string, bool) {
	switch arr := value.(type) {
	case []string:
		return arr, true
	case []any:
		out := make([]string, len(arr))
		for i, v := range arr {
			out[i] = jsString(v)
		}
		return out, true
	default:
		return nil, false
	}
}

// isTruthy mirrors JS truthiness for checkbox coercion.
func isTruthy(value any) bool {
	switch x := value.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case float64:
		return x != 0
	case int:
		return x != 0
	default:
		return true
	}
}
