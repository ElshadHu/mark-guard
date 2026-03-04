# mark-guard

A CLI tool that keeps your documentation in sync with your Go code.

You change code. You forget to update docs. mark-guard parses the AST of your old code (from git) and your new code (on disk), extracts a semantic diff of exported symbols, and produces a structured summary of what changed in the public API. It then feeds that diff plus your current markdown docs to an LLM and writes the updated docs back to disk.

Text diffs are noisy and miss the point. AST-level diffing tells you exactly what changed in the public API — which is exactly what documentation cares about.

## Status

End-to-end pipeline works. Hardening in progress (see docs/day8.md for the improvement plan).

| Phase | Description | Status |
|---|---|---|
| 1-2 | Skeleton + Git Integration | Done |
| 3 | Go Symbol Extraction | Done |
| 4 | Symbol Diffing | Done |
| 5 | Doc Scanning | Done |
| 6 | LLM Integration | Done |
| 7 | End-to-End Wiring | Done |

**What works today:**
- Detects changed `.go` files via git
- Parses old and new Go source, extracts exported symbols
- Diffs symbol sets: added, removed, modified (down to parameters, fields, methods)
- Produces human-readable diff summaries
- Scans and selects relevant markdown docs via config-based mapping
- Loads config from `.markguard.yaml` with sensible defaults
- Verifies the `.markguard.yaml` configuration for correctness
- Sends diff + docs to LLM (Gemini/OpenAI compatible) and writes updates back

**What needs hardening:**
- Dry-run mode and user confirmation before writing
- JSON structured output instead of full-file replacement
- Prompt injection mitigation
- Pre-write validation (content-loss guard)
- Token optimization for large repos

## How It Works

1. Detect changed `.go` files via `git diff --name-only` + `git ls-files --others`
2. Read old version from `git show HEAD:<file>`, new version from disk
3. Parse both with `go/parser.ParseFile`, extract exported symbols (functions, types, structs, interfaces, consts, vars)
4. Diff the two symbol sets: what was added, removed, or modified (down to individual parameters, fields, methods)
5. Scan configured doc paths, select relevant markdown files via config-based mapping
6. Build prompt with diff summary + doc content, send to LLM
7. Parse LLM response and write updated docs back to disk

## Key Design Decisions

| Decision | Choice | Why |
|---|---|---|
| Diff strategy | AST-level symbol diff, not text diff | Text diffs include noise (whitespace, imports, comments). AST diff gives semantic changes: "parameter added", "field type changed". That is what docs care about. |
| Parser | `go/parser` only, no `go/types` | We parse raw strings from `git show`. `go/types` needs the full module graph. We need signatures, not resolved types. |
| Git integration | `os/exec` shelling out to `git` | `go-git` pulls 30+ dependencies. System `git` is faster for simple operations. |
| Doc-to-code mapping | Config-based mapping + send-all fallback | Small repos: send all docs (zero config). Large repos: user adds mappings for precision. No false-positive symbol scanning. |
| CLI framework | Cobra without Viper | Cobra gives subcommands, flags, help text. Viper pulls 20 transitive deps for reading one YAML file. We use `yaml.v3` directly. |
| Config | `.markguard.yaml` with env var references | API key stored as env var name, not the key itself. Config is optional, defaults work out of the box. |

## What It Does Not Do

- Generate docs from scratch. It updates existing docs only.
- Support languages other than Go. Each language needs its own parser. Go-only for now.
- Auto-commit. You review the changes first.

## Dependencies

```
github.com/spf13/cobra       # CLI framework
gopkg.in/yaml.v3              # YAML config parsing
```

Two external deps. Everything else is Go stdlib (`go/parser`, `go/ast`, `go/token`, `os/exec`, `encoding/json`).

## Config

Create `.markguard.yaml` at your repo root (optional, defaults work without it):

```yaml
llm:
  base_url: "https://generativelanguage.googleapis.com/v1beta/openai"
  api_key_env: "GEMINI_API_KEY"
  model: "gemini-2.5-flash"
docs:
  paths:
    - "docs/"
    - "README.md"
  exclude:
    - "docs/roadmap.md"
  mappings:
    - docs: ["docs/api.md"]
      code: ["internal/git/", "internal/config/"]
    - docs: ["README.md"]
      code: ["cmd/", "internal/cli/"]
```

Without `.markguard.yaml`, defaults are:
- **Provider:** Gemini (`gemini-2.5-flash`)
- **API key env:** `GEMINI_API_KEY`
- **Doc paths:** `docs/`, `README.md`
- **Mappings:** None (sends all docs, fine for small repos)

## Development

```bash
make build     # build binary to bin/mark-guard
make test      # go test ./... -v -race
make lint      # golangci-lint run ./...
make run       # go run ./cmd/mark-guard format
```

## Roadmap

| Phase | What | What I Did | Status |
|---|---|---|---|
| 1-2 | Skeleton + Git | Cobra CLI, config loader with `yaml.v3`, git client that detects changed `.go` files and reads old content via `git show`. | Done |
| 3 | Symbol Extraction | Parser using `go/parser` that extracts exported functions, methods, structs, interfaces, consts, vars with structured params/fields. | Done |
| 4 | Symbol Diffing | Map-keyed comparison, three-pass detection (added/removed/modified), per-field change detection, deterministic sorted output, human-readable summary formatter. | Done |
| 5 | Doc Scanning | Scanner that walks configured paths, reads `.md` files, filters by config-based doc-to-code mapping, token estimation. | Done |
| 6 | LLM Integration | OpenAI-compatible client with retry/backoff, prompt builder, response parser, token budget check. | Done |
| 7 | End-to-End Wiring | `format` command runs the full pipeline: git diff, symbol extraction, doc scan, LLM call, write updates. | Done |

## References

| Project | What I used it for |
|---|---|
| `golang.org/x/exp/apidiff` | Reference for map-keyed symbol comparison and API change detection between package versions. |
| `go/doc` | Grouping methods, consts, and vars under parent types. |
| `go/parser` + `go/ast` | AST parsing without type-checking (works on raw strings from `git show`). |
| Cobra (`spf13/cobra`) | Subcommand routing and flag parsing. |
| `golangci-lint` | Reference for shelling out to `git` instead of pulling in a Go git library. |
| Gemini OpenAI compatibility | [ai.google.dev/gemini-api/docs/openai](https://ai.google.dev/gemini-api/docs/openai) |
