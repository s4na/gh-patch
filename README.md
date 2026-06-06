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
extension installs.

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

Replace a marker block in a PR body:

```sh
gh-prx body write 123 --marker section --file section.md
cat section.md | gh-prx body write 123 --marker section -
```

Preview a write without updating GitHub:

```sh
gh-prx body write 123 --marker section --file section.md --dry-run
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
