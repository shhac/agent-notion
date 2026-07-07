// Transforms mapping official REST responses (decoded as map[string]any, the
// Go analogue of the TS Record<string, unknown>) into the normalized types in
// package notion. Ported from the retired TS reference (see git history).

package official

import (
	"strconv"
	"strings"

	"github.com/shhac/agent-notion/internal/notion"
)

// --- map accessors ---

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func mstr(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func mmap(m map[string]any, key string) map[string]any { return asMap(m[key]) }

func mslice(m map[string]any, key string) []any { return asSlice(m[key]) }

func mbool(m map[string]any, key string) bool {
	b, _ := m[key].(bool)
	return b
}

// strOrNil returns the string at key, or nil when absent/non-string — the
// analogue of the TS `x ?? null` on an optional string.
func strOrNil(m map[string]any, key string) any {
	if s, ok := m[key].(string); ok {
		return s
	}
	return nil
}

// richTextToPlain concatenates plain_text across rich text runs.
func richTextToPlain(items []any) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	for _, it := range items {
		b.WriteString(mstr(asMap(it), "plain_text"))
	}
	return b.String()
}

// --- shared object formatters ---

// formatParent normalizes a parent reference, returning nil for unknown or
// missing parents.
func formatParent(parent map[string]any) *notion.ParentRef {
	if parent == nil {
		return nil
	}
	switch mstr(parent, "type") {
	case "database_id":
		return &notion.ParentRef{Type: "database", ID: mstr(parent, "database_id")}
	case "page_id":
		return &notion.ParentRef{Type: "page", ID: mstr(parent, "page_id")}
	case "workspace":
		return &notion.ParentRef{Type: "workspace"}
	case "block_id":
		return &notion.ParentRef{Type: "block", ID: mstr(parent, "block_id")}
	}
	return nil
}

// formatIcon normalizes a page icon (emoji or external image), nil when absent.
func formatIcon(icon map[string]any) *notion.Icon {
	if icon == nil {
		return nil
	}
	switch mstr(icon, "type") {
	case "emoji":
		return &notion.Icon{Type: "emoji", Emoji: mstr(icon, "emoji")}
	case "external":
		return &notion.Icon{Type: "external", URL: mstr(mmap(icon, "external"), "url")}
	}
	return nil
}

// formatUser normalizes a user reference, nil when absent.
func formatUser(user map[string]any) *notion.UserRef {
	if user == nil {
		return nil
	}
	return &notion.UserRef{ID: mstr(user, "id"), Name: mstr(user, "name")}
}

// mediaURL resolves a file/external block URL, preferring the hosted file.
func mediaURL(data map[string]any) string {
	if f := mmap(data, "file"); f != nil {
		if u, ok := f["url"].(string); ok {
			return u
		}
	}
	if e := mmap(data, "external"); e != nil {
		if u, ok := e["url"].(string); ok {
			return u
		}
	}
	return ""
}

// --- property flattening ---

// flattenPropertyValue reduces one Notion property object to a plain value,
// mirroring the TS type switch exactly.
func flattenPropertyValue(prop map[string]any) any {
	switch mstr(prop, "type") {
	case "title":
		return richTextToPlain(mslice(prop, "title"))
	case "rich_text":
		return richTextToPlain(mslice(prop, "rich_text"))
	case "number":
		return prop["number"]
	case "select":
		sel := mmap(prop, "select")
		if sel == nil {
			return nil
		}
		return strOrNil(sel, "name")
	case "multi_select":
		return optionNames(mslice(prop, "multi_select"))
	case "status":
		status := mmap(prop, "status")
		if status == nil {
			return nil
		}
		return strOrNil(status, "name")
	case "date":
		date := mmap(prop, "date")
		if date == nil {
			return nil
		}
		out := map[string]any{"end": nil}
		if s, ok := date["start"].(string); ok {
			out["start"] = s
		}
		if e, ok := date["end"].(string); ok {
			out["end"] = e
		}
		return out
	case "people":
		people := mslice(prop, "people")
		out := make([]any, 0, len(people))
		for _, p := range people {
			out = append(out, userRefValue(asMap(p)))
		}
		return out
	case "checkbox":
		return mbool(prop, "checkbox")
	case "url":
		return strOrNil(prop, "url")
	case "email":
		return strOrNil(prop, "email")
	case "phone_number":
		return strOrNil(prop, "phone_number")
	case "relation":
		rels := mslice(prop, "relation")
		out := make([]any, 0, len(rels))
		for _, r := range rels {
			out = append(out, map[string]any{"id": mstr(asMap(r), "id")})
		}
		return out
	case "rollup":
		rollup := mmap(prop, "rollup")
		if rollup == nil {
			return nil
		}
		if mstr(rollup, "type") == "array" {
			items := mslice(rollup, "array")
			out := make([]any, 0, len(items))
			for _, item := range items {
				out = append(out, flattenPropertyValue(asMap(item)))
			}
			return out
		}
		return flattenPropertyValue(rollup)
	case "formula":
		formula := mmap(prop, "formula")
		if formula == nil {
			return nil
		}
		return formula[mstr(formula, "type")]
	case "files":
		files := mslice(prop, "files")
		out := make([]any, 0, len(files))
		for _, f := range files {
			fm := asMap(f)
			var u any
			if fileObj := mmap(fm, "file"); fileObj != nil {
				if s, ok := fileObj["url"].(string); ok {
					u = s
				}
			}
			if u == nil {
				if ext := mmap(fm, "external"); ext != nil {
					if s, ok := ext["url"].(string); ok {
						u = s
					}
				}
			}
			out = append(out, map[string]any{"name": mstr(fm, "name"), "url": u})
		}
		return out
	case "created_time":
		return strOrNil(prop, "created_time")
	case "last_edited_time":
		return strOrNil(prop, "last_edited_time")
	case "created_by":
		return userRefOrNil(mmap(prop, "created_by"))
	case "last_edited_by":
		return userRefOrNil(mmap(prop, "last_edited_by"))
	case "unique_id":
		uid := mmap(prop, "unique_id")
		if uid == nil {
			return nil
		}
		num := formatNumber(uid["number"])
		if prefix := mstr(uid, "prefix"); prefix != "" {
			return prefix + "-" + num
		}
		return num
	case "verification":
		v := mmap(prop, "verification")
		if v == nil {
			return nil
		}
		return strOrNil(v, "state")
	default:
		return nil
	}
}

// optionNames extracts the "name" of each select option.
func optionNames(items []any) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, mstr(asMap(it), "name"))
	}
	return out
}

// userRefValue builds {id, name?} for a person, omitting name when absent.
func userRefValue(user map[string]any) map[string]any {
	m := map[string]any{"id": mstr(user, "id")}
	if n, ok := user["name"].(string); ok {
		m["name"] = n
	}
	return m
}

// userRefOrNil is userRefValue but nil when the user object is missing.
func userRefOrNil(user map[string]any) any {
	if user == nil {
		return nil
	}
	return userRefValue(user)
}

// formatNumber renders a JSON number the way JS String()/template literals do
// (no trailing zeros); empty string when the value is not a number.
func formatNumber(v any) string {
	f, ok := v.(float64)
	if !ok {
		return ""
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func flattenProperties(properties map[string]any) map[string]any {
	out := make(map[string]any, len(properties))
	for name, prop := range properties {
		out[name] = flattenPropertyValue(asMap(prop))
	}
	return out
}

// extractTitle returns the plain text of the first title-typed property.
func extractTitle(properties map[string]any) string {
	for _, name := range sortedKeys(properties) {
		prop := asMap(properties[name])
		if mstr(prop, "type") == "title" {
			return richTextToPlain(mslice(prop, "title"))
		}
	}
	return ""
}

// --- property schema ---

// optionsOf returns the select/multi-select/status options, or nil when the
// config has no options key.
func optionsOf(cfg map[string]any) []notion.PropertyOption {
	if _, ok := cfg["options"]; !ok {
		return nil
	}
	opts := asSlice(cfg["options"])
	out := make([]notion.PropertyOption, 0, len(opts))
	for _, o := range opts {
		om := asMap(o)
		out = append(out, notion.PropertyOption{Name: mstr(om, "name"), Color: mstr(om, "color")})
	}
	return out
}

// statusGroups resolves each status group to the option names it contains.
func statusGroups(cfg map[string]any) []notion.PropertyGroup {
	if _, ok := cfg["groups"]; !ok {
		return nil
	}
	groups := asSlice(cfg["groups"])
	options := asSlice(cfg["options"])
	out := make([]notion.PropertyGroup, 0, len(groups))
	for _, g := range groups {
		gm := asMap(g)
		ids := map[string]bool{}
		for _, idv := range asSlice(gm["option_ids"]) {
			if s, ok := idv.(string); ok {
				ids[s] = true
			}
		}
		names := []string{}
		for _, o := range options {
			om := asMap(o)
			if ids[mstr(om, "id")] {
				names = append(names, mstr(om, "name"))
			}
		}
		out = append(out, notion.PropertyGroup{Name: mstr(gm, "name"), Options: names})
	}
	return out
}

// buildPropertyDefinition normalizes one property's config into a
// PropertyDefinition.
func buildPropertyDefinition(prop map[string]any) notion.PropertyDefinition {
	def := notion.PropertyDefinition{ID: mstr(prop, "id"), Type: mstr(prop, "type")}

	switch def.Type {
	case "select":
		if cfg := mmap(prop, "select"); cfg != nil {
			def.Options = optionsOf(cfg)
		}
	case "multi_select":
		if cfg := mmap(prop, "multi_select"); cfg != nil {
			def.Options = optionsOf(cfg)
		}
	case "status":
		if cfg := mmap(prop, "status"); cfg != nil {
			def.Options = optionsOf(cfg)
			def.Groups = statusGroups(cfg)
		}
	case "unique_id":
		if cfg := mmap(prop, "unique_id"); cfg != nil {
			if prefix := mstr(cfg, "prefix"); prefix != "" {
				def.Prefix = prefix
			}
		}
	case "relation":
		if cfg := mmap(prop, "relation"); cfg != nil {
			if db := mstr(cfg, "database_id"); db != "" {
				def.RelatedDatabase = db
			}
		}
	}

	return def
}

// buildSchemaProperty flattens a property definition into a SchemaProperty.
func buildSchemaProperty(name string, prop map[string]any) notion.SchemaProperty {
	schema := notion.SchemaProperty{Name: name, ID: mstr(prop, "id"), Type: mstr(prop, "type")}
	def := buildPropertyDefinition(prop)

	if def.Options != nil {
		names := make([]string, len(def.Options))
		for i, o := range def.Options {
			names[i] = o.Name
		}
		schema.Options = names
	}
	if def.Groups != nil {
		schema.Groups = map[string][]string{}
		for _, g := range def.Groups {
			schema.Groups[g.Name] = g.Options
		}
	}
	if def.Prefix != "" {
		schema.Prefix = def.Prefix
	}
	if def.RelatedDatabase != "" {
		schema.RelatedDatabase = def.RelatedDatabase
	}
	return schema
}

// --- block normalization ---

// normalizeBlock reduces an official-API block to a NormalizedBlock, attaching
// type-specific extras.
func normalizeBlock(block map[string]any) notion.NormalizedBlock {
	typ := mstr(block, "type")
	data := mmap(block, typ)

	nb := notion.NormalizedBlock{
		ID:          mstr(block, "id"),
		Type:        typ,
		RichText:    richTextToPlain(mslice(data, "rich_text")),
		HasChildren: mbool(block, "has_children"),
	}

	switch typ {
	case "to_do":
		checked := mbool(data, "checked")
		nb.Checked = &checked
	case "code":
		nb.Language = mstr(data, "language")
	case "image":
		nb.URL = mediaURL(data)
		nb.Caption = richTextToPlain(mslice(data, "caption"))
	case "bookmark":
		nb.URL = mstr(data, "url")
		nb.Caption = richTextToPlain(mslice(data, "caption"))
	case "equation":
		nb.Expression = mstr(data, "expression")
	case "child_page", "child_database":
		nb.Title = mstr(data, "title")
	case "callout":
		nb.Emoji = mstr(mmap(data, "icon"), "emoji")
	case "link_preview", "embed":
		nb.URL = mstr(data, "url")
	case "video", "pdf", "audio", "file":
		nb.URL = mediaURL(data)
		nb.Caption = richTextToPlain(mslice(data, "caption"))
		nb.Title = mstr(data, "name")
	}

	return nb
}

// --- high-level transforms ---

func transformSearchResult(item map[string]any) notion.SearchResult {
	if mstr(item, "object") == "page" {
		return notion.SearchResult{
			ID:           mstr(item, "id"),
			Type:         "page",
			Title:        extractTitle(mmap(item, "properties")),
			URL:          mstr(item, "url"),
			Parent:       formatParent(mmap(item, "parent")),
			LastEditedAt: mstr(item, "last_edited_time"),
		}
	}
	return notion.SearchResult{
		ID:           mstr(item, "id"),
		Type:         "database",
		Title:        richTextToPlain(mslice(item, "title")),
		URL:          mstr(item, "url"),
		Parent:       formatParent(mmap(item, "parent")),
		LastEditedAt: mstr(item, "last_edited_time"),
	}
}

func transformDatabaseListItem(db map[string]any) notion.DatabaseListItem {
	return notion.DatabaseListItem{
		ID:            mstr(db, "id"),
		Title:         richTextToPlain(mslice(db, "title")),
		URL:           mstr(db, "url"),
		Parent:        formatParent(mmap(db, "parent")),
		PropertyCount: len(mmap(db, "properties")),
		LastEditedAt:  mstr(db, "last_edited_time"),
	}
}

func transformDatabaseDetail(db map[string]any) notion.DatabaseDetail {
	raw := mmap(db, "properties")
	properties := make(map[string]notion.PropertyDefinition, len(raw))
	for name, prop := range raw {
		properties[name] = buildPropertyDefinition(asMap(prop))
	}

	return notion.DatabaseDetail{
		ID:           mstr(db, "id"),
		Title:        richTextToPlain(mslice(db, "title")),
		Description:  richTextToPlain(mslice(db, "description")),
		URL:          mstr(db, "url"),
		Parent:       formatParent(mmap(db, "parent")),
		Properties:   properties,
		IsInline:     mbool(db, "is_inline"),
		CreatedAt:    mstr(db, "created_time"),
		LastEditedAt: mstr(db, "last_edited_time"),
	}
}

func transformDatabaseSchema(db map[string]any) notion.DatabaseSchema {
	raw := mmap(db, "properties")
	props := make([]notion.SchemaProperty, 0, len(raw))
	for _, name := range sortedKeys(raw) {
		props = append(props, buildSchemaProperty(name, asMap(raw[name])))
	}
	return notion.DatabaseSchema{
		ID:         mstr(db, "id"),
		Title:      richTextToPlain(mslice(db, "title")),
		Properties: props,
	}
}

func transformQueryRow(page map[string]any) notion.QueryRow {
	return notion.QueryRow{
		ID:           mstr(page, "id"),
		URL:          mstr(page, "url"),
		Properties:   flattenProperties(mmap(page, "properties")),
		CreatedAt:    mstr(page, "created_time"),
		LastEditedAt: mstr(page, "last_edited_time"),
	}
}

func transformPageDetail(page map[string]any) notion.PageDetail {
	return notion.PageDetail{
		ID:           mstr(page, "id"),
		URL:          mstr(page, "url"),
		Parent:       formatParent(mmap(page, "parent")),
		Properties:   flattenProperties(mmap(page, "properties")),
		Icon:         formatIcon(mmap(page, "icon")),
		CreatedAt:    mstr(page, "created_time"),
		CreatedBy:    formatUser(mmap(page, "created_by")),
		LastEditedAt: mstr(page, "last_edited_time"),
		LastEditedBy: formatUser(mmap(page, "last_edited_by")),
		Archived:     mbool(page, "archived"),
	}
}

func transformComment(comment map[string]any) notion.CommentItem {
	return notion.CommentItem{
		ID:        mstr(comment, "id"),
		Body:      richTextToPlain(mslice(comment, "rich_text")),
		Author:    formatUser(mmap(comment, "created_by")),
		CreatedAt: mstr(comment, "created_time"),
	}
}

func transformUser(user map[string]any) notion.UserItem {
	return notion.UserItem{
		ID:        mstr(user, "id"),
		Name:      mstr(user, "name"),
		Type:      mstr(user, "type"),
		Email:     mstr(mmap(user, "person"), "email"),
		AvatarURL: mstr(user, "avatar_url"),
	}
}
