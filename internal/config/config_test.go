package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---- helpers ----------------------------------------------------------------

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// ---- Load -------------------------------------------------------------------

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, ".svalconfig.yaml", `
rules:
  - pattern: "**/*.md"
    schema: "./schemas/fm.json"
  - pattern: "**/*.yaml"
    schema: "./schemas/data.json"
ignore:
  - ".git/**"
  - "vendor/**"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(cfg.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Pattern != "**/*.md" {
		t.Fatalf("unexpected rule pattern: %q", cfg.Rules[0].Pattern)
	}
	if cfg.Rules[0].Schema != "./schemas/fm.json" {
		t.Fatalf("unexpected rule schema: %q", cfg.Rules[0].Schema)
	}
	if len(cfg.Ignore) != 2 {
		t.Fatalf("expected 2 ignore patterns, got %d", len(cfg.Ignore))
	}
}

func TestLoadTOML(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, ".svalconfig.toml", `
ignore = [".git/**"]

[[rules]]
pattern = "**/*.md"
schema = "./schemas/fm.json"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(cfg.Rules))
	}
	if len(cfg.Ignore) != 1 {
		t.Fatalf("expected 1 ignore pattern, got %d", len(cfg.Ignore))
	}
}

func TestLoadJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, ".svalconfig.json", `{
  "rules": [{"pattern": "**/*.yaml", "schema": "./s.json"}],
  "ignore": ["vendor/**"]
}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(cfg.Rules))
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadUnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "svalconfig.ini", "rules=[]")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}

// ---- Discover ---------------------------------------------------------------

func TestDiscoverFindsConfigInDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".svalconfig.yaml", `rules: []`)

	cfgPath, cfg, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected cfg, got nil")
	}
	if cfgPath == "" {
		t.Fatal("expected non-empty config path")
	}
}

func TestDiscoverDoesNotWalkUp(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "subdir")
	if err := os.MkdirAll(child, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, parent, ".svalconfig.yaml", `rules: []`)

	// Config is in parent, not in child - should not be found.
	cfgPath, cfg, err := Discover(child)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if cfgPath != "" || cfg != nil {
		t.Fatalf("expected no config (should not walk up), got path=%q", cfgPath)
	}
}

func TestDiscoverReturnsNilWhenNotFound(t *testing.T) {
	dir := t.TempDir()

	cfgPath, cfg, err := Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfgPath != "" || cfg != nil {
		t.Fatalf("expected no config, got path=%q cfg=%v", cfgPath, cfg)
	}
}

func TestDiscoverPriorityOrder(t *testing.T) {
	dir := t.TempDir()
	// Write two candidates; the one earlier in the priority list should win.
	writeFile(t, dir, "sval.yaml", `rules: []`)
	writeFile(t, dir, ".svalconfig.yaml", `rules: []`)

	cfgPath, _, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if filepath.Base(cfgPath) != ".svalconfig.yaml" {
		t.Fatalf("expected .svalconfig.yaml to win, got: %q", cfgPath)
	}
}

// ---- FromVSCode -------------------------------------------------------------

func buildVSCodeSettings(t *testing.T, dir string, content string) string {
	t.Helper()
	return writeFile(t, filepath.Join(dir, ".vscode"), "settings.json", content)
}

func TestFromVSCodeBasic(t *testing.T) {
	dir := t.TempDir()
	settingsPath := buildVSCodeSettings(t, dir, `{
  "yaml.schemas": {
    "schemas/a.json": ["docs/*.md", "notes/*.yaml"],
    "schemas/b.json": "content/*.toml"
  }
}`)

	cfg, err := FromVSCode(settingsPath)
	if err != nil {
		t.Fatalf("FromVSCode() error: %v", err)
	}
	if len(cfg.Rules) != 3 {
		t.Fatalf("expected 3 rules (2 from a + 1 from b), got %d: %+v", len(cfg.Rules), cfg.Rules)
	}
	// All rule schemas should be absolute paths rooted in dir.
	for _, r := range cfg.Rules {
		if !filepath.IsAbs(r.Schema) {
			t.Fatalf("expected absolute schema path, got %q", r.Schema)
		}
	}
}

func TestFromVSCodeFileScheme(t *testing.T) {
	dir := t.TempDir()
	settingsPath := buildVSCodeSettings(t, dir, `{
  "yaml.schemas": {
    "file:///var/schemas/remote.json": ["definitions/*.yaml"]
  }
}`)

	cfg, err := FromVSCode(settingsPath)
	if err != nil {
		t.Fatalf("FromVSCode() error: %v", err)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Schema != filepath.Clean("/var/schemas/remote.json") {
		t.Fatalf("unexpected schema path: %q", cfg.Rules[0].Schema)
	}
}

func TestFromVSCodeHTTPSSchemaPreserved(t *testing.T) {
	dir := t.TempDir()
	settingsPath := buildVSCodeSettings(t, dir, `{
  "yaml.schemas": {
    "https://example.com/schema.json": ["docs/*.md"]
  }
}`)

	cfg, err := FromVSCode(settingsPath)
	if err != nil {
		t.Fatalf("FromVSCode() error: %v", err)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Schema != "https://example.com/schema.json" {
		t.Fatalf("expected https URL preserved, got %q", cfg.Rules[0].Schema)
	}
}

func TestFromVSCodeUnsupportedSchemeSkipped(t *testing.T) {
	dir := t.TempDir()
	settingsPath := buildVSCodeSettings(t, dir, `{
  "yaml.schemas": {
    "gcs://bucket/schema.json": ["docs/*.md"],
    "schemas/local.json": ["notes/*.yaml"]
  }
}`)

	cfg, err := FromVSCode(settingsPath)
	if err != nil {
		t.Fatalf("FromVSCode() error: %v", err)
	}
	// gcs:// skipped, only the local one should remain
	if len(cfg.Rules) != 1 {
		t.Fatalf("expected 1 rule (unsupported scheme skipped), got %d: %+v", len(cfg.Rules), cfg.Rules)
	}
}

func TestFromVSCodeMissingYAMLSchemas(t *testing.T) {
	dir := t.TempDir()
	settingsPath := buildVSCodeSettings(t, dir, `{"editor.tabSize": 2}`)

	_, err := FromVSCode(settingsPath)
	if err == nil {
		t.Fatal("expected error for missing yaml.schemas key")
	}
}

func TestFromVSCodeMissingFile(t *testing.T) {
	_, err := FromVSCode(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFromVSCodeBaseDirAboveVSCode(t *testing.T) {
	// Schemas with relative paths in .vscode/settings.json should resolve
	// relative to the project root (parent of .vscode/), not .vscode/ itself.
	dir := t.TempDir()
	settingsPath := buildVSCodeSettings(t, dir, `{
  "yaml.schemas": {
    "schemas/test.json": ["data/*.yaml"]
  }
}`)

	cfg, err := FromVSCode(settingsPath)
	if err != nil {
		t.Fatalf("FromVSCode() error: %v", err)
	}
	want := filepath.Join(dir, "schemas", "test.json")
	for _, r := range cfg.Rules {
		if r.Schema == want {
			return
		}
	}
	// marshal rules for the error message
	b, _ := json.MarshalIndent(cfg.Rules, "", "  ")
	t.Fatalf("expected schema %q in rules, got: %s", want, b)
}
