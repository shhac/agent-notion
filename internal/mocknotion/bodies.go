package mocknotion

// Canonical response-body builders so tests compose recordMap fixtures
// without hand-writing the wrapper nesting.

// wireVersion mirrors the "__version__" metadata number the live v3 API now
// includes alongside recordMap tables. Builders emit it so every fixture
// exercises the decoder's skip-non-object path.
const wireVersion = 3

// Entry wraps an entity in the current (spaceId + role-wrapped) v3 wire
// shape, so fixtures exercise the client's normalize-at-boundary path.
func Entry(entity any) map[string]any {
	return map[string]any{
		"spaceId": "space-1",
		"value":   map[string]any{"value": entity, "role": "reader"},
	}
}

// Table builds one recordMap table from id → entity, Entry-wrapping each and
// including the __version__ metadata the live API sends.
func Table(entities map[string]any) map[string]any {
	table := make(map[string]any, len(entities)+1)
	table["__version__"] = wireVersion
	for id, entity := range entities {
		table[id] = Entry(entity)
	}
	return table
}

// versionedTables builds table-name → Table with the __version__ metadata
// the live API sends alongside the tables.
func versionedTables(tables map[string]map[string]any) map[string]any {
	out := make(map[string]any, len(tables)+1)
	out["__version__"] = wireVersion
	for name, entities := range tables {
		out[name] = Table(entities)
	}
	return out
}

// RecordMapBody builds a {"recordMap": {...}} response body from
// table → (id → entity), Entry-wrapping every entity and including the
// __version__ metadata the live API sends.
func RecordMapBody(tables map[string]map[string]any) map[string]any {
	return map[string]any{"recordMap": versionedTables(tables)}
}

// GetSpacesBody builds a getSpaces response for one user in the current wire
// shape (role-wrapped records + __version__ metadata at every level), from
// table → (id → entity).
func GetSpacesBody(userID string, tables map[string]map[string]any) map[string]any {
	return map[string]any{"__version__": wireVersion, userID: versionedTables(tables)}
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
