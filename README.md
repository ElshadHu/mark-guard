# mark-guard

A CLI tool that keeps your documentation in sync with your Go code.

You change code. You forget to update docs. You run `mark-guard format`. It diffs your code at the AST level, feeds the structured diff plus your current docs to an LLM, and writes the updated docs back to disk. You review, commit. That is my current idea, because I  easily forget changing docs and see people that also they forget especially in open source projects.

## Status

WIP. Not usable yet.

## What it will do

1. Detect changed `.go` files via git
2. Parse old and new versions using Go's AST parser
3. Extract a semantic diff of exported symbols (functions, types, structs, interfaces)
4. Send the structured diff + your existing markdown docs to an LLM
5. Write updated docs to disk

## What it won't do

- Generate docs from scratch which  it updates existing docs
- Support languages other than Go (for now)
- Auto-commit which  you review the changes first