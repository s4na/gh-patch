# gh-prx

[English](README.md)

`gh-prx` は、GitHub Pull Request の body や PR comment に含まれる名前付き Markdown ブロックを、安全に読み取り・更新するための小さな Go CLI です。

人間だけでなく LLM agent にも扱いやすいように設計されています。agent に PR body 全体や長い comment 全体を読ませて書き換えさせるのではなく、HTML comment marker で管理対象ブロックを示し、そのブロックだけを read / replace できます。

```md
<!-- ai-summary:start -->
...
<!-- ai-summary:end -->
```

GitHub API 上は Markdown field 全体を更新しますが、`gh-prx` は利用者から見える操作を名前付き read/write ブロックとして扱い、write の前後に diff を表示します。

## Install

このリポジトリから gh extension としてインストールする場合:

```sh
gh extension install s4na/gh-patch
gh patch body read 123 --marker ai-summary
```

GitHub CLI はリポジトリ名から extension command を決めるため、このリポジトリは `gh patch ...` として公開されます。コマンドを正確に `gh-prx ...` として使いたい場合は、同じ binary を `gh-prx` extension repository から公開してください。このリポジトリには、source-based gh extension install 用の root `gh-patch` wrapper も含まれています。

tap へ release された後に Homebrew でインストールする場合:

```sh
brew install s4na/tap/gh-prx
gh-prx body read 123 --marker ai-summary
```

source からインストールする場合:

```sh
go install github.com/s4na/gh-patch/cmd/gh-prx@latest
```

## Examples

PR body から marker block を読み取る:

```sh
gh-prx body read 123 --marker ai-summary
gh-prx body read 123 --marker ai-summary --plain
```

PR body の marker block を差し替える:

```sh
gh-prx body write 123 --marker ai-summary --file summary.md
cat summary.md | gh-prx body write 123 --marker ai-summary -
```

GitHub を更新せずに write 結果を preview する:

```sh
gh-prx body write 123 --marker ai-summary --file summary.md --dry-run
```

PR comment を読み取り・更新する:

```sh
gh-prx comment read 123 --marker ai-review
gh-prx comment write 123 --comment-id 123456 --file review.md
gh-prx comment upsert 123 --marker ai-review --file review.md
```

## Conservative Defaults

デフォルトでは、marker が存在しない場合に新しいブロックを挿入せず失敗します。挿入が意図した操作である場合だけ、明示的に opt in してください。

```sh
gh-prx body write 123 --marker ai-summary --file summary.md --insert-if-missing
```

複数の comment が同じ marker を含んでいる場合、`gh-prx` はどれも更新せず、retry 用の candidate comment ID を表示します。marker-scoped update として retry する場合は、marker も明示してください。

```sh
gh-prx comment write 123 --comment-id 123456 --marker ai-review --file review.md
```

## Diff Output

write は、管理対象ブロックに対する compact な line diff を表示します。先頭 2 列は marker block 内での old / new line number です。

```diff
<!-- ai-summary:start -->
old | new | op | content
  1 |     |  - | old summary
  2 |     |  - | old risk note
    |   1 |  + | new summary
    |   2 |  + | new risk note
<!-- ai-summary:end -->
```

差し替え後の内容が既存内容と同じ場合、GitHub へ更新せず no-op として検出し、exit code `5` で終了します。

## JSON

machine-readable output が必要な場合は `--json` を渡します。

```sh
gh-prx body write 123 --marker ai-summary --file summary.md --json
```

例:

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

このリポジトリは GitHub Actions で `gofmt`、`go test ./...`、`./cmd/gh-prx` と `./cmd/gh-patch` の build、GitHub Actions workflow に対する `zizmor` audit を実行します。
