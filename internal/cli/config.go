package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/shhac/agent-notion/internal/config"
	libcli "github.com/shhac/lib-agent-cli/cli"
	"github.com/spf13/cobra"
)

// registerConfig adds the family-standard `config` group (get/set/unset/list)
// over the tunable settings in config.json.
func registerConfig(root *cobra.Command) {
	root.AddCommand(libcli.ConfigCommand(configKeys()))
}

// configKeys defines the settable config keys, mapping each dotted name to the
// matching field in config.Settings. Validation and error messages are ported
// from the TS config command group.
func configKeys() []libcli.ConfigKey {
	return []libcli.ConfigKey{
		{
			Name:        "page_size",
			Description: "Default results per list command (1-100)",
			Get: func() (string, bool) {
				if n := config.ReadSettings().PageSize; n != 0 {
					return strconv.Itoa(n), true
				}
				return "", false
			},
			Set: func(v string) error {
				n, err := strconv.Atoi(strings.TrimSpace(v))
				if err != nil || n < 1 || n > 100 {
					return fmt.Errorf("Invalid value: %s. Must be an integer between 1 and 100 (Notion API max).", v)
				}
				s := config.ReadSettings()
				s.PageSize = n
				return config.WriteSettings(s)
			},
			Unset: func() error {
				s := config.ReadSettings()
				s.PageSize = 0
				return config.WriteSettings(s)
			},
		},
		{
			Name:        "max_depth",
			Description: "Max nesting depth when recursively fetching blocks (unset = no limit)",
			Get: func() (string, bool) {
				if n := config.ReadSettings().MaxDepth; n != 0 {
					return strconv.Itoa(n), true
				}
				return "", false
			},
			Set: func(v string) error {
				// 0 cannot round-trip through the omitempty config field;
				// "no limit" is expressed by unsetting the key.
				n, err := strconv.Atoi(strings.TrimSpace(v))
				if err != nil || n < 1 {
					return fmt.Errorf("Invalid value: %s. Must be a positive integer; unset the key for no limit.", v)
				}
				s := config.ReadSettings()
				s.MaxDepth = n
				return config.WriteSettings(s)
			},
			Unset: func() error {
				s := config.ReadSettings()
				s.MaxDepth = 0
				return config.WriteSettings(s)
			},
		},
		{
			Name:        "truncation.max_length",
			Description: "Max characters before truncating description/body/content (unset = default 200)",
			Get: func() (string, bool) {
				s := config.ReadSettings()
				if s.Truncation == nil || s.Truncation.MaxLength == 0 {
					return "", false
				}
				return strconv.Itoa(s.Truncation.MaxLength), true
			},
			Set: func(v string) error {
				// 0 cannot round-trip through the omitempty config field;
				// per-invocation --full already covers "never truncate".
				n, err := strconv.Atoi(strings.TrimSpace(v))
				if err != nil || n < 1 {
					return fmt.Errorf("Invalid value: %s. Must be a positive integer; unset the key for the default.", v)
				}
				s := config.ReadSettings()
				if s.Truncation == nil {
					s.Truncation = &config.Truncation{}
				}
				s.Truncation.MaxLength = n
				return config.WriteSettings(s)
			},
			Unset: func() error {
				s := config.ReadSettings()
				if s.Truncation != nil {
					s.Truncation.MaxLength = 0
				}
				return config.WriteSettings(s)
			},
		},
		{
			Name:        "ai.default_model",
			Description: "Default AI model codename (e.g. oatmeal-cookie); use 'ai model list' for options",
			Get: func() (string, bool) {
				s := config.ReadSettings()
				if s.AI == nil || s.AI.DefaultModel == "" {
					return "", false
				}
				return s.AI.DefaultModel, true
			},
			Set: func(v string) error {
				model := strings.TrimSpace(v)
				if model == "" {
					return errors.New("Model name cannot be empty. Use 'ai model list' to see available models.")
				}
				s := config.ReadSettings()
				if s.AI == nil {
					s.AI = &config.AISettings{}
				}
				s.AI.DefaultModel = model
				return config.WriteSettings(s)
			},
			Unset: func() error {
				s := config.ReadSettings()
				if s.AI != nil {
					s.AI.DefaultModel = ""
				}
				return config.WriteSettings(s)
			},
		},
	}
}
