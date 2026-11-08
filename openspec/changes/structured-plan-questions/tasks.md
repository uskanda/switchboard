# Tasks

## 1. 宣言とコメント（Go）

- [x] 1.1 `PlanSchema` と `PlanSkipSchema` の質問を、`options`（2〜6個）、`recommended`、`reason`、任意の `diagram` にする。`planDeclaration` で、`recommended` が選択肢の数の範囲にあるかを検査する。範囲の外、選択肢1つ、推奨の欠けの宣言が誤りになることを、`planrun_test.go` の失敗するテストを先に書いてから通す
- [x] 1.2 `<diagramProblem>` を足し、外側の囲みの外し方、区切りの残り、種類、スタイルの指示を検査する。仕様の「質問の図の検査」の各シナリオと、`:::focus` だけの図が通ることをテストで確かめる
- [x] 1.3 `planQuestions` を、題の行、本文、図、`A: 〈文〉` の選択肢、推奨の段落の順に書くよう直す。`messages.go` に推奨の見出しを日本語と英語で足す。両方の言語の書き方と、図のある質問と無い質問をテストで確かめる
- [x] 1.4 `planQuestions` で図を捨てたときに、監査ログに `<plan.diagram_dropped>` を残す。issue、実行、質問の番号、理由が残ることをテストで確かめる
- [x] 1.5 `planSystem` に、1問1組の選択肢、図の目安、種類、大きさ、`:::focus` の指示を足し、宣言の項目の説明を新しい形に直す。`prompt_test.go` で、指示がsystem promptに入ることを確かめる

## 2. 図の読み取りと描画（Web表示）

- [x] 2.1 `parsePlanQuestions` で、最初の組より前の最初の `mermaid` のコードブロックを図の部分として取り出す。図のある問い、図の無い問い、組の後ろのコードブロック、古い形のコメントを `planQuestions.test.ts` で確かめる
- [x] 2.2 `OneQuestion` で推奨の理由を選択肢の下に出すとき、頭の太字の `推奨：〈ラベル〉` と `Recommended: 〈label〉` を除く。両方の言語をテストで確かめる
- [x] 2.3 `web/ui` に `mermaid@12.2.0` を固定の版で足し、`<Diagram>` の部品から `import("mermaid")` で読み込む。図の無い待ちでは読み込まないことを、Mermaidを差し替えたテストで確かめる
- [x] 2.4 `<Diagram>` で、Web表示の側の検査、`strict` と `dagre` の初期設定、OSの配色に合わせたテーマ、`classDef focus` の足し方を作る。差し替えたMermaidに渡る設定と元の文をテストで確かめる
- [x] 2.5 `<Diagram>` は、検査を通らない図と描くのを失敗した図の代わりに、「図を描けませんでした」と折りたたんだ元の文を出す。両方の場合をテストで確かめる
- [x] 2.6 `<Diagram>` で図を押すと、元の大きさの図をダイアログで開くようにする。開くことと閉じることを、テストで確かめる
- [x] 2.7 `<Diagram>` で、OSの配色が変わったら描き直す。変わる前と後のテーマを、テストで確かめる
- [x] 2.8 `Markdown` の部品が、前のラウンド、人のコメント、ほかの投稿のMermaidをコードのまま描くことを、ページのテストで確かめる
- [x] 2.9 `vite.config.ts` の `manualChunks` を直し、`vendor` へ入れるのは入口から静的にたどれるモジュールだけにする。`pnpm build` の出力で、Mermaidとその依存が `vendor` に入らないことを確かめる

## 3. CSPと文書

- [x] 3.1 `server.go` のCSPに `style-src 'self' 'unsafe-inline'` を足す。`server_test.go` と `icons_test.go` の期待する値を直し、`go test ./...` で確かめる
- [x] 3.2 `docs/DECISIONS.md` にADR-066を足し、ADR-055の「CSPを緩めない」をスタイルについてだけ改める
- [x] 3.3 `docs/SECURITY.md` と `docs/SECURITY.ja.md` の、CSPとMermaidの図についての記述を、同じ変更で直す
- [x] 3.4 DESIGN.mdに「問いの図」を足し、図の置き場所、縮め方、拡大のダイアログ、強調の枠、描けないときの見た目を書く

## 4. 確認

- [x] 4.1 `defineMessages` に図の文言を日本語と英語の両方で書き、英語の表示のテストで確かめる
- [x] 4.2 `make verify` を通す
- [x] 4.3 Playwrightで、ビルドしたWeb表示を同じCSPを付けたモックのサーバーで開き、図のある問いをデスクトップとスマホの幅、ダークモードで確かめる。FirefoxとWebKitも入れられれば同じように確かめ、入れられなければ未確認としてPRに書く
