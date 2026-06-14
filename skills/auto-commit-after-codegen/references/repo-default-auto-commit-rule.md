# Repository Default Auto Commit Rule

Use this text as a repository-level default rule for Codex or other coding agents:

## Rule

When a request results in code or tracked project file changes, treat the task as incomplete until the intended changes are committed locally.

## Expected Behavior

- After implementation, stage only files that belong to the task.
- Run the smallest relevant validation for the changed code.
- If validation passes, create one focused local commit.
- If validation fails, do not commit. Report the failure and stop.
- Do not stage unrelated local changes.
- Do not create empty commits.
- Do not amend or rewrite history unless explicitly requested.
- Do not push unless explicitly requested.

## Default Interpretation

- Implementation, bug fix, refactor, and code-generation requests should end with a local commit.
- Analysis-only, review-only, planning-only, and brainstorming requests should not create a commit.
- If the user explicitly says not to commit, skip the commit.

## Response Format

After a successful implementation:

- report that the change was committed locally
- include the commit hash and subject line
- include the validation command that was run

Example:

`Committed locally as abc1234 "Fix ternary null handling". Validation: go test ./...`

## Short Developer Prompt

Use this shorter version when you need a compact developer-prompt form:

```text
Default behavior in this repository: any implementation task that changes tracked files must end with a local commit unless the user explicitly opts out.

When a request results in code or tracked project file changes, treat the task as incomplete until the intended changes are committed locally.

After implementation:
- stage only files that belong to the task
- run the smallest relevant validation for the changed code
- if validation passes, create one focused local commit
- if validation fails, do not commit
- do not stage unrelated local changes
- do not create empty commits
- do not amend or rewrite history unless explicitly requested
- do not push unless explicitly requested

Interpretation rules:
- implementation, bug-fix, refactor, and code-generation requests should end with a local commit by default
- analysis-only, planning-only, brainstorming-only, and review-only requests must not create commits
- if the user explicitly says not to commit, skip the commit

Final response after a successful implementation should include:
- the local commit hash
- the commit subject line
- the validation command that was run
```
