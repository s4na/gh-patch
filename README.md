# gh-prx

`gh-prx` is a small Go CLI for safely reading and updating named Markdown
blocks in GitHub pull request bodies and PR comments.

It is designed for LLM agents as well as humans. Instead of asking an agent to
read and rewrite an entire PR body or long comment, mark the managed block with
HTML comments and let the CLI read or replace only that block.

```md
<!-- ai-summary:start -->
...
<!-- ai-summary:end -->
```

The GitHub API still updates full Markdown fields, but `gh-prx` presents those
fields as named read/write blocks and shows a diff before or after writes.

## Install

As a gh extension from this repository:

```sh
gh extension install s4na/gh-patch
gh patch body read 123 --marker ai-summary
```

GitHub CLI derives the extension command from the repository name, so this
repository is exposed as `gh patch ...`. To use the command exactly as
`gh-prx ...`, publish the same binary from a `gh-prx` extension repository.
This repository also includes a root `gh-patch` wrapper for source-based gh
extension installs.

With Homebrew after releases are published to the tap:

```sh
brew install s4na/tap/gh-prx
gh-prx body read 123 --marker ai-summary
```

From source:

```sh
go install github.com/s4na/gh-patch/cmd/gh-prx@latest
```

## Examples

Read a marker block from a PR body:

```sh
gh-prx body read 123 --marker ai-summary
gh-prx body read 123 --marker ai-summary --plain
```

Replace a marker block in a PR body:

```sh
gh-prx body write 123 --marker ai-summary --file summary.md
cat summary.md | gh-prx body write 123 --marker ai-summary -
```

Preview a write without updating GitHub:

```sh
gh-prx body write 123 --marker ai-summary --file summary.md --dry-run
```

Read or update PR comments:

```sh
gh-prx comment read 123 --marker ai-review
gh-prx comment write 123 --comment-id 123456 --file review.md
gh-prx comment upsert 123 --marker ai-review --file review.md
```

## Conservative Defaults

By default, missing markers fail instead of inserting new blocks. Explicitly opt
in when insertion is intended:

```sh
gh-prx body write 123 --marker ai-summary --file summary.md --insert-if-missing
```

If multiple comments contain the same marker, `gh-prx` refuses to update any of
them and prints candidate comment IDs so callers can retry with `--comment-id`.
When retrying a marker-scoped update, keep the marker explicit:

```sh
gh-prx comment write 123 --comment-id 123456 --marker ai-review --file review.md
```

## Diff Output

Writes print a compact line diff for the managed block. The first two columns
are the old and new line numbers inside the marker block:

```diff
<!-- ai-summary:start -->
1   - old summary
2   - old risk note
  1 + new summary
  2 + new risk note
<!-- ai-summary:end -->
```

No-op updates are detected before calling GitHub and exit with code `5`.

## JSON

Pass `--json` for machine-readable output:

```sh
gh-prx body write 123 --marker ai-summary --file summary.md --json
```

Example:

```json
{
  "target": "pull_request_body",
  "pull_number": 123,
  "marker": "ai-summary",
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

The repository runs `gofmt`, `go test ./...`, and builds both `./cmd/gh-prx`
and `./cmd/gh-patch` in GitHub Actions.
