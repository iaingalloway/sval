package loader

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixturePath(parts ...string) string {
	items := append([]string{"testdata"}, parts...)
	return filepath.Join(items...)
}

func TestLoadFrontmatter(t *testing.T) {
	dir := t.TempDir()

	t.Run("yaml success", func(t *testing.T) {
		path := writeTempFile(t, dir, "post.md", "---\nname: example\ncount: 2\n---\ncontent")
		doc, err := LoadFrontmatter(path)
		if err != nil {
			t.Fatalf("LoadFrontmatter() unexpected error: %v", err)
		}
		data, ok := doc.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data, got %T", doc.Data)
		}
		if name, ok := data["name"].(string); !ok || name != "example" {
			t.Fatalf("unexpected name: %#v", data["name"])
		}
		if count, ok := data["count"].(int); !ok || count != 2 {
			t.Fatalf("unexpected count: %#v", data["count"])
		}
		pos, ok := doc.PointerPosition("/count")
		if !ok {
			t.Fatalf("expected pointer position for /count")
		}
		if pos.Line != 3 {
			t.Fatalf("expected line 3 for /count, got %d", pos.Line)
		}
	})

	t.Run("json success", func(t *testing.T) {
		path := writeTempFile(t, dir, "post-json.md", "---\n{\"name\":\"example\",\"count\":2}\n---\ncontent")
		doc, err := LoadFrontmatter(path)
		if err != nil {
			t.Fatalf("LoadFrontmatter() unexpected error: %v", err)
		}
		data, ok := doc.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data, got %T", doc.Data)
		}
		if name, ok := data["name"].(string); !ok || name != "example" {
			t.Fatalf("unexpected name: %#v", data["name"])
		}
	})

	t.Run("multiple blocks are separate documents", func(t *testing.T) {
		path := writeTempFile(t, dir, "post-multi.md", "---\nname: first\n---\n+++\ncount = 7\n+++\ncontent")
		doc, err := LoadFrontmatter(path)
		if err != nil {
			t.Fatalf("LoadFrontmatter() unexpected error: %v", err)
		}
		data, ok := doc.Data.([]any)
		if !ok {
			t.Fatalf("expected sequence data, got %T", doc.Data)
		}
		if len(data) != 2 {
			t.Fatalf("expected 2 documents, got %d", len(data))
		}
		doc1, ok := data[0].(map[string]any)
		if !ok {
			t.Fatalf("expected first document map, got %T", data[0])
		}
		doc2, ok := data[1].(map[string]any)
		if !ok {
			t.Fatalf("expected second document map, got %T", data[1])
		}
		if name, ok := doc1["name"].(string); !ok || name != "first" {
			t.Fatalf("unexpected name: %#v", doc1["name"])
		}
		if count, ok := doc2["count"].(int64); !ok || count != 7 {
			t.Fatalf("unexpected count: %#v", doc2["count"])
		}
	})

	t.Run("toml success", func(t *testing.T) {
		path := writeTempFile(t, dir, "post-toml.md", "+++\nname = \"example\"\ncount = 2\n+++\ncontent")
		doc, err := LoadFrontmatter(path)
		if err != nil {
			t.Fatalf("LoadFrontmatter() unexpected error: %v", err)
		}
		data, ok := doc.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data, got %T", doc.Data)
		}
		if name, ok := data["name"].(string); !ok || name != "example" {
			t.Fatalf("unexpected name: %#v", data["name"])
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadFrontmatter(filepath.Join(dir, "missing.md"))
		if err == nil || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected os.ErrNotExist, got %v", err)
		}
	})

	t.Run("invalid frontmatter", func(t *testing.T) {
		path := writeTempFile(t, dir, "broken.md", "---\nname: [\n---\nbody")
		_, err := LoadFrontmatter(path)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse frontmatter") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()

	t.Run("success", func(t *testing.T) {
		path := writeTempFile(t, dir, "note.yaml", "name: example\ncount: 3\n")
		doc, err := LoadYAML(path)
		if err != nil {
			t.Fatalf("LoadYAML() unexpected error: %v", err)
		}
		data, ok := doc.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data, got %T", doc.Data)
		}
		if name, ok := data["name"].(string); !ok || name != "example" {
			t.Fatalf("unexpected name: %#v", data["name"])
		}
		if count, ok := data["count"].(int); !ok || count != 3 {
			t.Fatalf("unexpected count: %#v", data["count"])
		}
		pos, ok := doc.PointerPosition("/count")
		if !ok {
			t.Fatalf("expected pointer position for /count")
		}
		if pos.Line != 2 {
			t.Fatalf("expected line 2 for /count, got %d", pos.Line)
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		path := writeTempFile(t, dir, "broken.yaml", "name: [broken\n")
		_, err := LoadYAML(path)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse yaml") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("multiple documents are separate", func(t *testing.T) {
		path := writeTempFile(t, dir, "multi.yaml", "name: first\n---\ncount: 9\n")
		doc, err := LoadYAML(path)
		if err != nil {
			t.Fatalf("LoadYAML() unexpected error: %v", err)
		}
		data, ok := doc.Data.([]any)
		if !ok {
			t.Fatalf("expected sequence data, got %T", doc.Data)
		}
		if len(data) != 2 {
			t.Fatalf("expected 2 documents, got %d", len(data))
		}
		doc1, ok := data[0].(map[string]any)
		if !ok {
			t.Fatalf("expected first document map, got %T", data[0])
		}
		doc2, ok := data[1].(map[string]any)
		if !ok {
			t.Fatalf("expected second document map, got %T", data[1])
		}
		if name, ok := doc1["name"].(string); !ok || name != "first" {
			t.Fatalf("unexpected name: %#v", doc1["name"])
		}
		if count, ok := doc2["count"].(int); !ok || count != 9 {
			t.Fatalf("unexpected count: %#v", doc2["count"])
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadYAML(filepath.Join(dir, "missing.yaml"))
		if err == nil || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected os.ErrNotExist, got %v", err)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := writeTempFile(t, dir, "empty.yaml", "")
		doc, err := LoadYAML(path)
		if err != nil {
			t.Fatalf("LoadYAML() unexpected error: %v", err)
		}
		if doc.Data != nil {
			t.Fatalf("expected nil data for empty file, got %#v", doc.Data)
		}
	})
}

func TestLoadJSON(t *testing.T) {
	dir := t.TempDir()

	t.Run("success", func(t *testing.T) {
		path := writeTempFile(t, dir, "note.json", `{"name":"example","count":3}`)
		doc, err := LoadJSON(path)
		if err != nil {
			t.Fatalf("LoadJSON() unexpected error: %v", err)
		}
		data, ok := doc.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data, got %T", doc.Data)
		}
		if name, ok := data["name"].(string); !ok || name != "example" {
			t.Fatalf("unexpected name: %#v", data["name"])
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		path := writeTempFile(t, dir, "broken.json", `{"name":}`)
		_, err := LoadJSON(path)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse json") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadJSON(filepath.Join(dir, "missing.json"))
		if err == nil || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected os.ErrNotExist, got %v", err)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := writeTempFile(t, dir, "empty.json", "")
		doc, err := LoadJSON(path)
		if err != nil {
			t.Fatalf("LoadJSON() unexpected error: %v", err)
		}
		if doc.Data != nil {
			t.Fatalf("expected nil data for empty file, got %#v", doc.Data)
		}
	})
}

func TestLoadTOML(t *testing.T) {
	dir := t.TempDir()

	t.Run("success", func(t *testing.T) {
		path := writeTempFile(t, dir, "note.toml", "name = \"example\"\ncount = 3\n")
		doc, err := LoadTOML(path)
		if err != nil {
			t.Fatalf("LoadTOML() unexpected error: %v", err)
		}
		data, ok := doc.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data, got %T", doc.Data)
		}
		if name, ok := data["name"].(string); !ok || name != "example" {
			t.Fatalf("unexpected name: %#v", data["name"])
		}
	})

	t.Run("invalid toml", func(t *testing.T) {
		path := writeTempFile(t, dir, "broken.toml", "name = \"broken\n")
		_, err := LoadTOML(path)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse toml") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadTOML(filepath.Join(dir, "missing.toml"))
		if err == nil || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected os.ErrNotExist, got %v", err)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := writeTempFile(t, dir, "empty.toml", "")
		doc, err := LoadTOML(path)
		if err != nil {
			t.Fatalf("LoadTOML() unexpected error: %v", err)
		}
		if doc.Data != nil {
			t.Fatalf("expected nil data for empty file, got %#v", doc.Data)
		}
	})
}

func TestLoadFixtures(t *testing.T) {
	t.Run("yaml file with mixed value types across docs", func(t *testing.T) {
		path := fixturePath("yaml", "multi-docs-mixed-types.yaml")
		doc, err := LoadYAML(path)
		if err != nil {
			t.Fatalf("LoadYAML() unexpected error: %v", err)
		}
		seq, ok := doc.Data.([]any)
		if !ok {
			t.Fatalf("expected sequence data, got %T", doc.Data)
		}
		if len(seq) != 2 {
			t.Fatalf("expected 2 documents, got %d", len(seq))
		}
		m1, ok := seq[0].(map[string]any)
		if !ok {
			t.Fatalf("expected map for doc 1, got %T", seq[0])
		}
		if value, ok := m1["value"].(int); !ok || value != 1 {
			t.Fatalf("expected int value 1 in doc 1, got %#v", m1["value"])
		}
		m2, ok := seq[1].(map[string]any)
		if !ok {
			t.Fatalf("expected map for doc 2, got %T", seq[1])
		}
		if value, ok := m2["value"].(string); !ok || value != "not-a-number" {
			t.Fatalf("expected string value \"not-a-number\" in doc 2, got %#v", m2["value"])
		}
	})

	t.Run("md file with multi-block error in second block", func(t *testing.T) {
		path := fixturePath("frontmatter", "multi-blocks-error.md")
		_, err := LoadFrontmatter(path)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse frontmatter") {
			t.Fatalf("expected parse error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "doc 2") {
			t.Fatalf("expected doc index in error, got: %v", err)
		}
	})
	t.Run("leading blank lines before frontmatter", func(t *testing.T) {
		path := fixturePath("frontmatter", "leading-blank-lines.md")
		doc, err := LoadFrontmatter(path)
		if err != nil {
			t.Fatalf("LoadFrontmatter() unexpected error: %v", err)
		}

		data, ok := doc.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data, got %T", doc.Data)
		}
		if name, ok := data["name"].(string); !ok || name != "fixture" {
			t.Fatalf("unexpected name: %#v", data["name"])
		}

		pos, ok := doc.PointerPosition("/count")
		if !ok {
			t.Fatalf("expected pointer position for /count")
		}
		if pos.Line != 5 {
			t.Fatalf("expected line 5 for /count, got %d", pos.Line)
		}
	})

	t.Run("bom frontmatter", func(t *testing.T) {
		path := fixturePath("frontmatter", "bom-frontmatter.md")
		doc, err := LoadFrontmatter(path)
		if err != nil {
			t.Fatalf("LoadFrontmatter() unexpected error: %v", err)
		}

		data, ok := doc.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data, got %T", doc.Data)
		}
		if count, ok := data["count"].(int); !ok || count != 8 {
			t.Fatalf("unexpected count: %#v", data["count"])
		}

		pos, ok := doc.PointerPosition("/count")
		if !ok {
			t.Fatalf("expected pointer position for /count")
		}
		if pos.Line != 3 {
			t.Fatalf("expected line 3 for /count, got %d", pos.Line)
		}
	})

	t.Run("no frontmatter returns empty document", func(t *testing.T) {
		path := fixturePath("frontmatter", "no-frontmatter.md")
		doc, err := LoadFrontmatter(path)
		if err != nil {
			t.Fatalf("LoadFrontmatter() unexpected error: %v", err)
		}
		if doc == nil {
			t.Fatalf("expected non-nil document")
		}
		if doc.Data != nil {
			t.Fatalf("expected nil data for no frontmatter, got %#v", doc.Data)
		}
	})

	t.Run("unterminated frontmatter returns parse error", func(t *testing.T) {
		path := fixturePath("frontmatter", "unterminated-frontmatter.md")
		_, err := LoadFrontmatter(path)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse frontmatter") {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(err.Error(), "unterminated frontmatter") {
			t.Fatalf("missing unterminated detail: %v", err)
		}
	})

	t.Run("json file", func(t *testing.T) {
		path := fixturePath("json", "good.json")
		doc, err := LoadJSON(path)
		if err != nil {
			t.Fatalf("LoadJSON() unexpected error: %v", err)
		}
		data, ok := doc.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data, got %T", doc.Data)
		}
		if name, ok := data["name"].(string); !ok || name != "fixture" {
			t.Fatalf("unexpected name: %#v", data["name"])
		}
		if count, ok := data["count"].(float64); !ok || count != 4 {
			t.Fatalf("unexpected count: %#v", data["count"])
		}
	})

	t.Run("toml file", func(t *testing.T) {
		path := fixturePath("toml", "good.toml")
		doc, err := LoadTOML(path)
		if err != nil {
			t.Fatalf("LoadTOML() unexpected error: %v", err)
		}
		data, ok := doc.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data, got %T", doc.Data)
		}
		if name, ok := data["name"].(string); !ok || name != "fixture" {
			t.Fatalf("unexpected name: %#v", data["name"])
		}
		if count, ok := data["count"].(int64); !ok || count != 4 {
			t.Fatalf("unexpected count: %#v", data["count"])
		}
	})

	t.Run("md file with toml-only frontmatter", func(t *testing.T) {
		path := fixturePath("frontmatter", "toml-only.md")
		doc, err := LoadFrontmatter(path)
		if err != nil {
			t.Fatalf("LoadFrontmatter() unexpected error: %v", err)
		}
		data, ok := doc.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data, got %T", doc.Data)
		}
		if name, ok := data["name"].(string); !ok || name != "toml-fixture" {
			t.Fatalf("unexpected name: %#v", data["name"])
		}
		if count, ok := data["count"].(int64); !ok || count != 5 {
			t.Fatalf("unexpected count: %#v", data["count"])
		}
	})

	t.Run("md file with ---toml delimiter", func(t *testing.T) {
		path := fixturePath("frontmatter", "toml-dash.md")
		doc, err := LoadFrontmatter(path)
		if err != nil {
			t.Fatalf("LoadFrontmatter() unexpected error: %v", err)
		}
		data, ok := doc.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data, got %T", doc.Data)
		}
		if name, ok := data["name"].(string); !ok || name != "toml-dash-fixture" {
			t.Fatalf("unexpected name: %#v", data["name"])
		}
		if count, ok := data["count"].(int64); !ok || count != 7 {
			t.Fatalf("unexpected count: %#v", data["count"])
		}
	})

	t.Run("md file with json frontmatter parsed as yaml", func(t *testing.T) {
		path := fixturePath("frontmatter", "json-frontmatter.md")
		doc, err := LoadFrontmatter(path)
		if err != nil {
			t.Fatalf("LoadFrontmatter() unexpected error: %v", err)
		}
		data, ok := doc.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data, got %T", doc.Data)
		}
		if name, ok := data["name"].(string); !ok || name != "json-fixture" {
			t.Fatalf("unexpected name: %#v", data["name"])
		}
		if count, ok := data["count"].(int); !ok || count != 6 {
			t.Fatalf("unexpected count: %#v", data["count"])
		}
	})

	t.Run("md file with multiple frontmatter blocks in different formats", func(t *testing.T) {
		path := fixturePath("frontmatter", "multi-blocks-mixed.md")
		doc, err := LoadFrontmatter(path)
		if err != nil {
			t.Fatalf("LoadFrontmatter() unexpected error: %v", err)
		}
		seq, ok := doc.Data.([]any)
		if !ok {
			t.Fatalf("expected sequence data, got %T", doc.Data)
		}
		if len(seq) != 3 {
			t.Fatalf("expected 3 blocks, got %d", len(seq))
		}
		m1, ok := seq[0].(map[string]any)
		if !ok || m1["name"] != "block1" {
			t.Fatalf("unexpected first block: %#v", seq[0])
		}
		m2, ok := seq[1].(map[string]any)
		if !ok || m2["name"] != "block2" {
			t.Fatalf("unexpected second block: %#v", seq[1])
		}
		m3, ok := seq[2].(map[string]any)
		if !ok || m3["name"] != "block3" {
			t.Fatalf("unexpected third block: %#v", seq[2])
		}
	})
}

func writeTempFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
	return path
}
