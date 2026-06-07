# gh-prx

[日本語版](README.ja.md)

`gh-prx` is a small Go CLI for safely reading and updating named Markdown
blocks in GitHub pull request bodies and PR comments.

It is designed for LLM agents as well as humans. Instead of asking an agent to
read and rewrite an entire PR body or long comment, mark the managed block with
HTML comments and let the CLI read or replace only that block.

```md
<!-- section:start -->
...
<!-- section:end -->
```

The GitHub API still updates full Markdown fields, but `gh-prx` presents those
fields as named read/write blocks and shows a diff before or after writes.
For existing PR bodies that do not have markers yet, it can also replace an
explicit line range with expectation guards.

## Install

As a gh extension from this repository:

```sh
gh extension install s4na/gh-patch
gh patch body read 123 --marker section
```

GitHub CLI derives the extension command from the repository name, so this
repository is exposed as `gh patch ...`. To use the command exactly as
`gh-prx ...`, publish the same binary from a `gh-prx` extension repository.
This repository also includes a root `gh-patch` wrapper for source-based gh
extension installs. This source-based extension path runs `go run` and requires
a local Go toolchain. Use the Homebrew or `go install` paths below when you want
an installed binary.

With Homebrew after releases are published to the tap:

```sh
brew install s4na/tap/gh-prx
gh-prx body read 123 --marker section
```

From source:

```sh
go install github.com/s4na/gh-patch/cmd/gh-prx@latest
```

## Examples

Read a marker block from a PR body:

```sh
gh-prx body read 123 --marker section
gh-prx body read 123 --marker section --plain
```

Read a PR body line range and capture the current content hash:

```sh
gh-prx body read 123 --range 12:18 --json
```

Replace a marker block in a PR body:

```sh
gh-prx body write 123 --marker section --file section.md
cat section.md | gh-prx body write 123 --marker section -
```

Preview a write without updating GitHub:

```sh
gh-prx body write 123 --marker section --file section.md --dry-run
```

Preview and replace an existing PR body line range:

```sh
gh-prx body lines 123 --range 12:18 --file section.md --dry-run
gh-prx body lines 123 --range 12:18 --file section.md --expect-sha <body_sha>
```

Read or update PR comments:

```sh
gh-prx comment read 123 --marker section
gh-prx comment write 123 --comment-id 123456 --marker section --file section.md
gh-prx comment upsert 123 --marker section --file section.md
```

Replacing an entire comment is intentionally explicit:

```sh
gh-prx comment write 123 --comment-id 123456 --whole --file comment.md
```

## Conservative Defaults

By default, missing markers fail instead of inserting new blocks. Explicitly opt
in when insertion is intended:

```sh
gh-prx body write 123 --marker section --file section.md --insert-if-missing
```

Marker blocks are the preferred long-lived target because they survive unrelated
body edits. Line-range replacement is intended as an escape hatch for existing
PR bodies. A real `body lines` update requires one of `--expect-sha`,
`--expect-file`, or `--force`; `--dry-run` remains available without a guard so
humans and agents can inspect the exact replacement first.

```sh
gh-prx body read 123 --range 12:18 --json
gh-prx body lines 123 --range 12:18 --file section.md --expect-sha <body_sha>
gh-prx body lines 123 --range 12:18 --file section.md --expect-file old-section.md
```

For marker-based comment reads, if multiple comments contain the same marker,
`gh-prx` refuses to pick one implicitly and prints candidate comment IDs so
callers can retry with `--comment-id`. `comment upsert` is narrower: it only
matches marker comments authored by the current GitHub user.
When retrying a marker-scoped write, keep the marker explicit:

```sh
gh-prx comment write 123 --comment-id 123456 --marker section --file section.md
```

## Diff Output

Writes print a compact line diff for the managed block. The line number is from
the old side for `-` rows and from the new side for `+` rows:

```diff
<!-- section:start -->
op | line | content
 - |    1 | old content
 - |    2 | old line
 + |    1 | new content
 + |    2 | new line
<!-- section:end -->
```

No-op updates are detected before updating GitHub and exit with code `5`.

## JSON

Pass `--json` for machine-readable output:

```sh
gh-prx body write 123 --marker section --file section.md --json
```

Example:

```json
{
  "target": "pull_request_body",
  "pull_number": 123,
  "marker": "section",
  "updated": true,
  "changed": true,
  "dry_run": false,
  "url": "https://github.com/org/repo/pull/123",
  "message": "<diff omitted>"
}
```

## Exit Codes

| Code | Meaning |
| ---: | --- |
| 0 | success |
| 1 | validation error |
| 2 | marker not found |
| 3 | ambiguous target |
| 4 | GitHub API error |
| 5 | no changes |

## CI

The repository runs `gofmt`, `go test ./...`, builds both `./cmd/gh-prx` and
`./cmd/gh-patch`, and audits GitHub Actions workflows with `zizmor` in GitHub
Actions.
