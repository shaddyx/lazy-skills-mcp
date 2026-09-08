# Lazy-Skill MCP Server — Specification

| Field | Value |
|---|---|
| Status | Draft |
| Date | 2026-09-08 |
| Language | Go |
| Transport | stdio (default) |
| Skill root | `~/.agents/lazy-skills` (overridable) |

---

## 1. Overview

The **lazy-skill MCP server** exposes an agent-skill catalog to LLM agents over the Model Context Protocol (MCP), enabling **progressive disclosure** of skills instead of loading every skill's full text into the agent's context up front.

Skills live in a filesystem **category tree**:

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

- Each **directory** is a **category** (or subcategory).
- A directory that **contains a `SKILL.md`** is a **skill** (leaf).
- A skill directory may also contain **scripts** (shell, Python, etc.), **reference documents** (Markdown under `references/`), and other auxiliary files. These are served on demand, and scripts are executed on demand.
- "Lazy" = the agent first sees only the category/skill **names and short descriptions**, and loads a skill's **full body and scripts only on demand**. This keeps the context window small and lets the agent navigate the catalog the way a human would browse a filesystem.

---

## 2. Goals

- Serve the skill catalog to MCP clients over stdio.
- Expose only cheap metadata (names, descriptions) until a skill is explicitly requested.
- Provide keyword search across the catalog so agents can jump directly to a relevant skill.
- Serve skill-bundled scripts, reference documents, and auxiliary files on demand.
- Execute skill scripts and return stdout/stderr/exit code.
- Be a single, dependency-light Go binary that is easy to run and test.
- Be safe: confine all reads and executions to the configured root; never follow paths outside it.

## Non-Goals (v1)

- No authentication / multi-tenant access control.
- No network transport (stdio only; HTTP/SSE out of scope for v1).
- No semantic (embedding/vector) search — lexical search only in v1.
- No write API (skills are read-only to the server).
- No arbitrary command execution — only scripts that live inside a skill directory and match an allowed extension.
- No bundling/packaging tooling (a separate concern, out of scope).

---

## 3. Domain Model

### 3.1 Skill path

A skill is addressed by its **path relative to the root**, using `/` as the separator (POSIX-style on all platforms):

```
dev/golang/error-handling
```

Path rules:
- Relative, forward-slash separated.
- No leading `/`, no `..`, no absolute components.
- Case-sensitive on the underlying OS (see §12).
- A path resolves to a **skill only if the final segment is a directory containing `SKILL.md`**.

### 3.2 Node classification

For every directory under the root:

- **Category** = a directory with **no** `SKILL.md` but with at least one subdirectory (or leaf `SKILL.md` children).
- **Skill** = a directory that **contains a `SKILL.md`** file.
- A directory may be **both**: it has a `SKILL.md` *and* subdirectories. Such a node is a skill **with** sub-skills. The browse tool must surface both its own skill and its children.

### 3.3 Skill files

A skill directory may contain auxiliary files alongside `SKILL.md`:

```
dev/golang/error-handling/
  SKILL.md
  scripts/
    setup.sh
    run-linters.sh
  references/
    wrapping-examples.md
  examples.md
```

- **Script** = a file inside a skill directory (or its `scripts/` subdirectory) with an allowed extension: `.sh`, `.bash`, `.py`, `.rb`, `.js`, `.ts`, `.pl`.
- **Reference** = a Markdown (`.md`) file inside the skill's `references/` subdirectory. References hold supplementary detail (deep-dive guides, worked examples, API notes) that a skill body points the agent to instead of inlining it. They are fetched via `get_skill_file` (e.g. `references/wrapping-examples.md`) and listed by `list_skill_files` with `is_reference: true`.
- **Auxiliary file** = any other file in the skill directory (templates, data, etc.).
- Files are addressed by their **relative path within the skill directory**, e.g. `scripts/setup.sh` or `references/wrapping-examples.md`.
- Skill files are **not part of the in-memory index**; `list_skill_files` / `get_skill_file` / `execute_skill_script` resolve them from disk at call time, so they always reflect the current skill directory.

### 3.4 Index (in-memory)

```
root/
  └── Node tree (categories + skills)
  └── map[skillPath]Skill   // fast lookup
  └── InvertedIndex          // for search
```

Built once at startup. No background refresh in v1; restart the server to pick up catalog changes.

---

## 4. SKILL.md Format

`SKILL.md` is a Markdown file with an optional YAML frontmatter block followed by a body.

```markdown
---
name: error-handling          # optional; defaults to directory name
description: Idiomatic Go error handling with wrapping and custom types.
tags: [errors, oops, go]      # optional
version: 1.0                  # optional
---

# Error Handling

Full skill body / instructions / examples ...
```

### 4.1 Frontmatter fields

| Field | Required | Type | Notes |
|---|---|---|---|
| `name` | no | string | Falls back to the directory name (slug). |
| `description` | no | string | One-line summary; primary discovery signal. Falls back to first non-empty line of the body, then to `""`. |
| `tags` | no | []string | Indexed for search. |
| `version` | no | string | Informational. |
| `scripts` | no | []object | Optional list of declared scripts with `name` and `description`. Used for discoverability; not required for execution. |
| *(any other)* | — | — | Parsed and stored in `FrontMatter` map; ignored by search unless tagged. |

Example `scripts` field:

```yaml
scripts:
  - name: setup.sh
    description: "Initialises the dev environment"
  - name: run_tests.py
    description: "Runs the test suite with coverage"
```

### 4.2 Parsing rules

- Frontmatter must start at the **first line** with `---` and be closed by a matching `---` line.
- If no frontmatter, the entire file is the body; `name` = directory name; `description` = first non-empty line.
- Malformed YAML in frontmatter → **soft-fail**: the skill is still indexed with `name` = directory name and `description` = `""`, and a warning is logged. Never crash the whole index on one bad file.
- Empty body → skill is indexed but marked `Content: ""` (still listable/searchable by name/description).

---

## 5. Progressive Disclosure (the "Lazy" Model)

The core value proposition is that the agent **does not pay the context cost of every skill**. The intended interaction loop:

1. **Discover** — `list_skills(path="")` returns only the top-level categories and their immediate skill children (names + descriptions, no bodies).
2. **Drill down** — `list_skills(path="dev/golang")` returns that category's children.
3. **Fetch** — `get_skill(path="dev/golang/error-handling")` returns the full body **only for the chosen skill**.
4. **Shortcut** — `search_skills(query="goroutine leak")` jumps straight to a ranked match without manual navigation.
5. **Scripts & references** — `list_skill_files(path=...)` reveals a skill's bundled scripts and reference docs; `get_skill_file(...)` fetches a script or a `references/*.md` doc; `execute_skill_script(...)` runs a script. These load only when needed.

Only the steps the agent actually takes load content, so the agent's context stays proportional to the skills it actually uses.

---

## 6. MCP Server

### 6.1 Transport

- **stdio** (default and only transport in v1).
- Reads JSON-RPC messages from `stdin`, writes to `stdout`. Diagnostics → `stderr` only.

### 6.2 Capabilities

- `tools` — always.
- `resources` — optional (see §8); enabled by default, cheap to provide.

### 6.3 Protocol version

Implement the current stable MCP JSON-RPC surface (initialize, tools/list, tools/call, resources/list, resources/read, ping). Keep the dependency on a maintained Go MCP SDK (e.g. `github.com/modelcontextprotocol/go-sdk` or equivalent) so protocol evolution is contained.

---

## 7. Tools

All tools return JSON as their `content` (structured JSON in a single text block, or as MCP structured content where supported).

### 7.1 `list_skills`

Browse one level of the tree.

**Input schema**

```json
{
  "type": "object",
  "properties": {
    "path": { "type": "string", "description": "Category path relative to root; \"\" for root. e.g. \"dev/golang\".", "default": "" }
  }
}
```

**Output (JSON)**

```json
{
  "path": "dev/golang",
  "self": {
    "name": "golang",
    "description": "Go language skills.",
    "type": "category"
  },
  "skills": [
    { "name": "testing", "path": "dev/golang/testing", "description": "Production-ready Go tests..." }
  ],
  "categories": [
    { "name": "error-handling", "path": "dev/golang/error-handling", "description": "" }
  ]
}
```

- A directory that is both a skill and has children appears in `skills` (its own skill) **and** its children appear under `categories`/`skills` of that node when drilled into.
- Entries are sorted alphabetically within `skills` and `categories`.
- Empty `path` = root.

**Errors**
- Unknown path → tool error `not_found` with the resolved prefix that exists.
- Path that is a skill but user passed it as a category → still valid; returns its children (may be empty).

### 7.2 `get_skill`

Fetch the full skill.

**Input schema**

```json
{
  "type": "object",
  "required": ["path"],
  "properties": {
    "path": { "type": "string", "description": "Skill path, e.g. \"dev/golang/error-handling\"." },
    "include_frontmatter": { "type": "boolean", "default": false, "description": "Include parsed frontmatter object." }
  }
}
```

**Output (JSON)**

```json
{
  "path": "dev/golang/error-handling",
  "name": "error-handling",
  "description": "Idiomatic Go error handling...",
  "tags": ["errors", "oops", "go"],
  "version": "1.0",
  "content": "# Error Handling\n\nFull body...",
  "frontmatter": { "name": "error-handling", "description": "...", "tags": ["..."] }
}
```

- `content` is the raw markdown body (frontmatter stripped).
- `frontmatter` present only when `include_frontmatter` is true.

**Errors**
- `not_found` — path is not a skill.
- `not_skill` — path exists but has no `SKILL.md` (a pure category).

### 7.3 `search_skills`

Lexical search across the catalog.

**Input schema**

```json
{
  "type": "object",
  "required": ["query"],
  "properties": {
    "query": { "type": "string", "description": "Free-text query (e.g. \"goroutine leak detection\")." },
    "limit": { "type": "integer", "default": 10, "minimum": 1, "maximum": 50 },
    "path": { "type": "string", "description": "Optional: restrict search to a subtree." },
    "include_body": { "type": "boolean", "default": true, "description": "Include skill body text in the index." }
  }
}
```

**Output (JSON)**

```json
{
  "query": "goroutine leak detection",
  "total": 42,
  "results": [
    {
      "path": "dev/golang/testing",
      "name": "testing",
      "description": "Parallel tests, fuzzing, goleak...",
      "score": 3.42,
      "matched_fields": ["description", "body"],
      "snippet": "...use goleak to detect goroutine leaks after each test..."
    }
  ]
}
```

- Ranking: BM25-lite with field weights (name > description > tags > body). See §9.
- `snippet`: up to ~240 chars centered on the best matching token in the body (if a body match exists); otherwise the description.
- `total` = number of skills with score > 0 (before `limit`).

**Errors**
- Empty/whitespace `query` → tool error `invalid_query`.

### 7.4 `skill_tree` (optional, nice-to-have)

Return the whole catalog as a compact nested structure (names only, no bodies), for tooling/debugging or a one-shot "what's available" view.

**Input schema**

```json
{
  "type": "object",
  "properties": {
    "max_depth": { "type": "integer", "description": "Optional cap on tree depth. 0 = unlimited." }
  }
}
```

**Output**: nested `{ name, path, type, children }` tree.

### 7.5 `list_skill_files`

List the non-`SKILL.md` files (scripts, references, and auxiliary files) in a skill directory.

**Input schema**

```json
{
  "type": "object",
  "required": ["path"],
  "properties": {
    "path": { "type": "string", "description": "Skill path, e.g. \"dev/golang/error-handling\"." }
  }
}
```

**Output (JSON)**

```json
{
  "path": "dev/golang/error-handling",
  "files": [
    { "name": "examples.md", "size": 2048, "is_script": false, "is_reference": false, "executable": false },
    { "name": "references/wrapping-examples.md", "size": 4321, "is_script": false, "is_reference": true, "executable": false },
    { "name": "scripts/run-linters.sh", "size": 890, "is_script": true, "is_reference": false, "executable": true },
    { "name": "scripts/setup.sh", "size": 1234, "is_script": true, "is_reference": false, "executable": true }
  ]
}
```

- `name` = file path relative to the skill directory.
- `is_script` = `true` when the file has an allowed script extension (§3.3).
- `is_reference` = `true` when the file is a `.md` under `references/` (§3.3).
- `SKILL.md` is excluded. Entries sorted by `name`.

**Errors**
- `not_found` — path doesn't exist.
- `not_skill` — path exists but has no `SKILL.md`.

### 7.6 `get_skill_file`

Fetch the contents of a single file from a skill directory.

**Input schema**

```json
{
  "type": "object",
  "required": ["path", "file"],
  "properties": {
    "path": { "type": "string", "description": "Skill path, e.g. \"dev/golang/error-handling\"." },
    "file": { "type": "string", "description": "File path relative to the skill directory, e.g. \"scripts/setup.sh\"." }
  }
}
```

**Output (JSON)**

```json
{
  "path": "dev/golang/error-handling",
  "file": "scripts/setup.sh",
  "size": 1234,
  "content": "#!/usr/bin/env bash\nset -euo pipefail\n..."
}
```

- `content` = raw file text.
- Reference documents (`.md` under `references/`, §3.3) are a first-class fetch target, e.g. `get_skill_file(path, "references/wrapping-examples.md")`.
- Binary files (NUL byte / non-UTF-8) → tool error `binary_file`.

**Errors**
- `not_found` — skill or file not found.
- `not_skill` — path is not a skill.
- `binary_file` — file is not decodable as text.

### 7.7 `execute_skill_script`

Execute a script inside a skill directory and return the result.

**Input schema**

```json
{
  "type": "object",
  "required": ["path", "file"],
  "properties": {
    "path": { "type": "string", "description": "Skill path, e.g. \"dev/golang/error-handling\"." },
    "file": { "type": "string", "description": "Script path relative to the skill directory, e.g. \"scripts/setup.sh\"." },
    "args": { "type": "array", "items": { "type": "string" }, "description": "Optional positional arguments passed to the script." },
    "timeout": { "type": "integer", "default": 30, "minimum": 1, "maximum": 300, "description": "Timeout in seconds." }
  }
}
```

**Output (JSON)**

```json
{
  "path": "dev/golang/error-handling",
  "file": "scripts/setup.sh",
  "exit_code": 0,
  "stdout": "Setting up dev environment...\nDone.",
  "stderr": "",
  "timed_out": false,
  "duration_ms": 412
}
```

**Execution model**
- The script path must resolve to a **file inside the skill directory** (path confinement, §12).
- The file must have an **allowed script extension** (`.sh`, `.bash`, `.py`, `.rb`, `.js`, `.ts`, `.pl`); otherwise `not_a_script`.
- **Interpreter selection** by extension: `.sh`/`.bash` → `bash`, `.py` → `python3`, `.rb` → `ruby`, `.js` → `node`, `.ts` → `tsx`, `.pl` → `perl`. Missing interpreter → `missing_interpreter`.
- **Working directory** = the skill directory (so relative paths inside the script resolve within it).
- **Timeout** enforced via `context.WithTimeout`; on timeout the process is killed and the response carries `timed_out: true` (a valid result, not an error).

**Errors**
- `not_found` — skill or file not found.
- `not_skill` — path is not a skill.
- `not_a_script` — file has a non-allowed extension.
- `missing_interpreter` — required interpreter not on `PATH`.
- `execution_failed` — process could not start (e.g. permission); error in `stderr`, `exit_code: -1`.

---

## 8. Resources (optional, enabled by default)

Expose each skill as an MCP **resource** so clients that prefer the resource model can read skills directly.

- URI scheme: `lazy-skill://<path>`, e.g. `lazy-skill://dev/golang/error-handling`.
- `resources/list` → one resource per skill, `name` = skill name, `uri` = `lazy-skill://<path>`, `mimeType = text/markdown`.
- `resources/read(uri)` → the SKILL.md body (or full file).
- `resources/template` → `lazy-skill://*/**` (parameterized) for clients that want to enumerate.

This is additive to the tools; a client may use either. If keeping the surface minimal, resources may be deferred to M2 (see §16).

---

## 9. Search & Indexing (v1 = lexical)

### 9.1 Indexed fields

Per skill:
- `name` (weight 3.0)
- `description` (weight 2.0)
- `tags` (weight 2.0, flattened)
- `body` (weight 1.0)

### 9.2 Tokenization

- Lowercase; split on non-alphanumeric runs; split camelCase (`goroutineLeak` → `goroutine`, `leak`).
- Keep tokens of length ≥ 2.
- (Optional) small stopword list for English; off by default.

### 9.3 Ranking

BM25-lite per field with the weights above, summed. Parameters: `k1 = 1.5`, `b = 0.75` (standard). Document length = token count of the field being scored.

### 9.4 Scope

- Inverted index built at startup — `map[token][]docID` with per-field term frequencies.
- No embeddings / vector search in v1 (non-goal). A future extension may add a `semantic` search tool.

### 9.5 Subtree restriction

When `path` is provided, restrict candidate docs to skills whose path is under that prefix (string prefix on the normalized path + `/`).

---

## 10. Configuration

| Source | Key | Default | Notes |
|---|---|---|---|
| Env / flag | `LAZY_SKILLS_ROOT` / `--root` | `~/.agents/lazy-skills` | Skill tree root. |
| Env | `LAZY_SKILLS_LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error`. Logs to `stderr`. |
| flag | `--max-results` | `10` | Default search limit. |
| flag | `--max-results-max` | `50` | Hard cap on search limit. |
| Env | `LAZY_SKILLS_SCRIPT_TIMEOUT` | `30` | Default script timeout in seconds (per-call `timeout` still wins). |
| Env | `LAZY_SKILLS_SCRIPT_EXTS` | `.sh,.bash,.py,.rb,.js,.ts,.pl` | Comma-separated allowed script extensions. |

Config precedence: flag > env > default. The stdio server reads env; the CLI accepts flags (useful for `--list` and tests).

---

## 11. Error Handling

Tool-level errors are returned as MCP tool errors with a stable `code`:

| code | Meaning |
|---|---|
| `invalid_input` | Malformed argument (bad type, missing required). |
| `not_found` | Path doesn't exist under root. |
| `not_skill` | Path exists but is not a skill (no `SKILL.md`). |
| `invalid_query` | Search query empty/invalid. |
| `root_not_found` | Configured root does not exist / isn't a directory. |
| `not_a_script` | File does not have an allowed script extension. |
| `binary_file` | File is not decodable as text. |
| `missing_interpreter` | Required interpreter not found on `PATH`. |
| `execution_failed` | Script process could not be started (e.g. permission). |
| `internal` | Unexpected failure (logged with stack to stderr). |

Behavior rules:
- **Never crash** on a single bad `SKILL.md` (soft-fail per §4.2).
- A missing root is a **startup error**: log clearly and exit non-zero (fail fast) rather than serving an empty catalog silently. (`--allow-empty-root` opt-in can override for testing.)
- Search with zero matches → valid response with `results: []`, `total: 0` (not an error).

---

## 12. Security

- **Path confinement**: every path argument is normalized and must resolve **inside** the root. Reject `..`, absolute paths, and symlink escapes (resolve the final path, verify the prefix).
- **Symlinks**: do not follow symlinks that point outside the root. (Decision point: in v1, treat symlinks *inside* the root as valid; reject escapes.)
- **Read-only**: the server only reads `SKILL.md` files. No write tool.
- **No secrets**: SKILL.md content is passed to the agent verbatim; the server does not redact. Operators are responsible for not putting secrets in the skill tree. (Documented, not enforced.)
- **Resource URI parsing**: validate the `lazy-skill://` scheme and re-run the same path confinement before any read.
- **Script execution** (`execute_skill_script` only):
  - Script path must resolve to a file **inside** the skill directory (same path-confinement rules as above).
  - **Interpreter allow-list**: only the extensions/interpreters in §3.3 / §7.7 may run. No arbitrary interpreter or shell invocation.
  - **Working directory** is the skill directory.
  - **Timeout**: enforced via `context.WithTimeout`; on timeout the process is killed.

---

## 13. Go Project Layout & Types

```
lazy-skills/
  go.mod
  cmd/
    lazy-skill-mcp/
      main.go            # stdio server entrypoint + flag/env parsing
  internal/
    skill/
      skill.go           # Skill type, SKILL.md parsing, frontmatter
      skill_test.go
    index/
      index.go           # tree walk, Node tree, maps, startup index
      tree.go
      index_test.go
    search/
      token.go           # tokenization, camelCase split
      bm25.go            # BM25-lite scoring
      search.go          # query -> ranked matches + snippets
      search_test.go
    server/
      server.go          # MCP wiring, capabilities
      tools.go           # list_skills / get_skill / search_skills / skill_tree
      files.go           # list_skill_files / get_skill_file
      execute.go         # execute_skill_script (delegates to executor)
      resources.go       # lazy-skill:// resource handlers
      errors.go          # stable error codes
      server_test.go
    executor/
      executor.go        # script execution, interpreter dispatch, timeout
      executor_test.go
    config/
      config.go          # env + flag resolution
  spec/
    lazy-skill-mcp-server.md
```

### 13.1 Core types

```go
type Skill struct {
    Path        string            // "dev/golang/error-handling"
    Name        string            // frontmatter name or dir name
    Description string
    Tags        []string
    Version     string
    FrontMatter map[string]any
    Content     string            // body, frontmatter stripped
    ModTime     time.Time
}

type Node struct {
    Name     string
    Path     string
    IsSkill  bool
    Skill    *Skill
    Children []*Node
}

type Index struct {
    root     string
    tree     *Node
    byPath   map[string]*Skill
    inverted *InvertedIndex
    builtAt  time.Time
    count    int
}

type InvertedIndex struct {
    docs   []*Skill
    docLen []int
    avgdl  float64
    // per field
    tf     map[string]map[string]int  // field -> token -> docID -> freq
    df     map[string]map[string]int  // field -> token -> df
}

type SkillFile struct {
    Name        string // relative to the skill directory
    Size        int64
    IsScript    bool
    IsReference bool
    Executable  bool
}

type ExecResult struct {
    Path     string
    File     string
    ExitCode int
    Stdout   string
    Stderr   string
    TimedOut bool
    Duration time.Duration
}
```

### 13.2 Concurrency

- The `Index` is built once at startup and treated as immutable in v1.
- No background refresh or rebuild is performed; restart the server to reload the catalog.
- No global mutex is needed for steady-state reads.

---

## 14. CLI

```
lazy-skill-mcp [flags]
```

| Flag | Description |
|---|---|
| `--root <path>` | Skill root (default `~/.agents/lazy-skills`). |
| `--list` | Print the catalog tree to stdout and exit (no server). |
| `--search <query>` | One-shot search, print JSON results and exit (for debugging). |
| `--version` | Print version and exit. |
| `--help` | Usage. |

Default (no `--list`/`--search`) → run the stdio MCP server. `--list`/`--search` are diagnostic conveniences that reuse the same index code.

---

## 15. Testing

- **Unit**
  - `skill`: frontmatter parsing (valid, missing, malformed, empty body, name fallback).
  - `index`: tree walk on a fixture tree (including a node that is both skill and category), path normalization, confinement.
  - `search`: tokenization/camelCase, BM25 ordering, field weighting, subtree restriction, snippet extraction, zero-hit.
  - `server`: each tool's input validation and happy path (incl. fetching a `references/*.md` via `get_skill_file`); each error code.
  - `executor`: interpreter dispatch by extension, timeout, `not_a_script`, `missing_interpreter`.
- **Fixtures**: a checked-in `testdata/skills/` tree mirroring the §3 example (including a `references/` dir with `.md` files) plus edge cases (bad frontmatter, empty skill, deep nesting, skill+category node).
- **Integration**: an MCP client harness that speaks stdio against the server and exercises initialize → list → get → search round-trips.
- **Property/edge**: path traversal attempts (`../etc/passwd`, absolute, symlink escape) must all be rejected.
- **Performance (non-blocking)**: a fixture of ~1000 skills; startup index build and a representative search should complete in well under a second (target only, not a gate).

---

## 16. Milestones

- **M0 — Skeleton**
  - Project scaffold, config, tree walk, `list_skills` + `get_skill`, stdio server, basic tests.
- **M1 — Search**
  - Tokenizer, BM25-lite, `search_skills`, snippets, subtree restriction, tests.
- **M2 — Polish & extensibility**
  - `skill_tree` tool, `list_skill_files` + `get_skill_file`, MCP resources (`lazy-skill://`), `--list`/`--search` CLI, diagnostic logging.
- **M3 — Hardening**
  - `execute_skill_script` (timeout, interpreter allow-list, path confinement), security tests (traversal, symlink, script escape), performance fixture, docs, versioning.
- **Future (out of v1)**: semantic search, write/tool APIs, HTTP/SSE transport, multi-root, authz.

---

## 17. Open Questions / Decisions

1. **Node that is both skill and category** — currently modeled as "skill with children". Confirm this is desired vs. forbidding it at index time.
2. **Description fallback** — when no `description` frontmatter, use the first non-empty body line? (Proposed: yes.)
3. **Resources in v1** — include now (cheap) or defer to M2? (Proposed: M2 to keep v1 surface minimal.)
4. **Search stopword list** — on/off by default? (Proposed: off.)
5. **Symlinks inside root** — follow or ignore? (Proposed: follow, but reject escapes.)
6. **Multi-root** — is a single root sufficient for v1? (Proposed: yes; list of roots is a future extension.)
7. **Frontmatter parser** — pick a lightweight YAML lib (e.g. `gopkg.in/yaml.v3`) vs. a minimal hand-rolled parser. (Proposed: `yaml.v3`.)
8. **Allowed script extensions** — is the default set (`.sh`, `.bash`, `.py`, `.rb`, `.js`, `.ts`, `.pl`) right, and should it be configurable? (Proposed: fixed default, overridable via `LAZY_SKILLS_SCRIPT_EXTS`.)
9. **`.ts` execution** — which runner (`tsx` vs `ts-node` vs `node --loader`)? (Proposed: `tsx`.)
10. **Default timeout** — is 30 s a reasonable default? (Proposed: yes; overridable per-call and via env.)

---

## 18. Acceptance Criteria (v1)

- Given the fixture tree, `list_skills("")` returns the correct top-level categories/skills with names + descriptions and no bodies.
- `get_skill("dev/golang/error-handling")` returns the exact body and parsed frontmatter.
- `search_skills("goroutine leak")` ranks `dev/golang/testing` in the top results and returns a relevant snippet.
- All traversal/escape path attempts are rejected with `not_found` (or `invalid_input`) and no read occurs outside the root.
- A malformed `SKILL.md` does not prevent the rest of the catalog from being indexed; a warning is logged.
- `list_skill_files("dev/golang/testing")` returns the bundled scripts with correct `is_script` flags and excludes `SKILL.md`.
- `get_skill_file("dev/golang/testing", "scripts/setup.sh")` returns the exact file content; a binary file returns `binary_file`.
- `list_skill_files("dev/golang/testing")` flags `references/*.md` files with `is_reference: true`, and `get_skill_file("dev/golang/testing", "references/setup-guide.md")` returns the reference content.
- `execute_skill_script("dev/golang/testing", "scripts/run.sh")` runs in the skill directory and returns `stdout` + `exit_code: 0`.
- A script path containing `..` or resolving outside the root is rejected with `not_found`; a non-script file returns `not_a_script`.
- A script exceeding its timeout returns `timed_out: true` and the process is killed.
- The server runs over stdio and completes initialize → tools/list → tools/call against a standard MCP client.
- `go build`, `go test ./...`, and `go vet ./...` pass.

---

## Appendix A — Example catalog

```
~/.agents/lazy-skills/
  dev/
    golang/
      error-handling/SKILL.md
      testing/
        SKILL.md
        scripts/
          setup.sh
          run-linters.sh
        references/
          setup-guide.md
        examples.md
    web/
      responsive-design/SKILL.md
  research/
    web-search/SKILL.md
```

| Path | Type |
|---|---|
| `dev` | category |
| `dev/golang` | category |
| `dev/golang/error-handling` | skill |
| `dev/golang/testing` | skill |
| `dev/golang/testing/scripts/setup.sh` | script |
| `dev/golang/testing/scripts/run-linters.sh` | script |
| `dev/golang/testing/references/setup-guide.md` | reference |
| `dev/golang/testing/examples.md` | auxiliary file |
| `dev/web` | category |
| `dev/web/responsive-design` | skill |
| `research` | category |
| `research/web-search` | skill |
