package config

import "testing"

func TestWriteSettingsRoundTripAndPruneAll(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := WriteSettings(Settings{PageSize: 20}); err != nil {
		t.Fatal(err)
	}
	if got := ReadSettings(); got.PageSize != 20 {
		t.Fatalf("round-trip = %+v", got)
	}
	if Read().Settings == nil {
		t.Fatal("settings should be present after a set")
	}

	// Clearing the last value drops the whole settings object.
	if err := WriteSettings(Settings{}); err != nil {
		t.Fatal(err)
	}
	if Read().Settings != nil {
		t.Fatalf("settings should be pruned to nil, got %+v", Read().Settings)
	}
}

func TestWriteSettingsPrunesEmptyNested(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// A nested object with no value collapses to nothing.
	if err := WriteSettings(Settings{Truncation: &Truncation{}}); err != nil {
		t.Fatal(err)
	}
	if Read().Settings != nil {
		t.Fatal("empty truncation should prune the whole settings object")
	}

	// A populated nested object survives while an empty sibling is dropped.
	if err := WriteSettings(Settings{Truncation: &Truncation{MaxLength: 500}, AI: &AISettings{}}); err != nil {
		t.Fatal(err)
	}
	s := Read().Settings
	if s == nil || s.Truncation == nil || s.Truncation.MaxLength != 500 {
		t.Fatalf("truncation not persisted: %+v", s)
	}
	if s.AI != nil {
		t.Fatalf("empty ai should be pruned: %+v", s.AI)
	}
}

func TestReadSettingsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got := ReadSettings(); got != (Settings{}) {
		t.Fatalf("expected zero settings, got %+v", got)
	}
}
