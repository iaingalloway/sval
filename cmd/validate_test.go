package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"os"
)

func TestValidateCommandNoConfigFound(t *testing.T) {
	// Run from a temp dir that has no config file.
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	root := newRootCmd("dev")
	root.SetArgs([]string{"validate"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when no config is found")
	}
	if !strings.Contains(err.Error(), "no config file found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCommandSchemaWithoutFile(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schemaPath, []byte(`{"type":"object"}`), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	root := newRootCmd("dev")
	root.SetArgs([]string{"validate", "--schema", schemaPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --schema given without a file argument")
	}
	if !strings.Contains(err.Error(), "exactly one file argument") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCommandSchemaAndConfigMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	configPath := filepath.Join(dir, ".svalconfig.yaml")
	if err := os.WriteFile(schemaPath, []byte(`{"type":"object"}`), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("rules: []\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	root := newRootCmd("dev")
	root.SetArgs([]string{"validate", "--schema", schemaPath, "--config", configPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error combining --schema and --config")
	}
	if !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCommandSuccess(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	dataPath := filepath.Join(dir, "note.yaml")

	if err := os.WriteFile(schemaPath, []byte(`{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`), 0o600); err != nil {
		t.Fatalf("write schema file: %v", err)
	}
	if err := os.WriteFile(dataPath, []byte("name: ok\n"), 0o600); err != nil {
		t.Fatalf("write data file: %v", err)
	}

	root := newRootCmd("dev")
	root.SetArgs([]string{"validate", dataPath, "--schema", schemaPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestValidateCommandJSONValid(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	dataPath := filepath.Join(dir, "note.yaml")

	if err := os.WriteFile(schemaPath, []byte(`{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	if err := os.WriteFile(dataPath, []byte("name: ok\n"), 0o600); err != nil {
		t.Fatalf("write data: %v", err)
	}

	root := newRootCmd("dev")
	root.SetArgs([]string{"validate", dataPath, "--schema", schemaPath, "--json"})

	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result["valid"] != true {
		t.Fatalf("expected valid: true, got: %v", result["valid"])
	}
	if _, ok := result["errors"]; ok {
		t.Fatalf("expected no errors key in output, got: %v", result)
	}
}

func TestValidateCommandJSONInvalid(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	dataPath := filepath.Join(dir, "note.yaml")

	if err := os.WriteFile(schemaPath, []byte(`{"type":"object","required":["name","count"],"properties":{"name":{"type":"string"},"count":{"type":"integer"}}}`), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	// missing required "count" field
	if err := os.WriteFile(dataPath, []byte("name: ok\n"), 0o600); err != nil {
		t.Fatalf("write data: %v", err)
	}

	root := newRootCmd("dev")
	root.SetArgs([]string{"validate", dataPath, "--schema", schemaPath, "--json"})

	var out bytes.Buffer
	root.SetOut(&out)

	err := root.Execute()
	if err == nil {
		t.Fatal("expected non-nil error for invalid file")
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result["valid"] != false {
		t.Fatalf("expected valid: false, got: %v", result["valid"])
	}
	errors, ok := result["errors"].([]any)
	if !ok || len(errors) == 0 {
		t.Fatalf("expected non-empty errors array, got: %v", result)
	}
}

func TestValidateCommandJSONSystemError(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "note.yaml")
	if err := os.WriteFile(dataPath, []byte("name: ok\n"), 0o600); err != nil {
		t.Fatalf("write data: %v", err)
	}

	root := newRootCmd("dev")
	root.SetArgs([]string{"validate", dataPath, "--schema", filepath.Join(dir, "missing-schema.json"), "--json"})

	var out bytes.Buffer
	root.SetOut(&out)

	err := root.Execute()
	if err == nil {
		t.Fatal("expected non-nil error for system error")
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	errMsg, ok := result["error"].(string)
	if !ok || errMsg == "" {
		t.Fatalf("expected non-empty error field, got: %v", result)
	}
	if !strings.Contains(errMsg, "load schema") {
		t.Fatalf("expected 'load schema' in error, got: %s", errMsg)
	}
}

func TestValidateCommandNoJSONWritesToStderr(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	dataPath := filepath.Join(dir, "note.yaml")

	if err := os.WriteFile(schemaPath, []byte(`{"type":"object","required":["name","count"],"properties":{"name":{"type":"string"},"count":{"type":"integer"}}}`), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	// missing required "count"
	if err := os.WriteFile(dataPath, []byte("name: ok\n"), 0o600); err != nil {
		t.Fatalf("write data: %v", err)
	}

	root := newRootCmd("dev")
	root.SetArgs([]string{"validate", dataPath, "--schema", schemaPath})

	var out bytes.Buffer
	root.SetOut(&out)

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid file")
	}
	// Without --json, stdout should be empty (errors go via returned error, not stdout).
	if out.Len() != 0 {
		t.Fatalf("expected empty stdout without --json, got: %s", out.String())
	}
}

// ---- Config mode ------------------------------------------------------------

func writeConfigTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestValidateCommandConfigModeValid(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	dataPath := filepath.Join(dir, "data", "note.yaml")
	configPath := filepath.Join(dir, ".svalconfig.yaml")

	writeConfigTestFile(t, schemaPath, `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`)
	writeConfigTestFile(t, dataPath, "name: ok\n")
	writeConfigTestFile(t, configPath, "rules:\n  - pattern: \"data/**/*.yaml\"\n    schema: \"schema.json\"\n")

	root := newRootCmd("dev")
	root.SetArgs([]string{"validate", "--config", configPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestValidateCommandConfigModeInvalid(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	dataPath := filepath.Join(dir, "data", "note.yaml")
	configPath := filepath.Join(dir, ".svalconfig.yaml")

	writeConfigTestFile(t, schemaPath, `{"type":"object","required":["name","count"],"properties":{"name":{"type":"string"},"count":{"type":"integer"}}}`)
	// missing required "count"
	writeConfigTestFile(t, dataPath, "name: ok\n")
	writeConfigTestFile(t, configPath, "rules:\n  - pattern: \"data/**/*.yaml\"\n    schema: \"schema.json\"\n")

	root := newRootCmd("dev")
	root.SetArgs([]string{"validate", "--config", configPath})

	var errOut bytes.Buffer
	root.SetErr(&errOut)

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid file")
	}
	if !strings.Contains(errOut.String(), "count") {
		t.Fatalf("expected validation error on stderr, got: %q", errOut.String())
	}
}

func TestValidateCommandConfigModeIgnore(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	goodPath := filepath.Join(dir, "data", "good.yaml")
	ignoredPath := filepath.Join(dir, "vendor", "ignored.yaml")
	configPath := filepath.Join(dir, ".svalconfig.yaml")

	writeConfigTestFile(t, schemaPath, `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`)
	writeConfigTestFile(t, goodPath, "name: ok\n")
	// ignored.yaml is invalid - if ignore works, no error should be returned
	writeConfigTestFile(t, ignoredPath, "bad: true\n")
	// pattern covers both data/ and vendor/; ignore excludes vendor/
	writeConfigTestFile(t, configPath, "rules:\n  - pattern: \"{data,vendor}/**/*.yaml\"\n    schema: \"schema.json\"\nignore:\n  - \"vendor/**\"\n")

	root := newRootCmd("dev")
	root.SetArgs([]string{"validate", "--config", configPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("expected success (ignored file should not cause error), got: %v", err)
	}
}

func TestValidateCommandConfigModeJSON(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	dataPath := filepath.Join(dir, "data", "note.yaml")
	configPath := filepath.Join(dir, ".svalconfig.yaml")

	writeConfigTestFile(t, schemaPath, `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`)
	writeConfigTestFile(t, dataPath, "name: ok\n")
	writeConfigTestFile(t, configPath, "rules:\n  - pattern: \"data/**/*.yaml\"\n    schema: \"schema.json\"\n")

	root := newRootCmd("dev")
	root.SetArgs([]string{"validate", "--config", configPath, "--json"})

	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.Execute(); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Output should be NDJSON: one line per file
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 JSON line, got %d: %s", len(lines), out.String())
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &result); err != nil {
		t.Fatalf("line is not valid JSON: %v\nline: %s", err, lines[0])
	}
	if result["valid"] != true {
		t.Fatalf("expected valid: true, got: %v", result["valid"])
	}
}

func TestValidateCommandConfigModeAutoDiscover(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	dataPath := filepath.Join(dir, "data", "note.yaml")
	configPath := filepath.Join(dir, ".svalconfig.yaml")

	writeConfigTestFile(t, schemaPath, `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`)
	writeConfigTestFile(t, dataPath, "name: ok\n")
	writeConfigTestFile(t, configPath, "rules:\n  - pattern: \"data/*.yaml\"\n    schema: \"schema.json\"\n")

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	root := newRootCmd("dev")
	root.SetArgs([]string{"validate"})

	if err := root.Execute(); err != nil {
		t.Fatalf("expected auto-discovered config to succeed, got: %v", err)
	}
}
