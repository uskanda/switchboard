# Design

## Context

計画の実行は `PlanSchema` の宣言で終わり、質問は題、本文、推奨の文の3つの欄を持つ（`internal/agent/planrun.go`）。switchboardは質問を `❓ **Qn** - **題**`、本文、`➡️ 推奨の文` の形でコメントに書く。Web表示は、そのコメントから選択肢の組を推測で読む。推奨は、推奨の文で最初に出てくるラベルとみなす（`web/ui/src/lib/planQuestions.ts`）。#194で、最新の質問の問いは1つずつカードとして描かれ、推奨の理由は推奨の選択肢の行の下に出る。switchboard-webのCSPは `default-src 'self'` で、スタイルのインラインも止めている。動機はproposal.mdの「Why」を参照する。

## Goals / Non-Goals

**Goals:**

- 選択肢と推奨を宣言の構造で受け、コメントには今のWeb表示が読める形で書く。古い形のコメントも今までどおり読める
- 図は、Go側とWeb表示の両方が同じ規則によって検査する。描くのは検査を通った図だけとし、置き場所を1か所に決める

**Non-Goals:**

- 範囲や方向を決める問いへの図の必須化と、問いの区分（#191）
- 前のラウンドや計画のコメントでの図の描画

## Decisions

### 宣言のスキーマ

質問の項目を、`title`、`body`、`options`（文字列の配列、2〜6個）、`recommended`（`^[A-F]$`）、`reason`、任意の `diagram` にする。`recommendation` は無くす。`recommended` が選択肢の数の範囲にあるかはJSONのスキーマで書けないので、`planDeclaration` が検査し、外れていれば今の空の題と同じく出力の誤りにする。

ラベルをswitchboardが振るので、エージェントがラベルを抜かしたり重ねたりする誤りは起きない。ほかの案として、`options` を `{label, text}` の配列にする形があった。この形はラベルの検査が増えるので採らない（grilling第3ラウンドQ13）。

### コメントの形

`planQuestions` は、各質問を次の順に書く。題の行、本文、検査を通った図（`mermaid` のコードブロック）、`A: 〈文〉` の選択肢の行、推奨の段落である。推奨の段落は、`➡️` と、太字にした `推奨：B` で始め、そのあとに理由を書く。英語では `Recommended: B` とする。今のWeb表示の `defaultAnswers` は、推奨の段落で最初に出てくるラベルを推奨とするので、頭の `B` がそのまま推奨になる。

この形なら、隠しのJSONを埋めなくても、Web表示は推奨を確実に読める。フォージの画面でも、そのまま読める文になる。

### 図の検査（Go側）

`<diagramProblem>` は、図を書かない理由を返す。検査することは次の4つである。

- 外側の囲み：`mermaid` のコードブロックの囲みで全体を包んだ図は、囲みを1つだけ外してから検査する
- 区切りの残り：外したあとも ` ``` ` を含む図は捨てる。コメントのコードブロックが途中で閉じるためである
- 種類：最初の空白でない行の頭が `flowchart`、`stateDiagram-v2`、`sequenceDiagram` のどれでもない図は捨てる
- スタイルの指示：行頭の `classDef`、`style`、`linkStyle`、`click` か、`%%{` を含む図は捨てる

捨てたときは、監査ログに `<plan.diagram_dropped>` をissue、実行、質問の番号、理由とともに残す。Mermaidの書き方の誤りは、計画の実行の中では確かめられないので、Web表示の描画の失敗として扱う。

### Web表示の読み取り

`parsePlanQuestions` は、問いの本文のうち最初の組より前にある、最初の `mermaid` のコードブロックを図の部分（`kind: "diagram"`）として取り出す。ほかのコードブロックは、今までどおり文として残す。図の部分を描くのは `PlanQuestions` だけで、`PlanQuestions` は最新の質問にしか使わない。前のラウンド、人のコメント、ほかの投稿のMermaidは、`Markdown` の部品がコードとして描く。

推奨の理由を推奨の選択肢の下に出すときは、推奨の段落の頭の太字の `推奨：〈ラベル〉` と `Recommended: 〈label〉` を除く。

### 図の部品

`<Diagram>` の部品は、図を描く前に、Go側と同じ種類とスタイルの指示の検査をする（二重の検査）。検査を通った図だけを、`import("mermaid")` で読み込んだMermaidで描く。初期設定は次のとおりにする。

- `startOnLoad: false`、`securityLevel: 'strict'`、`layout: 'dagre'`
- テーマ：OSの配色に合わせ、明るいときは `neutral`、暗いときは `dark` にする。`matchMedia` の変化で描き直す
- 強調：描く前に、検査を通った元の文に `classDef focus` を1行足す。太さを `stroke-width:3px` にし、色はページの `--color-primary` の値にする。`sequenceDiagram` には足さない

描いたSVGは、カードの幅に収まるように縮めて置き、押すとネイティブの `<dialog>` で元の大きさに開く。SVGは、`DOMParser` で切り出した `<svg>` の要素だけをページに入れる。`DOMParser` の文書はスクリプトを動かさない。リポジトリのlintは `dangerouslySetInnerHTML` を禁じていて（PRODUCT.mdの原則2）、その決まりは残す。issueを経た文字列をマークアップとして入れるのは、この部品だけにする。検査に通らない図と、描くのに失敗した図は、「図を描けませんでした」と、元の文を入れた `<details>` を出す。

### 読み込みの分け方

`vite.config.ts` の `manualChunks` は、今は `node_modules` の全部を `vendor` にまとめる。このままではMermaidとその依存（elkjs、d3など、約1.5 MB、gzip）が、毎回の読み込みに入ってしまう。そこで、入口から静的にたどれるモジュールだけを `vendor` に入れる。`getModuleInfo` の `importedIds` を入口からたどって集め、動的な読み込みの先にしか無いモジュールは、Rollupの既定の分け方に任せる。結果は、ビルドの出力でMermaidが `vendor` に入っていないことで確かめる。

### CSPとADR

`server.go` のCSPに `style-src 'self' 'unsafe-inline'` を足し、`script-src` は `default-src 'self'` のまま残す。ADR-066を足し、ADR-055の「CSPを緩めない」を、スタイルについてだけ改める。

ほかの案は次の3つで、採らない。

- CSPを変えずに、Mermaidの `<style>` を別のスタイルシートへ移す回避策：Mermaidの内部の処理の順に頼り、図1つに70〜110件の違反の報告が出る
- 画像（`<img>`）として出す案：描く段階で同じ回避策が要る
- Web表示では描かない案：図を見られるのはフォージの画面だけになる

`docs/SECURITY.md` と `docs/SECURITY.ja.md` のCSPの記述も直す。

## Risks / Trade-offs

- HTMLの差し込みが別にあると、インラインのスタイルで見た目を偽られる → Web表示は生のHTMLを描かない（PRODUCT.mdの原則2）。描くMermaidは、検査を通った最新の質問の図だけである。スクリプトと外への読み込みは今までどおり止まる
- Mermaidの不具合で、消毒をすり抜けるSVGが出る → 版を `12.2.0` に固定し、上げるときは差分を読んでから上げる
- FirefoxとSafariでは確かめていない → 実装のときにPlaywrightのFirefoxとWebKitで確かめる。手元に入れられなければ、未確認としてPRに書く
- エージェントの図と本文が食い違う → 図は任意の補助で、正とするのは本文と選択肢である。プロンプトで、図は本文の補助として書くよう伝える
- 古いエージェントが投稿した、推奨が文だけの質問 → Web表示の今の推測の読み取りで、今までどおり読める

## Migration Plan

核の `switchboard` と `switchboard-web` を同じ版にそろえて入れ替える。古いWeb表示でも、新しい形のコメントは読める（選択肢の行と推奨の段落の形は今までと同じため）。ただし図はコードとして出る。戻すときは前の版に入れ替える。設定の変更は無い。
