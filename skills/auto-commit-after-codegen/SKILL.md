---
name: auto-commit-after-codegen
description: Automatically create a focused local git commit after implementation or code-generation work. Use whenever the user wants code changes delivered as a finished local commit, or when the standing preference is to commit code changes by default unless the user explicitly says not to commit. Stage only intended files, run the smallest relevant validation, write a concise commit message, and never push unless explicitly requested.
---

# Auto Commit After Codegen

Use this skill when implementation work should normally end with a local git commit.

## Trigger Intent

Typical requests:

- "每次生成代码后直接提交 commit"
- "Implement this and commit it"
- "改完自动提交，不用再问我"
- "Make the change and create a local commit"
- "以后默认帮我本地提交"
- "Code first, then commit locally"
- "Treat code changes as done only after a local commit"

Implicit trigger rule:

- If the user asks for implementation, code generation, bug fixing, refactoring, or file edits, and has an established preference that code changes should be committed locally by default, use this skill even if the current message does not repeat "commit".

Do not use this skill when:

- the user only wants analysis, planning, brainstorming, or review-only feedback
- the user explicitly says not to commit
- the task does not modify code or tracked project files

## Required Workflow

After making code changes:

1. Inspect the working tree before staging.
2. Stage only files that belong to the task.
3. Run the smallest relevant validation for the changed code.
4. If validation fails, do not commit. Report the failure clearly.
5. If validation passes, create one local commit with a focused message.
6. Do not push, open a PR, or modify remotes unless the user explicitly asks.

## Staging Rules

- Never stage unrelated files just because they are already modified.
- If unrelated changes exist in the worktree, leave them unstaged.
- If a generated artifact or debug binary is not intended for source control, remove it from the commit scope before committing.
- Prefer explicit `git add <path>` over broad staging.
- If the current task spans multiple distinct changes, prefer one focused commit only for the change actually delivered in this turn.

## Validation Rules

- Run the narrowest command that gives confidence for the change.
- Prefer existing project-native checks.
- If no project-specific validation exists, run the smallest obvious baseline for the touched language or package.
- If validation is impossible, do not silently commit. State that validation could not be run and why.
- If a validation command fails because of an unrelated pre-existing problem, do not hide it. Report that the commit was skipped or that validation was partial.

Examples:

- Go package change: `go test ./...` or a narrower package target if clearly sufficient
- Single-file formatting change: formatting check or relevant unit tests if available

## Commit Message Rules

Use a short imperative message. Keep it specific to the delivered change.

Preferred patterns:

- `Add RPC factor bytecode VM`
- `Extend get() to support object factors`
- `Fix ternary null type inference`

Avoid:

- Generic messages like `update`, `fix stuff`, `changes`
- Multi-topic commits when one focused commit is possible

## Safety Rules

- Never amend an existing commit unless the user explicitly asks.
- Never rewrite history as part of this workflow.
- Never auto-push.
- If validation failed, stop before commit.
- If you find unexpected unrelated modifications in files required for the task, inspect carefully and commit only the intended final state.
- If the task produced no material file change, do not create an empty commit.

## Response Pattern

When the task is finished:

- State that the code was committed locally.
- Include the commit hash and subject line.
- Mention the validation command that was run.
- If push was not requested, do not discuss push unless relevant.

Example:

`Committed locally as abc1234 "Extend get() to support object factors". Validation: go test ./...`

## Repository Default Rule

If this skill is installed because the repository owner wants local commits by default, interpret implementation requests with this default:

- code changes should end in one local commit unless the user explicitly opts out
- analysis-only or plan-only requests should not create commits
- push remains opt-in only

For reusable repo-level policy text, including a short developer-prompt version, see `references/repo-default-auto-commit-rule.md`.

## Minimal Command Pattern

Use this sequence when applicable:

1. `git status --short`
2. `git add <intended paths>`
3. relevant validation command
4. `git commit -m "<message>"`

Keep the workflow minimal and deterministic.
