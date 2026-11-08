# Proposal

## Why

計画の質問では、選択肢が本文の中の文として書かれ、Web表示は推奨の文から推奨の選択肢を推測している（#190）。問いが何を問うているかは文だけから読み取るしかなく、1問の本文の中央値は約460字で、コードの名前も多い。選択肢と推奨を構造として持ち、要るときは図で問いを示せば、読む量と読み違いを減らせる。

## What Changes

- 計画の実行の宣言で、問いに選択肢の文（2〜6個）、推奨のラベル、理由の文、任意の図を持たせる。今の推奨の文（`recommendation`）は、推奨のラベルと理由の文に置き換える
- switchboardが選択肢にA、B、C…のラベルを振り、選択肢、推奨、図を決まった形でコメントに書く。推奨のラベルが選択肢の範囲から外れた宣言は、出力の誤りとして扱う
- 図はMermaidの `flowchart`、`stateDiagram-v2`、`sequenceDiagram` に限る。ほかの種類の図と、CSSを変える指示（`classDef`、`style`、`linkStyle`、`click`、`%%{`）を含む図は捨て、監査ログに残す。強調には `:::focus` だけを許す
- 計画の実行のsystem promptに、図を付ける目安、図の種類と大きさの制限、1問1組の選択肢の書き方を足す
- Web表示は、最新の質問の問いのカードの本文と選択肢の間に、図を描く。描けない図は、元の文を折りたたみで見せる。図は押すとダイアログで元の大きさに開く
- **BREAKING**（セキュリティの方針）：switchboard-webのCSPに `style-src 'self' 'unsafe-inline'` を足す。ADR-055の「CSPを緩めない」を、新しいADRで改める

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `plan-mode`: 計画の実行の終了の宣言の形、質問のコメントの書き方、質問の図の検査
- `web`: 問いの図の描き方と描く場所、推奨の段落の頭の印、Web表示のCSP

## Impact

- `internal/agent/planrun.go`：宣言のスキーマ、検査、質問のコメントの書き方
- `internal/prompt/prompt.go`：計画の実行のsystem prompt
- `internal/agent/messages.go`：推奨の見出し（日本語と英語）
- `web/ui`：`mermaid@12.2.0` の追加、図の部品、`vite.config.ts` の分け方、問いのカード
- `web/internal/server/server.go` とそのテスト：CSPの文字列
- `docs/DECISIONS.md`（ADRの追加）、`docs/SECURITY.md`、`docs/SECURITY.ja.md`、DESIGN.md
