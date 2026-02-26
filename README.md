# mark-guard

A CLI tool that keeps your documentation in sync with your Go code.

You change code. You forget to update docs. You run `mark-guard format`. It parses the AST of your old code (from git) and your new code (on disk), extracts a semantic diff of exported symbols, feeds that structured diff plus your current markdown docs to an LLM, and writes the updated docs back to disk. You review, commit, done.

I built this because I keep forgetting to update docs after code changes. I see the same problem in open source projects all the time. Text diffs are noisy and miss the point. AST-level diffing tells you exactly what changed in the public API, which is exactly what documentation cares about.

## Status

Work in progress. Not usable yet.

| Phase | Description | Status |
|---|---|---|
| 1-2 | Skeleton + Git Integration | Done |
| 3 | Go Symbol Extraction | Done |
| 4 | Symbol Diffing | In progress |
| 5 | Doc Scanning | Not started |
| 6 | LLM Integration | Not started |
| 7 | End-to-End Wiring | Not started |
| 8 | Polish | Not started |

## How It Works


1. Detect changed `.go` files via `git diff --name-only` + `git ls-files --others`
2. Read old version from `git show HEAD:<file>`, new version from disk
3. Parse both with `go/parser.ParseFile`, extract exported symbols (functions, types, structs, interfaces, consts, vars)
4. Diff the two symbol sets: what was added, removed, or modified (down to individual parameters, fields, methods)
5. Scan markdown docs for references to changed symbols(Planned)
6. Send the structured diff + relevant docs to an LLM (Planned)
7. Write updated docs to disk (Planned)

## Key Design Decisions

| Decision | Choice | Why |
|---|---|---|
| Diff strategy | AST-level symbol diff, not text diff | Text diffs include noise (whitespace, imports, comments). AST diff gives semantic changes: "parameter added", "field type changed". That is what docs care about. |
| Parser | `go/parser` only, no `go/types` | We parse raw strings from `git show`. `go/types` needs the full module graph. We need signatures, not resolved types. |
| Git integration | `os/exec` shelling out to `git` | `go-git` pulls 30+ dependencies. System `git` is faster for simple operations. Assumes the project is a git repo. In the future I can add a check for this before running. |
| LLM provider | Single OpenAI-compatible endpoint | One wire format covers OpenAI, Anthropic, Ollama, Groq, Together. No provider-specific SDKs. Just `net/http` + JSON. |
| CLI framework | Cobra without Viper | Cobra gives subcommands, flags, help text. Viper pulls 20 transitive deps for reading one YAML file. We use `yaml.v3` directly. |
| Config | `.markguard.yaml` with env var references | API key stored as env var name, not the key itself. Config is optional, defaults work out of the box. |
| Auto-commit | No | The tool writes files to disk. You review with `git diff`, you commit. |



## What It Does Not Do

- Generate docs from scratch. It updates existing docs only.
- Support languages other than Go. Each language needs its own parser. Go-only for now.
- Auto-commit. You review the changes first.
- Cache or do incremental runs. The tool runs in seconds for typical repos.
- Custom prompt templates. The prompt is hardcoded. Customization is a future concern.

## Dependencies

```
github.com/spf13/cobra       # CLI framework
gopkg.in/yaml.v3              # YAML config parsing
```

Two external deps. Everything else is Go stdlib (`go/parser`, `go/ast`, `go/token`, `net/http`, `os/exec`, `encoding/json`).

## Usage (planned)

```bash
# Install
go install github.com/elshadaghazade/mark-guard/cmd/mark-guard@latest

# Set your API key
export OPENAI_API_KEY="sk-..."

# Run against last commit
mark-guard format

# Run against a specific branch
mark-guard format --base=main

# Dry run (show what would change without writing)
mark-guard format --dry-run

# Filter to specific paths
mark-guard format --path=internal/git/
```

## Config

Create `.markguard.yaml` at your repo root (optional, defaults work without it):

```yaml
llm:
  base_url: "https://api.openai.com/v1"   # or http://localhost:11434/v1 for Ollama
  api_key_env: "OPENAI_API_KEY"            # env var name, not the key itself
  model: "gpt-4o"
docs:
  paths:
    - "docs/"
    - "README.md"
  exclude:
    - "docs/roadmap.md"
```

## Development

```bash
make build     # build binary to bin/mark-guard
make test      # go test ./... -v -race
make lint      # golangci-lint run ./...
make run       # go run ./cmd/mark-guard format
```

## Roadmap

| Phase | What | What I did | Status |
|---|---|---|---|
| 1-2 | Skeleton + Git Integration | Cobra CLI with `format` subcommand. Config loader with `yaml.v3`. Git client that detects changed `.go` files and reads old content via `git show`. Table-driven tests with temp git repos. | Done |
| 3 | Go Symbol Extraction | Parser using `go/parser.ParseFile` that extracts exported functions, methods, structs, interfaces, consts, vars. Structured params/returns/fields for per-element diffing later. `go/format.Node` to render types back to source strings. | Done |
| 4 | Symbol Diffing | Map-keyed comparison of old vs new symbols. Three-pass detection: added, removed, modified. Per-parameter, per-field, per-method change detection. Deterministic output sorted by (kind, name). Human-readable summary formatter for LLM consumption. | In progress |
| 5 | Doc Scanning | I will figure out | Not started |
| 6 | LLM Integration | I will figure out | Not started |
| 7 | End-to-End Wiring | I will figure out | Not started |
| 8 | Polish | I will figure out | Not started |

## References

Projects and resources I studied while building this:

| Project | What I found |
|---|---|
| `golang.org/x/exp/apidiff` | API change detection between Go package versions. Map-keyed symbol comparison. |
| `go/doc` | Groups methods, consts, vars under parent types. |
| `go/parser` + `go/ast` | AST parsing without type-checking. |
| Cobra (`spf13/cobra`) | Subcommand routing and flag parsing. |
| GumTree | AST-level diffing across languages. Too heavy, but informed the symbol-level approach. |
| `golangci-lint` | Shells out to `git` instead of using a Go git library. |
| `sergi/go-diff` | Myers diff in Go. Considered for params/fields, skipped for MVP. |

