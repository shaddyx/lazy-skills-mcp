# lazy-skill MCP Server

A Go implementation of an **MCP server** that exposes an agent-skill catalog to
LLM agents over the [Model Context Protocol](https://modelcontextprotocol.io).
It enables **progressive disclosure**: agents first see only skill names and
short descriptions, and load a skill's full body, scripts, and reference docs
only on demand — keeping the agent's context window small.

The design is specified in [`spec/lazy-skill-mcp-server.md`](spec/lazy-skill-mcp-server.md).

## Features

- **Skill discovery & search** — browse a filesystem category tree and search
  it by intent with BM25-lite ranking.
- **7 MCP tools** — `list_skills`, `get_skill`, `search_skills`, `skill_tree`,
  `list_skill_files`, `get_skill_file`, `execute_skill_script`.
- **MCP resources** — each skill is exposed as a `lazy-skill://<path>` resource.
- **Scripts & references** — serve and execute skill-bundled scripts; serve
  `references/*.md` reference documents.
- **Path confinement** — all reads and executions are confined to the configured
  root; traversal and symlink escapes are rejected.
- **Single, dependency-light binary** — stdio transport, no network.

## Requirements

- Go 1.26+
- The skill catalog lives in a directory tree (see [Skill catalog](#skill-catalog)).

## Build

```bash
go build ./...
go vet ./...
go test ./...
```

Build the binary:

```bash
go build -o lazy-skill-mcp ./cmd/lazy-skill-mcp
```

## Usage

### MCP server (default)

Run the stdio server. Point it at your skill root (default
`~/.agents/lazy-skills`):

```bash
lazy-skill-mcp --root ~/.agents/lazy-skills
```

Connect any MCP client over stdio. The server completes the standard
`initialize → tools/list → tools/call` handshake.

### Diagnostic CLI

```bash
# Print the catalog tree and exit
lazy-skill-mcp --root ~/.agents/lazy-skills --list

# One-shot search and exit
lazy-skill-mcp --root ~/.agents/lazy-skills --search --query "goroutine leak"

# Version
lazy-skill-mcp --version
```

## Skill catalog

A skill is a directory containing a `SKILL.md` file. Directories without a
`SKILL.md` are categories (or subcategories). A directory may be both a skill
and a category (it has a `SKILL.md` *and* sub-skills).

```
~/.agents/lazy-skills/
  dev/
    golang/
      error-handling/
        SKILL.md          <- skill
      testing/
        SKILL.md          <- skill
        scripts/
          setup.sh        <- script
        references/
          setup-guide.md  <- reference
    web/
      responsive-design/
        SKILL.md          <- skill
  research/
    web-search/
        SKILL.md          <- skill
```

### SKILL.md format

`SKILL.md` is Markdown with an optional YAML frontmatter block:

```markdown
---
name: error-handling          # optional; defaults to directory name
description: Idiomatic Go error handling with wrapping and custom types.
tags: [errors, oops, go]      # optional
version: 1.0                  # optional
scripts:                      # optional; declared scripts for discoverability
  - name: setup.sh
    description: "Initialises the dev environment"
---

# Error Handling

Full skill body / instructions / examples ...
```

- `name` defaults to the directory name; `description` defaults to the first
  non-empty body line.
- Malformed YAML **soft-fails**: the skill is still indexed (with the directory
  name and an empty description) and a warning is logged — one bad file never
  crashes the index.

### Skill files

- **Scripts** — files in the skill directory or its `scripts/` subdirectory with
  an allowed extension (`.sh`, `.bash`, `.py`, `.rb`, `.js`, `.ts`, `.pl`).
- **References** — Markdown (`.md`) files under the skill's `references/`
  subdirectory, served via `get_skill_file`.
- **Auxiliary files** — any other file in the skill directory.

Skill files are resolved from disk at call time (not part of the in-memory
index), so they always reflect the current skill directory.

## Tools

All tools return JSON.

| Tool | Description |
|---|---|
| `list_skills(path?)` | Browse one level of the tree (names + descriptions, no bodies). |
| `get_skill(path, include_frontmatter?)` | Fetch a skill's full body (and optional parsed frontmatter). |
| `search_skills(query, limit?, path?, include_body?)` | BM25-lite keyword search with field weights and snippets. |
| `skill_tree(max_depth?)` | Return the whole catalog as a nested tree. |
| `list_skill_files(path)` | List a skill's scripts, references, and auxiliary files. |
| `get_skill_file(path, file)` | Fetch a single file's contents (e.g. `references/setup-guide.md`). |
| `execute_skill_script(path, file, args?, timeout?)` | Run a skill script; returns stdout/stderr/exit code. |

### Script execution

- The script must resolve to a file **inside** the skill directory.
- Only allowed extensions run; the interpreter is selected by extension
  (`.sh`/`.bash` → `bash`, `.py` → `python3`, `.rb` → `ruby`, `.js` → `node`,
  `.ts` → `tsx`, `.pl` → `perl`).
- The working directory is the skill directory.
- A timeout (default 30 s, max 300 s) is enforced; on timeout the process is
  killed and the result carries `timed_out: true`.

## Configuration

Precedence: **flag > env > default**.

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--root` | `LAZY_SKILLS_ROOT` | `~/.agents/lazy-skills` | Skill catalog root. |
| `--loglevel` | `LAZY_SKILLS_LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error`. |
| `--max-results` | `LAZY_SKILLS_MAX_RESULTS` | `10` | Default search limit. |
| `--max-results-max` | `LAZY_SKILLS_MAX_RESULTS_MAX` | `50` | Hard cap on search limit. |
| — | `LAZY_SKILLS_SCRIPT_TIMEOUT` | `30` | Default script timeout (seconds). |
| — | `LAZY_SKILLS_SCRIPT_EXTS` | `.sh,.bash,.py,.rb,.js,.ts,.pl` | Allowed script extensions. |
| `--allow-empty-root` | — | `false` | Start even if the root is missing/empty. |

## Security

- **Path confinement** — every path argument is normalized and must resolve
  inside the root; `..`, absolute paths, and symlink escapes are rejected.
- **Read-only** — the server only reads skill content; there is no write tool.
- **Script execution** — confined to the skill directory, interpreter
  allow-list only, working directory = skill directory, timeout enforced.
- **No secret redaction** — skill content is passed to the agent verbatim;
  operators are responsible for not putting secrets in the skill tree.

## Project layout

```
lazy-skills/
  cmd/lazy-skill-mcp/main.go   # stdio server entrypoint + CLI
  internal/
    config/                    # env + flag resolution
    skill/                     # Skill type, SKILL.md parsing, frontmatter
    index/                     # tree walk, Node tree, startup index
    search/                    # tokenizer, BM25-lite, search + snippets
    server/                    # MCP wiring, tools, files, execute, resources
    executor/                  # script execution, interpreter dispatch, timeout
  spec/lazy-skill-mcp-server.md
  testdata/skills/             # fixture catalog for tests
```

## Testing

```bash
go test ./...
go test -race ./...
go vet ./...
```

Unit tests cover frontmatter parsing (including soft-fail), index walk and path
confinement, tokenizer/BM25/subtree search, every tool handler and error code,
and script execution (interpreter dispatch, timeout, `not_a_script`,
`missing_interpreter`).

## License

See the repository for license details.
