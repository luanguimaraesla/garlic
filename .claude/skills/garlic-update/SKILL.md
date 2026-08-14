---
name: garlic-update
description: >
  Upgrade github.com/luanguimaraesla/garlic in a downstream Go project and
  apply every migration documented in the generated GitHub release notes.
  TRIGGER when: the user asks to update, upgrade, bump, or migrate Garlic, or
  asks what a newer Garlic release requires. DO NOT TRIGGER when: working in
  the Garlic repository itself or changing an unrelated Go dependency.
---

# Update Garlic

Upgrade Garlic from the version used by the current Go module to a requested
version, or to the latest stable release when no version is given. Generated
GitHub release notes are the source of release-specific migration requirements.
Use `garlic-conventions` only for the target version's steady-state rules.

## Guardrails

- Work only in downstream modules that depend on
  `github.com/luanguimaraesla/garlic`.
- Read every release between the installed and target versions. Do not inspect
  only the latest release when the upgrade skips versions.
- Preserve unrelated working-tree changes. Stop and ask if they make the
  dependency update unsafe to isolate.
- Do not commit, push, or change unrelated dependencies.
- Do not invent migrations from commit titles. Read the full release body and
  follow linked commits or issues when a requirement is unclear.
- Stop and explain the ambiguity when the current version is a local replace,
  an untagged pseudo-version, or newer than the requested target.

## 1. Establish the upgrade range

1. Inspect `git status`, `go.mod`, `go.sum`, and any `go.work` file.
2. Resolve the selected Garlic module and version with `go list -m -json`.
   Account for `replace` directives before changing anything.
3. Resolve the requested target. When none is given, use the latest stable tag,
   not a prerelease.
4. Record the exact current and target versions before editing files.

## 2. Read the generated release notes

Fetch releases from `luanguimaraesla/garlic` through the GitHub CLI, GitHub API,
or the release pages. Collect every tag greater than the current version and no
greater than the target, then read the complete release bodies in ascending
semantic-version order.

For each release:

1. Record the tag and URL.
2. Extract every `Upgrade notes:` block.
3. Inspect the rest of the body for incompatible API or behavior changes that
   predate that heading convention.
4. Follow a referenced commit or issue only when the release body does not make
   the required code change clear.
5. Build a migration checklist tied to concrete files or call sites in the
   downstream project.

If a required release or its notes cannot be retrieved, stop rather than
performing a partially informed upgrade.

## 3. Audit before updating

Search the project for every API and behavior named by the migration checklist.
Include production code, tests, tools, examples, generated-code inputs, and
build scripts. Do not edit generated files when their source is available.

Read the target `garlic-conventions` skill before applying steady-state Garlic
patterns. Release notes explain the transition; conventions explain what the
final code should look like.

## 4. Update and migrate

1. Update only Garlic to the target version with the project's native Go module
   workflow, normally:

   ```bash
   go get github.com/luanguimaraesla/garlic@<target>
   go mod tidy
   ```

2. Inspect module-file changes and reject unrelated version drift unless it is a
   necessary transitive consequence of the Garlic update.
3. Apply migration checklist items in release order.
4. Re-run the audit searches to catch missed call sites.
5. Format changed Go files with the project's native formatter.

## 5. Validate

Run the repository's documented test and lint commands. If none exist, run at
least `go test ./...`. Also run focused tests for migrated call sites when the
project provides them. Review the final diff for accidental dependency or
behavior changes.

Do not call the upgrade complete while tests fail or a release-note migration
remains unresolved.

## Report

Report:

- the old and new Garlic versions,
- every release note inspected, with its URL,
- each migration applied or confirmed not applicable,
- changed files and module changes,
- validation commands and results,
- any unresolved risk or skipped check.
