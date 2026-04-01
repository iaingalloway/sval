package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"os"
)

func TestValidateCommandRequiresArguments(t *testing.T) {
	root := newRootCmd("dev")
	root.SetArgs([]string{"validate"})

	var out bytes.Buffer
	var errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)

	err := root.Execute()
	if err == nil {
		t.Fatalf("expected error for missing file arg")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg(s)") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCommandRequiresSchemaFlag(t *testing.T) {
	root := newRootCmd("dev")
	root.SetArgs([]string{"validate", "note.yaml"})

	err := root.Execute()
	if err == nil {
		t.Fatalf("expected error for missing --schema")
	}
	if !strings.Contains(err.Error(), "required flag(s) \"schema\" not set") {
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
