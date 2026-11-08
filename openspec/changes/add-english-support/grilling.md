# grilling 記録: add-english-support

利用者の依頼は「本システム一式を英語対応し、Web表示をi18n化し、GitHub Pagesを日英両対応にする」である。CLIの出力、エージェントのコメント（`comments.language`）、Telegramの通知は、既に英語で出せる。日本語だけなのは、Web表示、Androidのアプリ、GitHub Pages、READMEと文書である。第1ラウンドの後に事実の洗い出しを行い、その結果から第2ラウンドを組んだ。

## 決定

| 決定 | 選択 | 出典 |
|---|---|---|
| 英語にする範囲 | Web表示、Androidのアプリ（画面と通知）、GitHub Pages、README、利用者向けの手引き（`docs/GUIDE.md`）。開発用の文書（SPEC、ARCHITECTURE、DECISIONS、ROADMAP、docs/DESIGN、OpenSpec）は日本語のまま | 第1ラウンド Q1 |
| Web表示の言語の決め方 | 既定はブラウザの言語で決め、日本語なら日本語、それ以外は英語にする。設定の画面で「自動・日本語・English」を選べ、選んだ値はブラウザごとに保存する | 第1ラウンド Q2 |
| GitHub Pagesの構成 | 英語をルート（`/`）に、日本語を `/ja/` に置く。両方に切り替えのリンクと `hreflang` を付け、自動の転送はしない | 第1ラウンド Q3 |
| READMEと手引きの英語版の置き方 | `README.md` を英語にし、日本語を `<README.ja.md>` に移す。手引きも同じ規則で、`docs/GUIDE.md` を英語、`<docs/GUIDE.ja.md>` を日本語にする。冒頭に互いへのリンクを置く | 第1ラウンド Q4 |
| Web表示のi18nの実装 | 自前の型付きの辞書にし、依存を足さない。片方の言語にしか無いキーは型検査の誤りにする。数の表し分けと日時は標準の `Intl` で作る | 第1ラウンド Q5 |
| Androidのアプリの言語 | アプリ自身の画面と通知は端末の言語に従う。WebViewの中のWeb表示は、Web表示の規則に従う | 第1ラウンド Q6 |
| 英語の理由のバッジ | 語だけでは分かりにくい理由にだけ短い名前を付ける。`question`、`plan-questions`、`conflict` は語だけにし、`verify` checks、`plan` approval、`error` run failed、`permission` denied、`push` failed、`ci` failed とする | 第2ラウンド Q1 |
| 言語の切り替えの置き場所 | 設定の画面を全員に開き、全員が使える「表示」の欄に言語を置く。管理者向けの欄は管理者だけに出す。ログインの画面にも、もう一方の言語へ切り替えるリンクを置く | 第2ラウンド Q2 |
| Web表示がissueに投稿する文言の言語 | 計画の質問への返答と再実行の依頼は、表示の言語に合わせる。サーバーは変えない | 第2ラウンド Q3 |
| `docs/SECURITY.md` | 英語版も作る。`docs/SECURITY.md` を英語、`<docs/SECURITY.ja.md>` を日本語にする | 第2ラウンド Q4 |
| 通知 | 変えない。Telegramの通知は `comments.language` に従い、アプリのプッシュ通知は言語に依存しない | 第2ラウンド Q5 |

## 後回しにしたもの

- 開発用の文書（SPEC、ARCHITECTURE、DECISIONS、ROADMAP、docs/DESIGN、PRIOR_ART、docs/examples）の英訳
- Telegramの通知の言語を `comments.language` と別に選ぶ設定

## 状態

frontierは空。共有理解は2026-11-08に確認済み。
