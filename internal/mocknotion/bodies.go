package mocknotion

// Canonical response-body builders so tests compose recordMap fixtures
// without hand-writing the wrapper nesting.

// Entry wraps an entity in the normalized v3 recordMap entry shape.
func Entry(entity any) map[string]any {
	return map[string]any{"value": entity, "role": "reader"}
}

// RoleWrappedEntry wraps an entity in the new (spaceId + role-wrapped) v3
// wire shape, for exercising the client's normalize-at-boundary path.
func RoleWrappedEntry(entity any, spaceID string) map[string]any {
	return map[string]any{
		"spaceId": spaceID,
		"value":   map[string]any{"value": entity, "role": "reader"},
	}
}

// Table builds one recordMap table from id → entity, Entry-wrapping each.
func Table(entities map[string]any) map[string]any {
	table := make(map[string]any, len(entities))
	for id, entity := range entities {
		table[id] = Entry(entity)
	}
	return table
}

// RecordMapBody builds a {"recordMap": {...}} response body from
// table → (id → entity), Entry-wrapping every entity.
func RecordMapBody(tables map[string]map[string]any) map[string]any {
	rm := make(map[string]any, len(tables))
	for name, entities := range tables {
		rm[name] = Table(entities)
	}
	return map[string]any{"recordMap": rm}
}

// PageChunkBody is RecordMapBody plus the loadPageChunk cursor envelope.
func PageChunkBody(tables map[string]map[string]any) map[string]any {
	body := RecordMapBody(tables)
	body["cursor"] = map[string]any{"stack": []any{}}
	return body
}

// BlockEntity builds a minimal alive v3 block entity. Extra fields land via
// the overrides map (nil is fine).
func BlockEntity(id, blockType string, overrides map[string]any) map[string]any {
	entity := map[string]any{
		"id":               id,
		"type":             blockType,
		"version":          1,
		"alive":            true,
		"created_time":     1700000000000,
		"last_edited_time": 1700000001000,
		"parent_id":        "parent-1",
		"parent_table":     "space",
		"space_id":         "space-1",
	}
	for k, v := range overrides {
		entity[k] = v
	}
	return entity
}
