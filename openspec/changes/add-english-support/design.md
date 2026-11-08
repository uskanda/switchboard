# Design

## Context

動機はproposal.mdのWhyにある。今の状態と制約は次のとおり。

- Web表示（`web/ui/src`）は、約300の日本語の文言を21のファイルに直書きしている。i18nの仕組みと依存は無い。`web/ui/index.html` は `lang="ja"` に固定している
- 文言の一部は、`lib/api.ts` の純粋な関数（`age`、`elapsed`、`timeText`、`queueText`、`progressText`、`autoMergeReason`）が作る。テストはこれらの日本語の出力を直接確かめている
- テストは約250種の日本語の文言で要素を探す。jsdomの `navigator.language` は `en-US` に固定され、`localStorage` を消すテストと `vi.unstubAllGlobals()` を呼ぶテストがある
- ログインの画面は、`App.tsx` の `AuthGate` の内側の部品より外に出る。言語の仕組みは `AuthGate` より外に置く必要がある
- 設定の画面は、ログインが有効なとき管理者だけが読める。ヘッダの設定へのリンクも管理者だけに出している
- Androidのアプリ（`app/src`）は約20の日本語の文言を持つ。通知の拒否の文は、Reactの部品の外の背景のタスクが作る。i18nの依存は無い
- 紹介ページは手書きの1枚の `site/index.html` で、`pages.yml` が `site/` をそのまま公開する
- サーバーの誤りの文、CLI、エージェントの投稿はすでに英語か `comments.language` に従う。Goは変えない

## Goals / Non-Goals

**Goals:**

- 文言の足し忘れを型検査で見つける。片方の言語にしか無いキーはビルドを止める
- 今のテストを書き換えずに通す。英語の表示は新しいテストで確かめる
- 部品のファイルを並行して直せる構造にする。全文言を1つのファイルに集めない

**Non-Goals:**

- 日本語と英語より多くの言語。仕組みは2言語に限って単純にする
- サーバーの誤りの文の翻訳。誤りのコードで日本語の文を選ぶ今の仕組みに英語を足すだけにする
- 開発用の文書の英訳（grilling.mdの後回し）

## Decisions

### 1. 辞書は部品のファイルの中に置き、共通の語だけを1か所に集める

各部品は、自分の文言を `defineMessages({ ja: {...}, en: {...} })` で同じファイルに書き、`useMessages(defs)` で今の言語の側を受け取る。`defineMessages` は `en` の型を `NoInfer<T>` で `ja` の型に合わせ、キーの欠けと関数の引数の違いを型検査の誤りにする。値は文字列か、数や名前を受けて文字列かReactの要素を返す関数とする。文の中にリンクや `<code>` があるときは、要素を引数に取る関数で語順ごと言語に任せる。

複数の部品が使う語（状態の語、「受信箱のサービスに届きません」、「失敗しました」の文、「やめる」）は `web/ui/src/lib/i18n` の共通の辞書に置く。

代わりの案として、全文言を言語ごとの2つのファイルに集める形を考えた。キーの一覧は見やすいが、部品と文言が離れ、並行して直すと同じファイルを奪い合うため採らない。react-i18nextは、2言語と型の検査には大きすぎるため採らない（grillingの第1ラウンドのQ5）。

### 2. 言語の状態はモジュールの1つの置き場が持ち、Reactの外からも読める

言語の状態は、`web/ui/src/lib/i18n` のモジュールが持つ。Providerは置かず、部品は `useSyncExternalStore` で購読する。そのため、ログインの画面や部品だけを描くテストでも、Providerで包まずに言語を読める。言語は、保存した選択と `navigator.languages` から決める。保存した選択は `localStorage` の `<switchboard.language>` に置き、値は `ja` か `en` とし、無ければ自動とする。別のタブでの変更は `storage` のイベントで受ける。`AppRoutes` の `useDocumentLang` が、言語が決まるたびに、描画の前に `document.documentElement.lang` を書き換える。

Reactの外のコード（`lib/api.ts` の時間の関数、トーストを出す関数）は、引数で言語を受け取る。引数を取れない呼び出し元のために、その時点の言語を返す `currentLang()` も用意する。

### 3. テストは日本語を既定にする

`vitest` の `setupFiles` の先頭に、`window.navigator` の `language` と `languages` を `ja-JP` に定義し直すファイルを足す。インスタンスへの `Object.defineProperty` は、`unstubAllGlobals`、`restoreAllMocks`、`localStorage.clear()` の後も残る。英語のテストは、描画の前に `localStorage` へ `en` を保存するヘルパーで切り替える。

### 4. 英語の書き方

英語の語は、CLIとエージェントの英語の投稿（`internal/agent/messages.go` の `english`）とそろえる。

| 日本語 | 英語 |
|---|---|
| 受信箱 | inbox（見出しはInbox） |
| 返答待ち、待ち | Waiting |
| 実行中 | Running |
| 順番待ち | Queued |
| 完了（close待ち） | Done (waiting for close) |
| 返答、返答を送る、送りました | Reply、Send reply、Sent |
| 再実行する | Run again |
| 承認する、マージする | Approve、Merge |
| 端末、プロジェクト、エージェント、実行 | host、project、agent、run |
| 変更要求、フォージ、中継 | change request、forge、relay |
| 計画、計画の質問、計画の承認 | plan、plan questions、plan approval |
| 追加の指示 | additional instructions |
| 設定、表示、言語、自動 | Settings、Display、Language、Automatic |
| やめる、読み込んでいます | Cancel、Loading |

時間はCLIの短い形（`5m`、`3h`、`2d`）にそろえる。

| 状態 | 日本語 | 英語 |
|---|---|---|
| 実行中 | 2分経過、始めて1分未満 | 2m elapsed、started just now |
| 待ち | 5分前から待ち、待ちになって1分未満 | waiting 5m、waiting under 1m |
| 順番待ち | 3分前から順番待ち | queued 3m |
| 完了 | 1時間前に完了、完了して1分未満 | done 1h ago、done just now |

日時は、日本語では今の `M/D HH:MM`、英語では `Nov 8, 14:30` の形にする。どちらも端末の時刻を使い、`Intl` の既定のロケールに依存しない形で組み立てる。件数は、英語では `1 issue`、`2 issues` のように単数と複数を分ける。

英語の文言は、`en-writing` の規則に従う。日本語の区切りの記号（「・」「（）」「：」「、」）は、英語では `, `、`()`、`: ` に置き換える。

### 5. issueに投稿する文は、投稿したときの表示の言語で書く

`composeReply` と再実行の決まった文は、言語を引数に取る。英語の形は `ui-language` の仕様にある。再実行の文は、英語では `Please run it again.` と `Resolve the conflict with the base branch and push again.` にする。エージェントは両方の言語を読めるため、サーバーとプロンプトは変えない（grillingの第2ラウンドのQ3）。

### 6. 設定の画面を全員に開く

設定の画面は、先頭に全員向けの表示の節（言語の選択）を置き、その下に管理者向けの節を置く。管理者でない人には、管理者向けの節の代わりに `プロジェクトと追加の指示を変えられるのは管理者だけです` を出し、APIは呼ばない。ヘッダの設定へのリンクは全員に出す。APIの権限は変えない。

### 7. Androidのアプリは端末のロケールを読む

`app/src/lib/i18n.ts` に、端末の言語を返す関数と、日本語と英語の文言の表を置く。言語は標準の `Intl` が返すロケールから読み、読めなければReact Nativeの `I18nManager` のロケールを使う。依存は足さない。背景のタスクも同じ関数を呼ぶ。通知のチャンネルとボタンは、アプリを起動するたびに登録し直すため、端末の言語を変えた後の次の起動で切り替わる。テストは言語を引数で渡すか、関数をモックする。

### 8. 紹介ページは2つの静的なHTMLにする

英語の `site/index.html` と日本語の `<site/ja/index.html>` を置く。日本語の版は `../favicon.png` を参照する。各版は、`<link rel="alternate" hreflang>`（`en`、`ja`、`x-default` は英語）と、ヘッダのもう一方の版へのリンクを持つ。手引きとセキュリティへのリンクは、同じ言語の版を指す。生成の仕組みは足さず、`pages.yml` も変えない。

### 9. 文書の2つの版

`README.md`、`docs/GUIDE.md`、`docs/SECURITY.md` を英語にし、今の日本語を `<*.ja.md>` の版に移す。英語の版の文書の一覧では、日本語だけの文書の名前の後ろに「(Japanese)」と書く。日本語の見出しへのアンカーは、同じ言語の版の中でだけ使う。AGENTS.mdに、この3つを変えるときは両方の版を同じ変更で直す規則を足す。

## Risks / Trade-offs

- ［ブラウザが英語の日本語の利用者は、更新後に英語の画面を見る］→ 設定の画面の表示の節で日本語を選べる。READMEの両方の版とGUIDEに、言語の決め方を書く
- ［英語の文言が長く、固定幅の表の列やスマホの幅に収まらない］→ 時間をCLIの短い形にする。英語の表示で、幅375pxと800pxの画面をPlaywrightで確かめる
- ［文書の2つの版がずれる］→ AGENTS.mdの規則で、同じ変更で両方を直す
- ［仕様の要件が引く日本語の文言と、英語の文言の対応が仕様に無い］→ `ui-language` の「文言の言語」の要件で、英語は同じ意味の文言とする。対応は辞書を正とする

## Migration Plan

データと設定の移行は無い。ブラウザの言語が日本語の利用者には、今と同じ日本語の画面が出る。戻すときは、このchangeのコミットを戻す。

## Open Questions

なし。
