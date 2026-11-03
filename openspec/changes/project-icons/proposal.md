# Proposal

## Why

Webの受信箱は、プロジェクトを参照の文字（`o/r#12`）とフォージのバッジだけで出している。プロジェクトが増えると、一覧で行を見分けにくい。issue #105で、プロジェクトのアイコンを参照のそばに出すよう求められた。

## What Changes

- 参照の前に16pxの角丸のアイコンを置く。対象は、一覧の表とカード、1件の表示の見出し、プロジェクトの節、追加の指示の欄、新規issueとプロジェクト追加のトースト
- アイコンは、既定ではフォージから取る。GitLabはプロジェクトのアバター（無ければグループのアバター）、GitHubはオーナーのアバターを使う
- config.yamlのプロジェクトに`icon`を足す。画像のURL（`https://`）、端末の画像ファイル（`/`か`~/`）、絵文字1文字のどれかを書け、フォージのアバターより優先する
- inboxサービスが画像を取って端末に保存し、`GET /v1/project-icon`と`GET /v1/project-icons`で返す。switchboard-webは`/api/project-icon`と`/api/project-icons`で同じものを返す。CSPは変えない
- アイコンが無いとき、取れないとき、まだ取っていないときは、プロジェクト名の頭文字を出す丸を置く

## Capabilities

### New Capabilities

### Modified Capabilities
- `config`: プロジェクトの`icon`を足し、書き方を検める
- `inbox-api`: プロジェクトのアイコンの画像と絵文字を返す2つのエンドポイントを足す
- `web`: 参照の前にプロジェクトのアイコンを出す。switchboard-webにアイコンの2つのルートを足す

## Impact

- `internal/forge`（`Avatars`、GitHubとGitLabのアダプタ）
- `internal/inboxserve`、`inboxapi`、`internal/config`、`cmd/switchboard/inboxserve.go`
- `web/internal/server`、`web/ui`の`IssueRef`と新しい`ProjectIcon`
- `docs/DECISIONS.md`（ADR-049）、`docs/SECURITY.md`、`docs/SPEC.md`、`DESIGN.md`、`config/config.example.yaml`
- 中継に流すものは変えない。受信箱のトークンに新しい権限は要らない
