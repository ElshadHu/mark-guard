# Contributing

## Getting Started

1. Fork the repo
2. Clone your fork
3. Create a branch from `main`: `git checkout -b test/edit-parsing`
4. Make your changes
5. Run tests and lint before pushing:
   ```bash
   make test
   make lint
   ```
6. Open a PR against `main`

## Have a Question or Idea?

Open an issue. If you spot something odd, have a suggestion, or want to discuss an approach before writing code, create an issue first. It saves everyone time.

## Rules

**One PR per issue.** Each pull request should address exactly one issue. Do not combine multiple issues into a single PR.

**Keep the scope tight.** Only change what the issue asks for. If you notice something unrelated that needs fixing, open a new issue for it.

**Small, incremental PRs.** Break large work into smaller pieces. A PR with 200 lines is easy to review. A PR with 2000 lines sits in the queue.

**If your test reveals a bug**, you have two options:
1. If the fix is small and obvious (a one-liner, an off-by-one), fix it in the same PR and mention it in your PR description.
2. If the fix is non-trivial or touches code outside your issue's scope, open a separate issue and PR for it. Reference the failing test in the new issue.

**Write tests the Go way.** Use table-driven tests with `t.Run()`. Use `t.Helper()` for helper functions. Keep test names descriptive.

**No unrelated formatting changes.** Do not reformat files you did not change. It makes diffs noisy and harder to review.

## Branch Naming

Use a prefix that describes the type of change:

- `feat/config-validation`
- `fix/empty-old-text-edge-case`
- `chore/update-dependencies`
- `test/pipeline-integration`

## Commit Messages

Keep them short and direct. Start with a verb.

- `add unit tests for ParseEdits`
- `validate config fields in Load()`
- `fix empty old_text edge case in ApplyEdits`

## Before You Submit

- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Your PR description references the issue: `Closes #NUMBER`
- [ ] You did not change files outside the scope of the issue
