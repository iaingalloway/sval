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
  - [ ] Glob-based rule matching
  - [ ] Directory and recursive validation
  - [ ] Explicit config file
  - [ ] .gitignore-style ignore support
- Schema handling
  - [ ] Local $ref resolution (relative to schema file)
  - [ ] Remote $ref resolution
  - [ ] Schema bundling/caching
- CLI ergonomics
  - [ ] stdin support
  - [ ] validate specific files, globs, or entire repo
  - [ ] fail-fast and summary modes
  - [ ] stable exit codes for CI
  - [ ] verbosity control (--quiet / --verbose)
- Git integration
  - [ ] validate changed files (--changed)
  - [ ] validate staged files (--staged)
- Dev workflow
  - [ ] watch mode for local development

## Examples

Validate a single file against a schema:

```bash
sval validate ./path/to/file --schema ./path/to/schema.json
```

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
