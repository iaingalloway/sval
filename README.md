# Sval schema validator

Sval is a CLI tool for validating structured files against JSON Schema, designed for use in content repositories, documentation sites, data-as-code projects, and CI pipelines.

It supports JSON, YAML, and TOML documents, as well as Markdown frontmatter. It is distributed as a single static binary with no runtime dependencies, and is designed for fast startup times and predictable, explicit behaviour.

Use cases include:

- validating metadata in Markdown notes or content repos
- enforcing structure in YAML/TOML config files
- running schema checks in CI pipelines
- catching invalid data early in mixed-format repositories

## Installation

Download a binary for your platform from the [releases page](https://github.com/iaingalloway/sval/releases), or install with Go:

```bash
go install github.com/iaingalloway/sval@latest
```

Also available as a Docker image:

```bash
docker run --rm -v "$(pwd):/work" -w /work ghcr.io/iaingalloway/sval:latest validate --config .svalconfig.yaml
```

And as a [devcontainer feature](https://github.com/iaingalloway/features):

```json
    "ghcr.io/iaingalloway/features/sval:1.0.0": {
      "version": "1.0.0"
    }
```

## Scope

Sval validates **structured data** extracted from files:

File type | Behaviour
--- | ---
`.json` | validate full document
`.yaml` / `.yml` | validate full document
`.toml` | validate full document
`.md` | extract and validate frontmatter only

It does **not** validate Markdown body content.

### Features

- Validation
  - JSON Schema validation
  - JSON, YAML, TOML, and Markdown frontmatter
  - Clear, deterministic error reporting
  - Machine-readable output (`--json`)
- File handling
  - Explicit config file (`--config` flag or auto-discovery)
  - Glob-based rule matching (doublestar patterns per rule)
  - Recursive validation (patterns expand recursively)
  - Ignore patterns from config file
- Schema handling
  - Local `$ref` resolution (relative to schema file)
  - Remote `$ref` resolution
- CLI ergonomics
  - Fail-fast mode (`--fail-fast`)
  - Verbosity control (`--verbosity`, with shortcuts `--quiet`, `--verbose`, `--summary`, `--diag`)
  - Stable exit codes
- Git integration
  - Validate changed files (`--changed`)
  - Validate staged paths (`--staged-paths`)

## Exit codes

Code | Meaning
--- | ---
`0` | All validated files are valid, or no files matched any rule
`1` | One or more files failed validation
`2` | Usage error or unexpected failure

## Examples

Validate a single file against a schema:

```bash
sval validate ./path/to/file.yaml --schema ./path/to/schema.json
```

Validate all files matched by a config file:

```bash
sval validate --config .svalconfig.yaml
```

When no `--config` flag is provided, sval looks for a config file in the current directory automatically - see [Configuration](#configuration).

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

> `--staged-paths` validates the **on-disk** content of staged files, not the indexed blob. If you have unstaged edits to a staged file, those will be seen too. For strict pre-commit checks, see the [Pre-commit hook](#pre-commit-hook) section below, or wrap the call with `git stash --keep-index` / `git stash pop`.

### Pre-commit hook

For use with [pre-commit](https://pre-commit.com/) or other tools that pass staged filenames as arguments, sval works without any special configuration - pre-commit already isolates the indexed content for you:

```yaml
# .pre-commit-config.yaml
repos:
  - repo: local
    hooks:
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
4. This repository uses `just` as a task runner. Run `just --list` to see available recipes
