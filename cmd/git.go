package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitRepoRoot returns the absolute path of the git repository containing cwd,
// or an error if cwd is not inside a git working tree.
func gitRepoRoot(cwd string) (string, error) {
	out, err := runGit(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository (or git not on PATH): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitChangedFiles returns absolute paths of files that differ between the
// working tree and base (typically "HEAD"). When includeUntracked is true,
// untracked files (respecting .gitignore via --exclude-standard) are also
// included. Deletions are excluded.
func gitChangedFiles(cwd, base string, includeUntracked bool) ([]string, error) {
	root, err := gitRepoRoot(cwd)
	if err != nil {
		return nil, err
	}

	out, err := runGit(cwd, "diff", "--name-only", "--diff-filter=ACMR", "-z", base)
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}
	files := splitNUL(out)

	if includeUntracked {
		out, err := runGit(cwd, "ls-files", "--others", "--exclude-standard", "-z")
		if err != nil {
			return nil, fmt.Errorf("git ls-files failed: %w", err)
		}
		files = append(files, splitNUL(out)...)
	}

	return absolutise(root, files), nil
}

// gitStagedFiles returns absolute paths of files staged in the index relative
// to HEAD. Deletions are excluded.
func gitStagedFiles(cwd string) ([]string, error) {
	root, err := gitRepoRoot(cwd)
	if err != nil {
		return nil, err
	}

	out, err := runGit(cwd, "diff", "--name-only", "--cached", "--diff-filter=ACMR", "-z", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("git diff --cached failed: %w", err)
	}
	return absolutise(root, splitNUL(out)), nil
}

func runGit(cwd string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

// splitNUL splits NUL-terminated git output into a slice of paths, dropping
// the trailing empty entry that follows the final NUL.
func splitNUL(b []byte) []string {
	s := strings.TrimRight(string(b), "\x00")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\x00")
}

func absolutise(root string, paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		out = append(out, filepath.Join(root, p))
	}
	return out
}
