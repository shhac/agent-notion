// V3 RecordMap — the shared data vocabulary of the v3 internal API.
// Entity types, wire-format normalization, and typed lookup helpers.
// No HTTP here: the transport lives in the client.

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

// UnmarshalJSON normalizes both RecordMap entry wire formats.
func (e *Entry) UnmarshalJSON(data []byte) error {
	var outer struct {
		Value json.RawMessage `json:"value"`
		Role  string          `json:"role"`
	}
	if err := json.Unmarshal(data, &outer); err != nil {
		return err
	}

	// New format: outer.Value is itself role-wrapped — an object whose own
	// "value" is an object. (In the old format that slot is the entity, whose
	// "value" field — if any — is not an object.)
	if isJSONObject(outer.Value) {
		var inner struct {
			Value json.RawMessage `json:"value"`
			Role  string          `json:"role"`
		}
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

// Table maps record ID → entry.
type Table map[string]Entry

// UnmarshalJSON skips non-object values (metadata like __version__) so they
// don't fail the whole parse.
func (t *Table) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := make(Table, len(raw))
	for id, entry := range raw {
		if !isJSONObject(entry) {
			continue
		}
		var e Entry
		if err := json.Unmarshal(entry, &e); err != nil {
			return err
		}
		out[id] = e
	}
	*t = out
	return nil
}

// RecordMap maps table name (block, collection, notion_user, space, …) →
// records. Decoding normalizes every entry; see Entry.
type RecordMap map[string]Table

// UnmarshalJSON skips non-object values (metadata like __version__ the API
// now includes alongside the tables) so they don't fail the whole parse.
func (rm *RecordMap) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := make(RecordMap, len(raw))
	for name, table := range raw {
		if !isJSONObject(table) {
			continue
		}
		var t Table
		if err := json.Unmarshal(table, &t); err != nil {
			return err
		}
		out[name] = t
	}
	*rm = out
	return nil
}

// Block is a v3 block record.
type Block struct {
	ID             string              `json:"id"`
	Type           string              `json:"type"`
	Version        int64               `json:"version"`
	CreatedTime    int64               `json:"created_time"`
	LastEditedTime int64               `json:"last_edited_time"`
	ParentID       string              `json:"parent_id"`
	ParentTable    string              `json:"parent_table"`
	Alive          *bool               `json:"alive"`
	Properties     map[string]RichText `json:"properties"`
	Content        []string            `json:"content"`
	Discussions    []string            `json:"discussions"`
	Format         map[string]any      `json:"format"`
	SpaceID        string              `json:"space_id"`
	// CollectionID and ViewIDs are set on collection_view/_page blocks.
	CollectionID string   `json:"collection_id"`
	ViewIDs      []string `json:"view_ids"`
}

// IsAlive treats a missing alive field as alive (matching the TS
// `alive !== false` checks).
func (b *Block) IsAlive() bool { return b.Alive == nil || *b.Alive }

// Property returns the named property's rich text (nil when absent).
func (b *Block) Property(name string) RichText { return b.Properties[name] }

// Collection is a v3 collection (database) record.
type Collection struct {
	ID          string                    `json:"id"`
	Version     int64                     `json:"version"`
	Name        RichText                  `json:"name"`
	Description RichText                  `json:"description,omitempty"`
	Schema      map[string]PropertySchema `json:"schema"`
	ParentID    string                    `json:"parent_id"`
	ParentTable string                    `json:"parent_table"`
	Icon        string                    `json:"icon,omitempty"`
	Cover       string                    `json:"cover,omitempty"`
	Format      map[string]any            `json:"format,omitempty"`
}

// PropertySchemaOption is one select/multi-select/status option.
type PropertySchemaOption struct {
	ID    string `json:"id"`
	Value string `json:"value"`
	Color string `json:"color,omitempty"`
}

// PropertySchemaGroup groups status options.
type PropertySchemaGroup struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	OptionIDs []string `json:"optionIds,omitempty"`
	Color     string   `json:"color,omitempty"`
}

// PropertySchema describes one collection property.
type PropertySchema struct {
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Options      []PropertySchemaOption `json:"options,omitempty"`
	Groups       []PropertySchemaGroup  `json:"groups,omitempty"`
	NumberFormat string                 `json:"number_format,omitempty"`
	CollectionID string                 `json:"collection_id,omitempty"`
}


// User is a v3 notion_user record.
type User struct {
	ID           string `json:"id"`
	Version      int64  `json:"version"`
	Email        string `json:"email"`
	GivenName    string `json:"given_name"`
	FamilyName   string `json:"family_name"`
	ProfilePhoto string `json:"profile_photo,omitempty"`
}

// Space is a v3 space record.
type Space struct {
	ID       string `json:"id"`
	Version  int64  `json:"version"`
	Name     string `json:"name"`
	Icon     string `json:"icon,omitempty"`
	Domain   string `json:"domain,omitempty"`
	PlanType string `json:"plan_type,omitempty"`
}

// Discussion is a v3 discussion (comment thread) record.
type Discussion struct {
	ID          string   `json:"id"`
	Version     int64    `json:"version"`
	ParentID    string   `json:"parent_id"`
	ParentTable string   `json:"parent_table"`
	Resolved    bool     `json:"resolved"`
	Comments    []string `json:"comments"`
}

// Comment is a v3 comment record.
type Comment struct {
	ID             string   `json:"id"`
	Version        int64    `json:"version"`
	Alive          *bool    `json:"alive"`
	ParentID       string   `json:"parent_id"`
	ParentTable    string   `json:"parent_table"`
	Text           RichText `json:"text"`
	CreatedByID    string   `json:"created_by_id"`
	CreatedByTable string   `json:"created_by_table"`
	CreatedTime    int64    `json:"created_time"`
	LastEditedTime int64    `json:"last_edited_time"`
}

// AuthorRef identifies an edit author in snapshots and activity.
type AuthorRef struct {
	ID    string `json:"id"`
	Table string `json:"table"`
}

// Snapshot is a v3 page-history snapshot record.
type Snapshot struct {
	ID          string      `json:"id"`
	Version     int64       `json:"version"`
	LastVersion int64       `json:"last_version"`
	Timestamp   int64       `json:"timestamp"`
	Authors     []AuthorRef `json:"authors"`
}

// ActivityEdit is one edit inside an activity record.
type ActivityEdit struct {
	Type      string      `json:"type"`
	BlockID   string      `json:"block_id,omitempty"`
	Timestamp int64       `json:"timestamp"`
	Authors   []AuthorRef `json:"authors,omitempty"`
}

// Activity is a v3 activity-log record.
type Activity struct {
	ID               string         `json:"id"`
	Version          int64          `json:"version"`
	Type             string         `json:"type"`
	ParentID         string         `json:"parent_id"`
	ParentTable      string         `json:"parent_table"`
	NavigableBlockID string         `json:"navigable_block_id,omitempty"`
	CollectionID     string         `json:"collection_id,omitempty"`
	SpaceID          string         `json:"space_id"`
	Edits            []ActivityEdit `json:"edits,omitempty"`
	StartTime        int64          `json:"start_time,omitempty"`
	EndTime          int64          `json:"end_time,omitempty"`
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

// sortedIDs returns a table's record IDs in stable order. The TS relied on
// JS object insertion order for "first X" lookups; Go maps are unordered, so
// we sort for determinism.
func sortedIDs(t Table) []string {
	ids := make([]string, 0, len(t))
	for id := range t {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func firstEntity[T any](t Table) (*T, bool) {
	for _, id := range sortedIDs(t) {
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
	for _, id := range sortedIDs(table) {
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
	for _, id := range sortedIDs(table) {
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
	ids := sortedIDs(rm["collection_view"])
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
