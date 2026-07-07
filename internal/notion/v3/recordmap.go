// V3 RecordMap — the shared data vocabulary of the v3 internal API.
// Wire-format normalization and typed lookup helpers; the entity structs
// live in recordmap_types.go. No HTTP here: the transport lives in the
// client.

package v3

import (
	"bytes"
	"encoding/json"
	"sort"
)

// Entry is one normalized RecordMap record: { value: entity, role? }.
//
// Invariant: after JSON decoding, Value always holds the entity directly.
// Notion's v3 API changed the wire format from { value: entity, role? } to
// { spaceId, value: { value: entity, role? } }; UnmarshalJSON unwraps the
// extra nesting, so consumers never re-unwrap. (The TS port did this with a
// normalizeRecordMapResponse tree-walk after parsing; here it happens at the
// decode boundary.)
type Entry struct {
	Value json.RawMessage `json:"value"`
	Role  string          `json:"role,omitempty"`
}

// roleWrapped is one level of {value, role} wrapping on the wire.
type roleWrapped struct {
	Value json.RawMessage `json:"value"`
	Role  string          `json:"role"`
}

// UnmarshalJSON normalizes both RecordMap entry wire formats.
func (e *Entry) UnmarshalJSON(data []byte) error {
	var outer roleWrapped
	if err := json.Unmarshal(data, &outer); err != nil {
		return err
	}

	// New format: outer.Value is itself role-wrapped — an object whose own
	// "value" is an object. (In the old format that slot is the entity, whose
	// "value" field — if any — is not an object.)
	if isJSONObject(outer.Value) {
		var inner roleWrapped
		if err := json.Unmarshal(outer.Value, &inner); err == nil && isJSONObject(inner.Value) {
			e.Value, e.Role = inner.Value, inner.Role
			return nil
		}
	}

	e.Value, e.Role = outer.Value, outer.Role
	return nil
}

func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	return len(trimmed) > 0 && trimmed[0] == '{'
}

// decodeObjectMap decodes a JSON object into key → T, skipping non-object
// values (metadata like the __version__ number the API includes alongside
// records). The single home of the skip-non-object contract.
func decodeObjectMap[T any](data []byte) (map[string]T, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]T, len(raw))
	for key, val := range raw {
		if !isJSONObject(val) {
			continue
		}
		var decoded T
		if err := json.Unmarshal(val, &decoded); err != nil {
			return nil, err
		}
		out[key] = decoded
	}
	return out, nil
}

// Table maps record ID → entry. Decoding skips non-object metadata.
type Table map[string]Entry

func (t *Table) UnmarshalJSON(data []byte) error {
	m, err := decodeObjectMap[Entry](data)
	if err != nil {
		return err
	}
	*t = m
	return nil
}

// RecordMap maps table name (block, collection, notion_user, space, …) →
// records. Decoding skips non-object metadata and normalizes every entry;
// see Entry.
type RecordMap map[string]Table

func (rm *RecordMap) UnmarshalJSON(data []byte) error {
	m, err := decodeObjectMap[Table](data)
	if err != nil {
		return err
	}
	*rm = m
	return nil
}

// decodeEntry unmarshals a table entry's entity into T.
func decodeEntry[T any](t Table, id string) (*T, bool) {
	entry, ok := t[id]
	if !ok || len(entry.Value) == 0 {
		return nil, false
	}
	var v T
	if err := json.Unmarshal(entry.Value, &v); err != nil {
		return nil, false
	}
	return &v, true
}

// sortedKeys returns a map's keys in stable order. The TS relied on JS
// object insertion order for "first X" lookups; Go maps are unordered, so we
// sort for determinism.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func firstEntity[T any](t Table) (*T, bool) {
	for _, id := range sortedKeys(t) {
		if v, ok := decodeEntry[T](t, id); ok {
			return v, true
		}
	}
	return nil, false
}

// GetBlock extracts a block by ID.
func (rm RecordMap) GetBlock(id string) (*Block, bool) { return decodeEntry[Block](rm["block"], id) }

// GetCollection extracts a collection by ID.
func (rm RecordMap) GetCollection(id string) (*Collection, bool) {
	return decodeEntry[Collection](rm["collection"], id)
}

// GetDiscussion extracts a discussion by ID.
func (rm RecordMap) GetDiscussion(id string) (*Discussion, bool) {
	return decodeEntry[Discussion](rm["discussion"], id)
}

// GetComment extracts a comment by ID.
func (rm RecordMap) GetComment(id string) (*Comment, bool) {
	return decodeEntry[Comment](rm["comment"], id)
}

// GetUser extracts a user by ID.
func (rm RecordMap) GetUser(id string) (*User, bool) {
	return decodeEntry[User](rm["notion_user"], id)
}

// AllBlocks returns every alive block, ordered by record ID.
func (rm RecordMap) AllBlocks() []*Block {
	table := rm["block"]
	blocks := make([]*Block, 0, len(table))
	for _, id := range sortedKeys(table) {
		if b, ok := decodeEntry[Block](table, id); ok && b.IsAlive() {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

// AllUsers returns every user, ordered by record ID.
func (rm RecordMap) AllUsers() []*User {
	table := rm["notion_user"]
	users := make([]*User, 0, len(table))
	for _, id := range sortedKeys(table) {
		if u, ok := decodeEntry[User](table, id); ok {
			users = append(users, u)
		}
	}
	return users
}

// FirstCollection returns the first collection (by record ID).
func (rm RecordMap) FirstCollection() (*Collection, bool) {
	return firstEntity[Collection](rm["collection"])
}

// FirstCollectionViewID returns the first collection view's ID (by record ID).
func (rm RecordMap) FirstCollectionViewID() (string, bool) {
	ids := sortedKeys(rm["collection_view"])
	if len(ids) == 0 {
		return "", false
	}
	return ids[0], true
}

// FirstUser returns the first user (by record ID).
func (rm RecordMap) FirstUser() (*User, bool) { return firstEntity[User](rm["notion_user"]) }

// FirstSpace returns the first space (by record ID).
func (rm RecordMap) FirstSpace() (*Space, bool) { return firstEntity[Space](rm["space"]) }

// Merge copies records from source into rm, overwriting on ID collision.
func (rm RecordMap) Merge(source RecordMap) {
	for table, records := range source {
		if len(records) == 0 {
			continue
		}
		if rm[table] == nil {
			rm[table] = Table{}
		}
		for id, entry := range records {
			rm[table][id] = entry
		}
	}
}
