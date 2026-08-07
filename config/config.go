package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// CurrentVersion is the current persisted Atlas config format version.
const CurrentVersion = 2

// FilePath returns the conventional Atlas config path for a project root.
func FilePath(root string) string {
	return filepath.Join(root, ".goforj", "atlas.json")
}

// Config describes project-owned Atlas installation state.
type Config struct {
	Version        int                   `json:"version"`
	Features       Features              `json:"features"`
	Agents         []string              `json:"agents"`
	Skills         []string              `json:"skills"`
	GeneratedFiles map[string][]string   `json:"generated_files,omitempty"`
	OwnershipRules []OwnershipRuleConfig `json:"ownership_rules,omitempty"`
	LastDiscovered DiscoverySnapshot     `json:"last_discovered"`
}

// Features records which Atlas surfaces should be kept synchronized.
type Features struct {
	Guidelines bool `json:"guidelines"`
	Skills     bool `json:"skills"`
	MCP        bool `json:"mcp"`
}

// DiscoverySnapshot stores the last project facts Atlas used during install or update.
type DiscoverySnapshot struct {
	Apps       []string `json:"apps,omitempty"`
	Components []string `json:"components,omitempty"`
}

// OwnershipRuleConfig describes a project-owned file ownership override.
type OwnershipRuleConfig struct {
	Pattern         string `json:"pattern"`
	Classification  string `json:"classification"`
	Editable        bool   `json:"editable"`
	PreferredAction string `json:"preferred_action,omitempty"`
	ChangeThrough   string `json:"change_through,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// Default returns the default Atlas configuration.
func Default() Config {
	return Config{
		Version:        CurrentVersion,
		GeneratedFiles: map[string][]string{},
		Features: Features{
			Guidelines: true,
			Skills:     true,
			MCP:        true,
		},
	}
}

// Load reads the Atlas config from a project root.
func Load(root string) (Config, error) {
	content, err := os.ReadFile(FilePath(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(content, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Version == 0 {
		cfg.Version = CurrentVersion
	}
	if cfg.GeneratedFiles == nil {
		cfg.GeneratedFiles = map[string][]string{}
	}
	return cfg, nil
}

// Save writes the Atlas config to a project root.
func Save(root string, cfg Config) error {
	if cfg.Version == 0 {
		cfg.Version = CurrentVersion
	}

	path := FilePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(path, content, 0o644)
}
