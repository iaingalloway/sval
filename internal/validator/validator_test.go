package validator

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func fixturePath(parts ...string) string {
	items := append([]string{"testdata"}, parts...)
	return filepath.Join(items...)
}

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		path string
		want FileType
	}{
		{path: "note.md", want: FileTypeMarkdown},
		{path: "config.yaml", want: FileTypeYAML},
		{path: "config.YML", want: FileTypeYAML},
		{path: "config.json", want: FileTypeJSON},
		{path: "config.toml", want: FileTypeTOML},
	}

	for _, tt := range tests {
		if got := DetectFileType(tt.path); got != tt.want {
			t.Fatalf("DetectFileType(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestValidateFile(t *testing.T) {
	schema := newTestSchema(t)
	dir := t.TempDir()

	t.Run("yaml success", func(t *testing.T) {
		path := writeTempFile(t, dir, "good.yaml", "name: ok\ncount: 1\n")
		if err := ValidateFile(path, schema); err != nil {
			t.Fatalf("ValidateFile() unexpected error: %v", err)
		}
	})

	t.Run("markdown success", func(t *testing.T) {
		path := writeTempFile(t, dir, "good.md", "---\nname: ok\ncount: 1\n---\nBody")
		if err := ValidateFile(path, schema); err != nil {
			t.Fatalf("ValidateFile() unexpected error: %v", err)
		}
	})

	t.Run("json success", func(t *testing.T) {
		path := writeTempFile(t, dir, "good.json", `{"name":"ok","count":1}`)
		if err := ValidateFile(path, schema); err != nil {
			t.Fatalf("ValidateFile() unexpected error: %v", err)
		}
	})

	t.Run("toml success", func(t *testing.T) {
		path := writeTempFile(t, dir, "good.toml", "name = \"ok\"\ncount = 1\n")
		if err := ValidateFile(path, schema); err != nil {
			t.Fatalf("ValidateFile() unexpected error: %v", err)
		}
	})

	t.Run("markdown json frontmatter success", func(t *testing.T) {
		path := writeTempFile(t, dir, "good-json-frontmatter.md", "---json\n{\"name\":\"ok\",\"count\":1}\n---\nBody")
		if err := ValidateFile(path, schema); err != nil {
			t.Fatalf("ValidateFile() unexpected error: %v", err)
		}
	})

	t.Run("markdown toml frontmatter success", func(t *testing.T) {
		path := writeTempFile(t, dir, "good-toml-frontmatter.md", "+++\nname = \"ok\"\ncount = 1\n+++\nBody")
		if err := ValidateFile(path, schema); err != nil {
			t.Fatalf("ValidateFile() unexpected error: %v", err)
		}
	})

	t.Run("markdown multiple frontmatter blocks success", func(t *testing.T) {
		path := writeTempFile(t, dir, "good-multi-frontmatter.md", "---\nname: ok\ncount: 1\n---\n+++\nname = \"ok2\"\ncount = 2\n+++\nBody")
		if err := ValidateFile(path, schema); err != nil {
			t.Fatalf("ValidateFile() unexpected error: %v", err)
		}
	})

	t.Run("yaml multiple documents success", func(t *testing.T) {
		path := writeTempFile(t, dir, "good-multi.yaml", "name: ok\ncount: 1\n---\nname: ok2\ncount: 2\n")
		if err := ValidateFile(path, schema); err != nil {
			t.Fatalf("ValidateFile() unexpected error: %v", err)
		}
	})

	t.Run("yaml multiple documents failing error shows file line", func(t *testing.T) {
		path := writeTempFile(t, dir, "bad-multi.yaml", "name: ok\ncount: 1\n---\nname: bad\n")
		err := ValidateFile(path, schema)
		if err == nil {
			t.Fatalf("expected validation error, got nil")
		}
		// doc 2 starts at line 4 of the file; line numbers are absolute
		if !strings.Contains(err.Error(), ":4:") {
			t.Fatalf("expected absolute line number in error, got: %v", err)
		}
	})

	t.Run("validation error includes file and line", func(t *testing.T) {
		path := writeTempFile(t, dir, "bad.yaml", "name: 7\ncount: not-a-number\n")
		err := ValidateFile(path, schema)
		if err == nil {
			t.Fatalf("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("error does not include file path: %v", err)
		}
		if !strings.Contains(err.Error(), fmt.Sprintf("%s:1", path)) {
			t.Fatalf("error does not include line info: %v", err)
		}
	})

	t.Run("nil schema", func(t *testing.T) {
		path := writeTempFile(t, dir, "nil-schema.yaml", "name: ok\ncount: 1\n")
		err := ValidateFile(path, nil)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "schema is nil") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		path := writeTempFile(t, dir, "file.txt", "hello")
		err := ValidateFile(path, schema)
		if err == nil {
			t.Fatalf("expected unsupported type error, got nil")
		}
		if !strings.Contains(err.Error(), "unsupported file type") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidatePath(t *testing.T) {
	t.Run("yaml success", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		dataPath := fixturePath("yaml", "good.yaml")

		if err := ValidatePath(dataPath, schemaPath); err != nil {
			t.Fatalf("ValidatePath() unexpected error: %v", err)
		}
	})

	t.Run("json success", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		if err := ValidatePath(fixturePath("json", "good.json"), schemaPath); err != nil {
			t.Fatalf("ValidatePath() unexpected error: %v", err)
		}
	})

	t.Run("json schema error", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		err := ValidatePath(fixturePath("json", "bad.json"), schemaPath)
		if err == nil {
			t.Fatalf("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "count") {
			t.Fatalf("expected missing field in error, got: %v", err)
		}
	})

	t.Run("toml success", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		if err := ValidatePath(fixturePath("toml", "good.toml"), schemaPath); err != nil {
			t.Fatalf("ValidatePath() unexpected error: %v", err)
		}
	})

	t.Run("toml schema error", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		err := ValidatePath(fixturePath("toml", "bad.toml"), schemaPath)
		if err == nil {
			t.Fatalf("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "count") {
			t.Fatalf("expected missing field in error, got: %v", err)
		}
	})

	t.Run("markdown yaml frontmatter success", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		if err := ValidatePath(fixturePath("frontmatter", "good-yaml.md"), schemaPath); err != nil {
			t.Fatalf("ValidatePath() unexpected error: %v", err)
		}
	})

	t.Run("markdown toml frontmatter success", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		if err := ValidatePath(fixturePath("frontmatter", "good-toml.md"), schemaPath); err != nil {
			t.Fatalf("ValidatePath() unexpected error: %v", err)
		}
	})

	t.Run("markdown json frontmatter success", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		if err := ValidatePath(fixturePath("frontmatter", "good-json.md"), schemaPath); err != nil {
			t.Fatalf("ValidatePath() unexpected error: %v", err)
		}
	})

	t.Run("markdown yaml frontmatter schema error", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		err := ValidatePath(fixturePath("frontmatter", "bad-yaml.md"), schemaPath)
		if err == nil {
			t.Fatalf("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "count") {
			t.Fatalf("expected missing field in error, got: %v", err)
		}
	})

	t.Run("markdown mixed frontmatter blocks all valid", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		if err := ValidatePath(fixturePath("frontmatter", "mixed-valid.md"), schemaPath); err != nil {
			t.Fatalf("ValidatePath() unexpected error: %v", err)
		}
	})

	t.Run("markdown mixed frontmatter blocks second fails", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		err := ValidatePath(fixturePath("frontmatter", "mixed-schema-error.md"), schemaPath)
		if err == nil {
			t.Fatalf("expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "count") {
			t.Fatalf("expected missing field in error, got: %v", err)
		}
	})

	t.Run("yaml multi-doc schema error shows absolute line number", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		err := ValidatePath(fixturePath("yaml", "multi-docs-schema-error.yaml"), schemaPath)
		if err == nil {
			t.Fatalf("expected validation error, got nil")
		}
		// count field is on line 7 of multi-docs-schema-error.yaml
		if !strings.Contains(err.Error(), ":7:") {
			t.Fatalf("expected absolute line number in error, got: %v", err)
		}
	})

	t.Run("schema with local $ref resolves", func(t *testing.T) {
		// with-ref.json contains {"$ref": "valid-object.json"} — tests local $ref resolution.
		schemaPath := fixturePath("schema", "with-ref.json")
		if err := ValidatePath(fixturePath("yaml", "good.yaml"), schemaPath); err != nil {
			t.Fatalf("expected $ref to resolve and validation to pass, got: %v", err)
		}
		err := ValidatePath(fixturePath("yaml", "bad-multi.yaml"), schemaPath)
		if err == nil {
			t.Fatal("expected validation error via $ref schema, got nil")
		}
	})

	t.Run("schema loaded from http URL", func(t *testing.T) {
		schemaBytes, err := os.ReadFile(fixturePath("schema", "valid-object.json"))
		if err != nil {
			t.Fatalf("reading fixture: %v", err)
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(schemaBytes)
		}))
		defer srv.Close()

		if err := ValidatePath(fixturePath("yaml", "good.yaml"), srv.URL+"/schema.json"); err != nil {
			t.Fatalf("expected validation to pass, got: %v", err)
		}
		err = ValidatePath(fixturePath("yaml", "bad-multi.yaml"), srv.URL+"/schema.json")
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
	})

	t.Run("missing schema path", func(t *testing.T) {
		dataPath := fixturePath("yaml", "good.yaml")
		err := ValidatePath(dataPath, fixturePath("schema", "missing.json"))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "load schema") {
			t.Fatalf("expected load schema prefix, got: %v", err)
		}
		if !strings.Contains(err.Error(), "no such file or directory") {
			t.Fatalf("expected missing file detail, got: %v", err)
		}
	})

	t.Run("invalid schema json", func(t *testing.T) {
		dataPath := fixturePath("yaml", "good.yaml")
		err := ValidatePath(dataPath, fixturePath("schema", "invalid-schema.json"))
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "load schema") {
			t.Fatalf("expected load schema prefix, got: %v", err)
		}
	})
}

func TestValidatePathResult(t *testing.T) {
	t.Run("valid file returns valid result", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		dataPath := fixturePath("yaml", "good.yaml")

		result, err := ValidatePathResult(dataPath, schemaPath)
		if err != nil {
			t.Fatalf("ValidatePathResult() unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("ValidatePathResult() returned nil result")
		}
		if !result.Valid {
			t.Fatalf("expected valid: true, got errors: %v", result.Errors)
		}
		if len(result.Errors) != 0 {
			t.Fatalf("expected no errors, got: %v", result.Errors)
		}
		if result.File != dataPath {
			t.Fatalf("expected file %q, got %q", dataPath, result.File)
		}
	})

	t.Run("invalid file returns structured errors", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		dir := t.TempDir()
		// missing required "count" field
		path := writeTempFile(t, dir, "bad.yaml", "name: ok\n")

		result, err := ValidatePathResult(path, schemaPath)
		if err != nil {
			t.Fatalf("ValidatePathResult() unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("ValidatePathResult() returned nil result")
		}
		if result.Valid {
			t.Fatal("expected valid: false")
		}
		if len(result.Errors) == 0 {
			t.Fatal("expected at least one error")
		}
		// Each error should have a non-empty message.
		for _, ve := range result.Errors {
			if ve.Message == "" {
				t.Fatalf("expected non-empty message in error: %+v", ve)
			}
		}
	})

	t.Run("invalid yaml has line numbers", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		dir := t.TempDir()
		path := writeTempFile(t, dir, "bad.yaml", "name: 7\ncount: not-a-number\n")

		result, err := ValidatePathResult(path, schemaPath)
		if err != nil {
			t.Fatalf("ValidatePathResult() unexpected error: %v", err)
		}
		if result.Valid {
			t.Fatal("expected valid: false")
		}
		for _, ve := range result.Errors {
			if ve.Line == 0 {
				t.Fatalf("expected non-zero line in error: %+v", ve)
			}
		}
	})

	t.Run("system error: missing schema returns nil result and error", func(t *testing.T) {
		dataPath := fixturePath("yaml", "good.yaml")
		result, err := ValidatePathResult(dataPath, fixturePath("schema", "missing.json"))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if result != nil {
			t.Fatalf("expected nil result on system error, got: %+v", result)
		}
		if !strings.Contains(err.Error(), "load schema") {
			t.Fatalf("expected load schema prefix, got: %v", err)
		}
	})

	t.Run("system error: missing data file returns nil result and error", func(t *testing.T) {
		schemaPath := fixturePath("schema", "valid-object.json")
		result, err := ValidatePathResult(fixturePath("yaml", "missing.yaml"), schemaPath)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if result != nil {
			t.Fatalf("expected nil result on system error, got: %+v", result)
		}
	})
}

func newTestSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	if err := c.AddResource("https://sval.test/schema", map[string]any{
		"type":     "object",
		"required": []any{"name", "count"},
		"properties": map[string]any{
			"name":  map[string]any{"type": "string"},
			"count": map[string]any{"type": "integer"},
		},
	}); err != nil {
		t.Fatalf("AddResource: %v", err)
	}
	schema, err := c.Compile("https://sval.test/schema")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return schema
}

func writeTempFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
	return path
}
