package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
	}
	for _, args := range cmds {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// setupGitChangedRepo creates a repo with one committed (good) file plus
// uncommitted changes/untracked files. Returns the repo dir.
func setupGitChangedRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()

	writeConfigTestFile(t, filepath.Join(dir, "schema.json"),
		`{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`)
	writeConfigTestFile(t, filepath.Join(dir, ".svalconfig.yaml"),
		"rules:\n  - pattern: \"data/**/*.yaml\"\n    schema: \"schema.json\"\n")
	// Committed valid file.
	writeConfigTestFile(t, filepath.Join(dir, "data", "good.yaml"), "name: ok\n")

	gitInit(t, dir)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-q", "-m", "init")

	// Modify committed file (bad), and create one new untracked invalid file.
	writeConfigTestFile(t, filepath.Join(dir, "data", "good.yaml"), "bad: 1\n")
	writeConfigTestFile(t, filepath.Join(dir, "data", "untracked.yaml"), "bad: 2\n")
	return dir
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestValidateChangedIncludesUntracked(t *testing.T) {
	dir := setupGitChangedRepo(t)
	chdir(t, dir)

	_, errOut, err := runValidate(t, "--changed")
	if err == nil {
		t.Fatal("expected validation failure for changed files")
	}
	// Both good.yaml (modified) and untracked.yaml (new) should be reported.
	if !strings.Contains(errOut, "good.yaml") || !strings.Contains(errOut, "untracked.yaml") {
		t.Fatalf("expected both modified and untracked files in output, got: %s", errOut)
	}
}

func TestValidateChangedNoUntracked(t *testing.T) {
	dir := setupGitChangedRepo(t)
	chdir(t, dir)

	_, errOut, err := runValidate(t, "--changed", "--no-untracked")
	if err == nil {
		t.Fatal("expected validation failure for modified file")
	}
	if !strings.Contains(errOut, "good.yaml") {
		t.Fatalf("expected good.yaml in output, got: %s", errOut)
	}
	if strings.Contains(errOut, "untracked.yaml") {
		t.Fatalf("untracked.yaml should be excluded with --no-untracked: %s", errOut)
	}
}

func TestValidateStaged(t *testing.T) {
	dir := setupGitChangedRepo(t)
	chdir(t, dir)

	// Stage the modified good.yaml; untracked stays untracked.
	gitRun(t, dir, "add", "data/good.yaml")

	_, errOut, err := runValidate(t, "--staged-paths")
	if err == nil {
		t.Fatal("expected validation failure for staged file")
	}
	if !strings.Contains(errOut, "good.yaml") {
		t.Fatalf("expected good.yaml in output, got: %s", errOut)
	}
	if strings.Contains(errOut, "untracked.yaml") {
		t.Fatalf("untracked.yaml should not be in --staged set: %s", errOut)
	}
}

func TestValidateChangedStagedExclusive(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	_, _, err := runValidate(t, "--changed", "--staged-paths")
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("expected exclusivity error, got: %v", err)
	}
}

func TestValidateGitFlagsRejectSchema(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	_, _, err := runValidate(t, "--changed", "--schema", "x.json")
	if err == nil || !strings.Contains(err.Error(), "cannot be combined with --schema") {
		t.Fatalf("expected schema-conflict error, got: %v", err)
	}
}

func TestValidateGitFlagsRejectPositional(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	_, _, err := runValidate(t, "--staged-paths", "foo.yaml")
	if err == nil || !strings.Contains(err.Error(), "positional") {
		t.Fatalf("expected positional-conflict error, got: %v", err)
	}
}

func TestValidateBaseRequiresChanged(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	_, _, err := runValidate(t, "--base", "main")
	if err == nil || !strings.Contains(err.Error(), "--base requires --changed") {
		t.Fatalf("expected --base requires --changed, got: %v", err)
	}
}

func TestValidateChangedNotARepo(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	// Need a config so resolveConfig succeeds before git is called.
	writeConfigTestFile(t, filepath.Join(dir, "schema.json"), `{"type":"object"}`)
	writeConfigTestFile(t, filepath.Join(dir, ".svalconfig.yaml"),
		"rules:\n  - pattern: \"**/*.yaml\"\n    schema: \"schema.json\"\n")
	chdir(t, dir)

	_, _, err := runValidate(t, "--changed")
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("expected not-a-git-repo error, got: %v", err)
	}
}
