# Sval schema validator

Sval is a CLI tool for validating structured files against JSON Schema, designed for use in content respositories, documentation sites, data-as-code projects, and CI pipelines.

It supports JSON, YAML, and TOML documents, as well as Markdown frontmatter.

Typical use cases include:

- validating metadata in Markdown notes or content repos
- enforcing structure in YAML/TOML config files
- running schema checks in CI pipelines
- catching invalid data early in mixed-format repositories

Sval is distributed as a single static binary and has no runtime dependencies.

## Scope

Sval validates **structured data** extracted from files:

File type | Behaviour
--- | ---
`.json` | validate full document
`.yaml` / `.yml` | validate full document
`.toml` | validate full document
`.md` | extract and validate frontmatter only

It does **not** validate Markdown body content.

### Design Principles

- Single static binary
- No runtime dependencies
- Predictable behaviour over magic
- Explicit configuration
- Fast startup and execution (CI-friendly)

## Features

- Validation
  - [x] JSON Schema validation
  - [x] Supports JSON, YAML, TOML, Markdown frontmatter
  - [x] Clear, deterministic error reporting
  - [x] Machine-readable output (--json)
- File handling
  - [x] Explicit config file (`--config` flag or auto-discovery)
  - [x] Glob-based rule matching (doublestar patterns per rule)
  - [x] Directory and recursive validation (patterns expand recursively)
  - [x] Ignore support (config `ignore` patterns, no `.gitignore` reading)
- Schema handling
  - [x] Local $ref resolution (relative to schema file)
  - [x] Remote $ref resolution
- CLI ergonomics
  - [x] Fail-fast mode (`--fail-fast`)
  - [x] Verbosity control (`--verbosity`, and shortcuts for `--quiet` / `--verbose` / `--summary` / `--diag`)
  - [x] Stable exit codes for CI
- Git integration
  - [x] Validate changed files (--changed)
  - [x] Validate staged paths (--staged-paths)
- Future features
  - [ ] Watch mode for local development
  - [ ] Option to ignore files ignored by .gitignore
  - [ ] Schema bundling/caching

## Examples

Validate a single file against a schema:

```bash
sval validate ./path/to/file --schema ./path/to/schema.json
```

## Git integration

When run inside a git working tree, Sval can pick up files to validate from git itself. The selected files are filtered through your config's rules (files with no matching rule are skipped).

Validate everything changed in the working tree (modified + untracked, vs `HEAD`):

```bash
sval validate --changed
```

Compare against a different base, or exclude untracked files:

```bash
sval validate --changed --base main
sval validate --changed --no-untracked
```

Validate everything staged in the index:

```bash
sval validate --staged-paths
```

> `--staged-paths` validates the **on-disk** content of staged files, not the indexed blob. If you have unstaged edits to a staged file, those will be seen too. For strict pre-commit checks, wrap the call with `git stash --keep-index` / `git stash pop`, or use the positional-args recipe below.

### Pre-commit hook

For use with [pre-commit](https://pre-commit.com/) or other similar tools, prefer passing the staged file list as positional arguments - pre-commit already isolates the indexed content for you:

```yaml
# .pre-commit-hooks.yaml
- id: sval
  name: sval
  entry: sval validate
  language: system
  pass_filenames: true
```

## Configuration

When no `--config` flag is provided, `sval validate` looks for a config file in the current working directory.

- `.svalconfig.yaml`
- `svalconfig.yaml`
- `.sval.yaml`
- `sval.yaml`

`.yml`, `.json`, and `.toml` extensions are also supported. The first matching file is used.

### Example config file

```yaml
rules:
  - pattern: "docs/**/*.yaml"
    schema: "schemas/doc.json"
  - pattern: "config/*.toml"
    schema: "schemas/config.json"

ignore:
  - "vendor/**"
  - "node_modules/**"
```

You can also reuse the schema associations defined in VS Code settings (`yaml.schemas` in `.vscode/settings.json`) by using the `--config-from-vscode` flag.

## Frontmatter Detection

For Markdown files (`.md`), Sval looks for frontmatter blocks at the top of the file and validates the extracted data.

Within each block, Sval tries to parse content as YAML, then TOML. JSON frontmatter is valid YAML and will be parsed as such. If there are multiple frontmatter blocks, Sval parses each block and validates each resulting document independently.

If no frontmatter block is detected, Sval treats Markdown input as an empty document. Markdown body content is not validated.

## Development

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop) (v4.21.1 or higher) or equivalent
- [Visual Studio Code](https://code.visualstudio.com/) (v.1.80 or higher)
- [Dev containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) (v0.299 or higher)

### Setup

1. Clone this repository
2. Open the repository in Visual Studio Code
3. When you see the prompt "Folder contains a Dev Container configuration file", click "Reopen in Container", or select "Dev Containers: Reopen in Container" from the Command Palette (Ctrl+Shift+P)
