// Settings read/modify/write helpers, kept beside the Settings type so the
// pruning rules that keep config.json minimal live with the shape they prune.

package config

// ReadSettings returns the persisted settings, or the zero value when the
// config has none.
func ReadSettings() Settings {
	c := Read()
	if c.Settings == nil {
		return Settings{}
	}
	return *c.Settings
}

// WriteSettings persists settings back to config.json, pruning empty nested
// objects (and the whole `settings` key) so a fully-cleared configuration
// leaves no empty scaffolding behind — matching how the TS dropped settings on
// a full reset.
func WriteSettings(s Settings) error {
	c := Read()
	c.Settings = pruneSettings(s)
	return Write(c)
}

// pruneSettings drops empty truncation/ai sub-objects and returns nil when no
// setting carries a value, so `settings` disappears from the file entirely.
func pruneSettings(s Settings) *Settings {
	if s.Truncation != nil && *s.Truncation == (Truncation{}) {
		s.Truncation = nil
	}
	if s.AI != nil && *s.AI == (AISettings{}) {
		s.AI = nil
	}
	if s == (Settings{}) {
		return nil
	}
	return &s
}
