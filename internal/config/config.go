package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Rule maps a glob pattern to a schema file path.
type Rule struct {
	Pattern string `yaml:"pattern" toml:"pattern" json:"pattern"`
	Schema  string `yaml:"schema"  toml:"schema"  json:"schema"`
}

// Config is the parsed representation of a sval config file.
type Config struct {
	Rules  []Rule   `yaml:"rules"  toml:"rules"  json:"rules"`
	Ignore []string `yaml:"ignore" toml:"ignore" json:"ignore"`
}

// discoveryNames lists candidate config file base names in priority order.
// For each base name all four extensions are tried in order.
var discoveryNames = []string{
	".svalconfig",
	"svalconfig",
	".sval",
	"sval",
}

var discoveryExts = []string{".yaml", ".yml", ".toml", ".json"}

// Load reads and parses the config file at path, dispatching on file extension.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse yaml config: %w", err)
		}
		return &cfg, nil
	case ".toml":
		var cfg Config
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse toml config: %w", err)
		}
		return &cfg, nil
	case ".json":
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse json config: %w", err)
		}
		return &cfg, nil
	default:
		return nil, fmt.Errorf("unsupported config file extension %q", ext)
	}
}

// Discover looks for a config file in dir only (no upward traversal).
// Returns ("", nil, nil) if no config file is found.
func Discover(dir string) (string, *Config, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", nil, fmt.Errorf("resolve directory %q: %w", dir, err)
	}

	for _, base := range discoveryNames {
		for _, ext := range discoveryExts {
			candidate := filepath.Join(abs, base+ext)
			if _, err := os.Stat(candidate); err == nil {
				cfg, err := Load(candidate)
				if err != nil {
					return "", nil, fmt.Errorf("load config %s: %w", candidate, err)
				}
				return candidate, cfg, nil
			}
		}
	}

	return "", nil, nil
}

// FromVSCode reads a VS Code settings.json file and converts the yaml.schemas
// mapping to a Config. Schema paths are resolved relative to the directory
// containing the .vscode folder. Remote (http/https) schemas are preserved;
// file:// schemas are resolved to local paths. Schemas with unsupported URL
// schemes are skipped; their keys are returned as warnings.
func FromVSCode(settingsPath string) (*Config, []string, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read vscode settings: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("parse vscode settings: %w", err)
	}

	schemasNode, ok := raw["yaml.schemas"]
	if !ok {
		return nil, nil, fmt.Errorf("yaml.schemas key not found in %s", settingsPath)
	}

	// yaml.schemas maps schema path/URL → []glob or glob
	var schemaMap map[string]json.RawMessage
	if err := json.Unmarshal(schemasNode, &schemaMap); err != nil {
		return nil, nil, fmt.Errorf("parse yaml.schemas: %w", err)
	}

	baseDir := vscodeBaseDir(settingsPath)

	var rules []Rule
	var warnings []string
	for rawSchema, patternsRaw := range schemaMap {
		schemaPath, skippedKey, err := normalizeVSCodeSchema(rawSchema, baseDir)
		if err != nil {
			return nil, nil, err
		}
		if skippedKey != "" {
			warnings = append(warnings, skippedKey)
			continue
		}

		// patterns can be a JSON string or array of strings
		patterns, err := parseVSCodePatterns(patternsRaw)
		if err != nil {
			return nil, nil, fmt.Errorf("parse patterns for schema %q: %w", rawSchema, err)
		}

		for _, p := range patterns {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if !filepath.IsAbs(p) {
				p = filepath.Join(baseDir, p)
			}
			rules = append(rules, Rule{Pattern: p, Schema: schemaPath})
		}
	}

	return &Config{Rules: rules}, warnings, nil
}

func vscodeBaseDir(settingsPath string) string {
	dir := filepath.Dir(settingsPath)
	if filepath.Base(dir) == ".vscode" {
		return filepath.Dir(dir)
	}
	return dir
}

// normalizeVSCodeSchema resolves a schema path/URL from VS Code settings.
// Returns (resolved, skippedKey, error). skippedKey is non-empty when the
// schema has an unsupported URL scheme and should be skipped; it contains the
// original key for use in diagnostic messages.
func normalizeVSCodeSchema(rawSchema string, baseDir string) (string, string, error) {
	trimmed := strings.TrimSpace(rawSchema)
	if trimmed == "" {
		return "", "", fmt.Errorf("empty schema path in vscode settings")
	}

	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" {
		switch parsed.Scheme {
		case "file":
			if parsed.Path == "" {
				return "", "", fmt.Errorf("schema %q has empty file path", rawSchema)
			}
			return filepath.Clean(parsed.Path), "", nil
		case "http", "https":
			return parsed.String(), "", nil
		default:
			// unsupported scheme - report the key so the caller can warn
			return "", rawSchema, nil
		}
	}

	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed), "", nil
	}
	return filepath.Clean(filepath.Join(baseDir, trimmed)), "", nil
}

func parseVSCodePatterns(raw json.RawMessage) ([]string, error) {
	// try array first
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	// try single string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []string{s}, nil
	}
	return nil, fmt.Errorf("expected string or array of strings")
}
