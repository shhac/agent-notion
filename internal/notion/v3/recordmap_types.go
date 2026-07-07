// V3 entity records — the typed shapes stored in RecordMap tables.
// Pure data declarations; decoding and lookup live in recordmap.go.

package v3

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
