# Repository Rules

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
