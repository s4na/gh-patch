# gh-prx

[English](README.md)

`gh-prx` は、GitHub Pull Request の body や PR comment に含まれる名前付き Markdown ブロックを、安全に読み取り・更新するための小さな Go CLI です。

人間だけでなく LLM agent にも扱いやすいように設計されています。agent に PR body 全体や長い comment 全体を読ませて書き換えさせるのではなく、HTML comment marker で管理対象ブロックを示し、そのブロックだけを read / replace できます。

```md
<!-- section:start -->
...
<!-- section:end -->
```

GitHub API 上は Markdown field 全体を更新しますが、`gh-prx` は利用者から見える操作を名前付き read/write ブロックとして扱い、write の前後に diff を表示します。
まだ marker が入っていない既存の PR body 向けには、expectation guard 付きで明示的な行範囲を差し替えることもできます。

## Install

このリポジトリから gh extension としてインストールする場合:

```sh
gh extension install s4na/gh-patch
gh patch body read 123 --marker section
```

GitHub CLI はリポジトリ名から extension command を決めるため、このリポジトリは `gh patch ...` として公開されます。コマンドを正確に `gh-prx ...` として使いたい場合は、同じ binary を `gh-prx` extension repository から公開してください。このリポジトリには、source-based gh extension install 用の root `gh-patch` wrapper も含まれています。

tap へ release された後に Homebrew でインストールする場合:

```sh
brew install s4na/tap/gh-prx
gh-prx body read 123 --marker section
```

source からインストールする場合:

```sh
go install github.com/s4na/gh-patch/cmd/gh-prx@latest
```

## Examples

PR body から marker block を読み取る:

```sh
gh-prx body read 123 --marker section
gh-prx body read 123 --marker section --plain
```

PR body の行範囲を読み取り、現在内容の hash を取得する:

```sh
gh-prx body read 123 --range 12:18 --json
```

PR body の marker block を差し替える:

```sh
gh-prx body write 123 --marker section --file section.md
cat section.md | gh-prx body write 123 --marker section -
```

GitHub を更新せずに write 結果をプレビューする:

```sh
gh-prx body write 123 --marker section --file section.md --dry-run
```

既存の PR body の行範囲を preview / 差し替えする:

```sh
gh-prx body lines 123 --range 12:18 --file section.md --dry-run
gh-prx body lines 123 --range 12:18 --file section.md --expect-sha <body_sha>
```

PR comment を読み取り・更新する:

```sh
gh-prx comment read 123 --marker section
gh-prx comment write 123 --comment-id 123456 --marker section --file section.md
gh-prx comment upsert 123 --marker section --file section.md
```

comment 全体の置換は明示的に指定します:

```sh
gh-prx comment write 123 --comment-id 123456 --whole --file comment.md
```

## Conservative Defaults

デフォルトでは、marker が存在しない場合に新しいブロックを挿入せず失敗します。挿入が意図した操作である場合だけ、明示的に opt in してください。

```sh
gh-prx body write 123 --marker section --file section.md --insert-if-missing
```

長期的に管理する対象には、周辺の本文編集に強い marker block を推奨します。line-range replacement は、まだ marker がない既存 PR body のための escape hatch です。実際に `body lines` で更新する場合は、`--expect-sha`、`--expect-file`、`--force` のいずれかが必須です。`--dry-run` は guard なしで実行できるため、人間や agent が先に差し替え内容を確認できます。

```sh
gh-prx body read 123 --range 12:18 --json
gh-prx body lines 123 --range 12:18 --file section.md --expect-sha <body_sha>
gh-prx body lines 123 --range 12:18 --file section.md --expect-file old-section.md
```

marker 指定で comment を読む場合に複数の comment が同じ marker を含んでいると、`gh-prx` は暗黙に1つを選ばず、retry 用の candidate comment ID を表示します。`comment upsert` はより狭く、current GitHub user が作成した marker comment だけを対象にします。marker-scoped write として retry する場合は、marker も明示してください。

```sh
gh-prx comment write 123 --comment-id 123456 --marker section --file section.md
```

## Diff Output

write は、管理対象ブロックに対する compact な line diff を表示します。line number は `-` 行では old 側、`+` 行では new 側の番号です。

```diff
<!-- section:start -->
op | line | content
 - |    1 | old content
 - |    2 | old line
 + |    1 | new content
 + |    2 | new line
<!-- section:end -->
```

差し替え後の内容が既存内容と同じ場合、GitHub へ更新せず no-op として検出し、exit code `5` で終了します。

## JSON

machine-readable output が必要な場合は `--json` を渡します。

```sh
gh-prx body write 123 --marker section --file section.md --json
```

例:

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

このリポジトリは GitHub Actions で `gofmt`、`go test ./...`、`./cmd/gh-prx` と `./cmd/gh-patch` の build、GitHub Actions workflow に対する `zizmor` audit を実行します。
