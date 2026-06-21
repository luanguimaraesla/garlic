# Contributing

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/) and follow
[cbea.ms/git-commit](https://cbea.ms/git-commit/): a capitalized, imperative
subject under 50 characters, a blank line, then a body wrapped at 72 that
explains what changed and why.

Common types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`,
`build`.

## Releases and versioning

Releases are automated. On every push to `main`, the `release` job runs
[go-semantic-release](https://github.com/go-semantic-release/semantic-release),
reads the commits since the latest tag, and cuts the next version. You do not
tag releases by hand.

How the next version is chosen, configured in [`.semrelrc`](.semrelrc):

- a `feat` commit bumps the **minor** version,
- a `fix` commit bumps the **patch** version,
- anything else bumps the **patch** version, because the CI job sets
  `force-bump-patch-version`,
- only a `major!` commit (for example `major!: drop the legacy client`) bumps
  the **major** version.

The point of the `major!` gate is that breaking changes do not force a major on
their own. A breaking change ships as a minor by default, and the major bump
happens only when you deliberately ask for it with a `major!` commit. Call this
out clearly in the release notes when a minor carries breaking changes.

### Do not use `BREAKING CHANGE` footers

go-semantic-release always cuts a **major** when a commit body contains the
literal text `BREAKING CHANGE` or `BREAKING CHANGES`, and that behavior cannot
be configured. Do not use those footers. Describe breaking changes in plain
prose in the commit body instead, and reserve the major bump for an explicit
`major!` commit.

### A note on major versions

The module path is `github.com/luanguimaraesla/garlic` with no `/v2` suffix, so
the library is meant to stay within the `v1.x` line. Cutting a real `v2.0.0`
under Go's semantic import versioning would require renaming the module to
`github.com/luanguimaraesla/garlic/v2`, updating every internal import, and
forcing all consumers to update their import paths. Do not cut a `v2` by tagging
alone; plan the module-path migration first.
