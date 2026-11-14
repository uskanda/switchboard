# Tasks

## 1. 検証

- [x] 1.1 `claude -p` の実物で、`--tools` に `WebFetch` を含め、allowに `WebFetch` を書いた計画の実行の設定と、実行ごとのsandboxの組み合わせを試す。外のページを読めるかを確かめ、結果を `docs/ROADMAP.md` の検証項目V66に書く。読めなければ、ここで止めてユーザーに知らせる

## 2. 設定

- [x] 2.1 `config.Agent` と `config.Project` に `PlanWebFetch *bool` を足し、プロジェクト、全体、既定のオンの順に解決するメソッドを足す。どちらも無い、全体だけ `false`、全体が `false` でプロジェクトだけ `true` の3つを設定のテストで確かめる
- [x] 2.2 <projects[].plan_web_fetch> の変化が `projects` の読み直しで反映されることを、読み直しのテストで確かめる
- [x] 2.3 `config/switchboard.example.yaml` に2つの項目を、説明のコメント付きで書く

## 3. 許可とツール

- [x] 3.1 `planSettings` と `planToolsFor` にWebFetchの真偽値を足す。オンでallowとツールに `WebFetch` があってdenyに無いこと、オフで今と同じ結果になることを、`plan_prototype` と撮影の組み合わせも含めてテストで確かめる
- [x] 3.2 `run.go` の計画の実行で、プロジェクトの解決した値を渡す。実装の実行と直す実行のdenyに `WebFetch` が残ることを、既存のテストに足して確かめる
- [x] 3.3 `realclaude` のタグの付いたテストで、WebFetchがオンの計画の実行のinitの `tools` に `WebFetch` があることを確かめる

## 4. 指示

- [x] 4.1 `prompt.WithWebFetch` を足し、WebFetchがオンの計画の実行のsystem promptにだけ、読んだページの扱いの節を差し込む。節の中身を `prompt` のテストで、オンとオフと実装の実行での有無をエージェントのテストで確かめる

## 5. 参照したページの一覧

- [x] 5.1 `internal/claudecode` の `Parse` が、`WebFetch` の `tool_use` の `url` を、重複をまとめて最初の順で `Result` へ集める。拒否された呼び出しを含む出力の断片でテストする
- [x] 5.2 `planTail` に一覧を足す。見出しは日本語と英語の両方を `msg` に持つ。2件、重複あり、23件、200文字を超えるURL、バッククォートを含むURL、0件の各場合を、質問、計画、計画を省いた実装の開始のコメントのテストで確かめる

## 6. 文書

- [x] 6.1 `docs/DECISIONS.md` に、ADR-052の「WebFetchは拒否」を計画の実行について覆すADRを足す
- [x] 6.2 `docs/SECURITY.md` と `docs/SECURITY.ja.md` に、読んだページを経由する持ち出しの脅威（T15）と、その対策（計画の実行だけ、指示、一覧、設定）を同じ変更で足す
- [x] 6.3 `docs/GUIDE.md` と `docs/GUIDE.ja.md` の、実行に渡すツールの説明に、計画の実行のWebFetchと2つの項目を同じ変更で書く
- [x] 6.4 `docs/DESIGN.md` の、実行に渡すツールの節を直す

## 7. 確認

- [x] 7.1 `make verify` が通ることを確かめる
- [x] 7.2 `openspec validate add-plan-web-fetch --strict` が通ることを確かめる
