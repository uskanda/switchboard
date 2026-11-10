# Proposal

## Why

計画の質問の図は、Mermaidの `flowchart`、`stateDiagram-v2`、`sequenceDiagram` に限られている（ADR-066）。流れや状態は描けるが、利用者が知りたいのは多くの場合「GUIやWeb画面が実際にどう変わるか」であり、流れ図では直感的に伝わらない。GitLabはアップロードのAPIで画像を本文の任意の位置に置けるので、まずGitLabで、問いと選択肢へ画面のスクリーンショットを付けられるようにする。

## What Changes

- 計画の実行の宣言で、問いに1枚、選択肢ごとに1枚の画像のファイル名を任意で受け取る。1つの問いの図は、Mermaidか画像のどちらか1つにする
- 計画の実行が `.switchboard/plan-images/` に書いた画像を、実行のあとの検査でworktreeを戻す前に集める。PNG、JPEG、WebPで、1枚5MBまで、1回の質問で10枚までとする
- GitLabのプロジェクトでは、画像をアップロードし、問いの画像は本文と選択肢の間に、選択肢の画像はその選択肢の行の直後に置く。付けられなかった画像は、コメントに1行書き、監査ログに `<plan.image_dropped>` を残す
- GitHubのプロジェクトは今のまま（Mermaidだけ）とする
- 撮影のサブコマンド `<switchboard shot>` を足す。ホストのChrome/Chromiumで `localhost` か `127.0.0.1` のページを撮り、撮る前のJSの実行と、枠、矢印、文字の注記ができる。Go側にCDPのライブラリを1つ足す
- 計画の実行への指示に、画像の項を足す。画面の変化はスクリーンショット、流れや状態はMermaidを目安に選ばせる。変更後の見た目は、①注記、②撮る前のDOMとCSSの書き換え、③worktreeの一時的な試作、の順に、足りるところで止めさせる。項を入れるのは、GitLabのプロジェクトでブラウザが見つかったときだけとする
- 設定に `<agent.browser>`（ブラウザの実行ファイル）と、プロジェクトごとの `<projects[].plan_prototype>`（③の試作を許す。既定は無効）を足す
- 計画の実行にも `allow_tools_extra` を渡す。`<projects[].plan_prototype>` が有効なプロジェクトでは、計画の実行に `Edit` と `Write` も許す。worktreeを戻す処理は今の「計画の実行のあとの検査」をそのまま使う
- 受信箱のAPIに、issueのプロジェクトのGitLabのアップロードの画像を受信箱のトークンで取る口を足す。Web表示は、最新の質問の問いのカードで、その画像を縮めて描き、押すと元の大きさで開く
- **BREAKING**（spec上）：計画の実行に追加の許可を加えない、という `agent-run` の要件を改める

## Capabilities

### New Capabilities

- `screenshot`: 撮影のサブコマンド `<switchboard shot>` の引数、開けるURL、注記、ブラウザの見つけ方

### Modified Capabilities

- `plan-mode`: 計画の実行の権限（追加の許可と試作の `Edit/Write`）、実行のあとの検査の前の画像の回収、宣言の画像の欄、質問のコメントへの画像の置き方、画像の指示
- `agent-run`: 追加の許可を使う実行に、計画の実行を加える
- `config`: `<agent.browser>` と `<projects[].plan_prototype>` の設定
- `inbox-api`: 質問の画像を取る口
- `web`: 問いのカードでの画像の描画

## Impact

- 計画の実行：`internal/agent/planrun.go`（宣言のスキーマ、画像の回収と検査、コメントの組み立て）と `internal/agent/run.go`（許可）
- 指示：`internal/prompt/prompt.go`（画像の項）
- `internal/forge`（`Attacher` の、本文の位置を指定して置くアップロード。GitLabだけ）
- 新しい `internal/shot`（CDPでの撮影と注記）と、`cmd` のサブコマンド。`go.mod` にCDPのライブラリが増える
- `internal/config`（新しい2項目と検査）、`internal/inboxserve`（画像の口）、`inboxapi`、`web/internal/server`、`web/ui`（問いのカードの画像）
- `docs/DECISIONS.md`：ADR-051の「撮影専用の設定の項目は作らない」と、計画の実行は読むだけという前提を改める新しいADR
- `docs/ROADMAP.md`：sandboxの中でサーバーとブラウザを1回の呼び出しで動かせるかの検証項目
- 利用者向けの文書：`docs/GUIDE.md`、`docs/GUIDE.ja.md`、`docs/SECURITY.md`、`docs/SECURITY.ja.md`
