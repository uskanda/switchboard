# Design

## Context

動機はproposal.mdの「Why」を参照。今のコードで、この設計を縛る点は次のとおり。

- `config.Load` は、`yaml.Node` に読んでから `KnownFields(true)` で構造体に復号し、既定値を当てて検証する。書き込みの口は無い
- エージェント（`internal/agent`）は、`Start` で担当プロジェクトごとに公開かどうかを読み、`UntrustedPolicy` を通ったものだけを `a.projects` に持つ。フォージのクライアントと信頼の判定（`Route`）は、`cmd/switchboard` の `agentRoutes` と `judge` が起動時に作って渡す。以後、`a.projects` は変わらない
- `inbox serve` の `Server` は、起動時に作った `inbox.Inbox` と `Instructions.Projects` を持つ。どちらも設定のプロジェクトで絞っている
- エージェントのクレデンシャルは、`service install` のときに、この端末の担当プロジェクトが使う `token_ref` だけを組む（`service.Credentials`）
- WUI（`web/`）は核と別のモジュールで、`inboxapi` とUnixソケットだけで核とつながる（ADR-037）。POSTの検査（JSONと `Origin`/`Host`）は `web/internal/server` にある
- 土台は `add-web-two-pane-live`。設定の画面（`web/ui/src/pages/SettingsPage.tsx`）は、その変更で作り直した形を前提にする

## Goals / Non-Goals

**Goals:**

- 設定の書き込みを `internal/config` の1か所に閉じ込め、`inbox serve` だけが呼ぶ
- エージェントと `inbox serve` が同じ読み直しの部品を使い、`projects` の入れ替えだけを反映する
- WUIに新しい権限を渡さない。WUIは、これまでどおりトークンと設定ファイルを持たない

**Non-Goals:**

- `projects` 以外の節の読み直し（`forges`、`agent`、`relay`、`notify` など）。クライアントやクレデンシャルの組み直しが要る
- CLIのプロジェクトの追加のコマンド。`config.yaml` を手で書けば足りる
- 追加のときに受信箱のトークンでラベルを作ること。受信箱のトークンの権限は広げない（ADR-038と同じ方針）

## Decisions

### D1. 書き込みは `yaml.Node` への1項目の追記で行う

`internal/config` に、設定ファイルのパスと足すプロジェクトを受ける関数を足す。手順は次のとおり。

1. `filepath.EvalSymlinks` で実体のパスを得て、中身を読み、SHA-256を取る
2. `yaml.Node` に読んで、`projects` の最後の項目が終わる行を探す。その次の行に、最初の項目と同じ字下げで1項目を文字列として差し込む。`projects` は検証で1件以上が必須なので、無い場合は起こらない。flow style（`[...]`）で書かれた `projects` には足さずに誤りを返す
3. 足したあとのバイト列を、`Load` と同じ検証（復号、既定値、`validate`）に通す。そのうえで、既存のプロジェクトとほかの節が変わらず、新しい1件が末尾に足されたことを確かめる
4. 実体のパスの中身をもう一度読み、1のハッシュと違えば `ErrConfigChanged` を返す
5. 元の中身を `<config.yaml.bak>` に書く。新しい中身を同じディレクトリの一時ファイルに書き、`fsync` してから `rename` で置き換える。ファイルの権限は元に合わせる

代わりの案は2つあった。構造体に復号して書き戻す形は、コメントと並びを失う。`yaml.Node` を丸ごと書き出す形は、空行とコメントの桁揃えを消し、末尾のコメントの字下げも変える。どちらも採らず、差し込む位置を探すのにだけ `yaml.Node` を使う。

### D2. 読み直しは `<config.Watcher>` にまとめる

`internal/config` に、設定ファイルを10秒ごとに確かめる `Watcher` を足す。更新の時刻と大きさが変わったときだけ中身を読み、SHA-256が前と違えば `Load` に通す。

- 検証を通らなければ、誤りをログに残し、前の設定を保つ
- 通ったら、`projects` 以外の節を前の設定と比べる。違う節の名前をログに残す
- 新しい `projects` だけを、呼び手に渡す。呼び手の `Config` の `projects` 以外は差し替えない

ハッシュで比べるのは、エディタが同じ中身で保存し直したときに、読み直しを起こさないためである。fsnotifyは使わない。依存を増やさず、シンボリックリンクとエディタの置き換え保存を同じ扱いにできる。

### D3. エージェントは、差分を「受け入れ」と「取り下げ」に分けて反映する

`Agent` に、読み直した `projects` を受け取る口を足す。この端末の担当分について、前の `a.projects` との差を取る。

- 取り下げ：消えたプロジェクトを `a.projects` から外す。`active` の実行には触れないので、実行は最後まで続く。待ち行列（`pending`）の項目は捨てる
- 受け入れ：新しいプロジェクトを「受け入れ待ち」に入れる。照合のたびに、待ちのプロジェクトを順に処理する
  1. `Route` を作る。作り方は起動時と同じで、`cmd/switchboard` から関数として渡す。そのフォージの信頼の判定は、起動時に作ったものを使う。フォージの判定が無ければ（この端末で初めてのフォージ）失敗とする。エージェントの身元の上書き（`token_ref` と `agent_identity`）を持つプロジェクトも、判定とクレデンシャルが起動時に決まるので、再起動まで失敗とする
  2. 公開かどうかを読み、`UntrustedPolicy` を通す。通らなければ、起動時と同じく拒んで監査に残す。拒んだプロジェクトはやり直さない
  3. `path` が無ければcloneする。あれば `origin` のURLを確かめる
  4. 6つの状態ラベルを、`labels.Sync` で作る
  5. `a.projects` に足す
- どこかで失敗したら、受け入れ待ちに残し、ログに残す。やり直しは毎回のポーリングの前に行う。照合はポーリングに含まれるので、specの「次の照合のときに」を満たす。同じ失敗のログは、文が変わるまで繰り返さない

受け入れを照合のときに行うのは、cloneとAPIの呼び出しを、ポーリングと同じ `pollMu` の下で順に行うためである。読み直しの直後に別のゴルーチンで行う案は、ポーリングと同じプロジェクトを同時に触りうるので採らない。

中継の購読は、イベントを担当プロジェクトかどうかで絞っている。絞りに `a.projects` を使うので、受け入れたあとのイベントは扱われる。

### D4. cloneは `workspace` の `askpass` を使う

`internal/workspace` に、httpsのURLとトークンを受けてcloneする関数を足す。トークンの渡し方は、pushと同じ `GIT_ASKPASS` のスクリプトとする。URLは、GitHubは `https://github.com/<owner>/<repo>.git`、GitLabは `<base_url>/<path>.git` とする。

`origin` の確かめは、`git remote get-url origin` の結果を正規化して比べる。httpsとsshのどちらの形でも、ホストとパスが一致すればよい。

cloneが途中で失敗したときは、作りかけのディレクトリを消す。次のやり直しで「既にあって `origin` が違う」と誤って扱わないためである。

### D5. `inbox serve` はプロジェクトの一覧と追加のAPIを持つ

`Server` に、`/v1/projects`（GET）と `/v1/projects`（POST）を足す。

- GETは、この端末の担当プロジェクトごとに、`path` が `git rev-parse --git-dir` を通るかと、無い状態ラベルを返す。ラベルは受信箱のトークンでラベルの一覧を読み、6つの名前と比べる。ラベルの一覧はプロジェクトごとに60秒キャッシュする
- POSTは、spec（`inbox-api` の「プロジェクトの追加の前の確認」）の順に確かめる。そのあと受信箱のトークンでリポジトリを読み、既定のブランチを得てから、D1で書き込む
- 書き込みのあとは、`Watcher` を待たずに、その場で読み直しを行う。`Inbox` と `Instructions.Projects` を作り直し、一覧のキャッシュを捨てる

`Server` の `Inbox` と `Instructions` を、読み直しで差し替えられるよう、ミューテックスの下に置く。

`inboxapi` には、`Project`、`Projects`、`AddProjectRequest`、`AddProjectResponse` の型と、拒んだ理由を表す `Code` を足す。理由は、無効、形の誤り、重複、この端末で使っていないフォージ、読めないリポジトリ、`path` の重複、設定の誤り、設定の別の変更である。

### D6. フォージには、受信箱のトークンで読む口を2つ足す

`forge` に、`Hooks` と同じく `Forge` とは別のインターフェース `Repos` を足す。リポジトリの既定のブランチを返す口と、ラベルの名前の一覧を返す口を持つ。エージェントは使わず、受信箱のサービスだけが受信箱のトークンで呼ぶので、`Forge` には入れない。GitHubでは `GET /repos/{owner}/{repo}` と `GET /repos/{owner}/{repo}/labels` を使う。GitLabでは `GET /projects/:id` と `GET /projects/:id/labels` を使う。どちらも受信箱のトークンの今の権限で読める。

### D7. WUIは設定の画面に節を足すだけにする

`web/internal/server` に `/api/projects` のGETとPOSTを足し、`inboxapi` にそのまま渡す。POSTには、指示の保存と同じ検査（JSONと `Origin`/`Host`）を通す。

`SettingsPage.tsx` に、プロジェクトの節を指示の節の上に足す。追加の欄は、`ref` の入力、`plan_first` のチェックボックス（初期値はオン）、追加のボタンだけとする。誤りは `Code` ごとに日本語の文に直して欄の下に出す。成功したら一覧を取り直し、webhookの案内があれば欄の下に出す。

### D8. ADR-039を足す

`docs/DECISIONS.md` に、ADR-039「WUIからのプロジェクトの追加」を足す。次を記録する。

- ADR-035の「`switchboard` は設定ファイルを書き換えない」を改め、`inbox serve` がプロジェクトを1項目足す場合に限って書き換えを許すこと
- WUIから決められるのは `ref` と `plan_first` だけであること
- cloneとラベルの作成は、エージェントが自分のトークンで行うこと

## Risks / Trade-offs

- ［差し込む位置を誤り、既存の項目の値を変えてしまう］→差し込んだあとの設定を読み直し、既存のプロジェクトとほかの節が変わらないことを確かめてから書く。複数行のブロックのスカラーで終わる項目もテストする。元は `<config.yaml.bak>` に残る
- ［認証の無いWUIから、LANの誰でもプロジェクトを足せる］→足せるのは `ref` と `plan_first` だけで、実行の対象は今までどおり `trusted_authors` のコメントに限られる。公開のリポジトリの `include_untrusted` は常に偽である。`path` は `<inbox.project_root>` の下に固定する。機能は `<inbox.project_root>` を書いたときだけ有効になる
- ［手で `token_ref` の上書きを足したプロジェクトは、クレデンシャルが無いので受け入れに失敗し続ける］→受け入れの失敗をログに残し、WUIのクローンの有無で見える。`service install` のやり直しを促す文をログに出す
- ［GitHubのfine-grainedのPATの対象に新しいリポジトリが入っていない］→cloneに失敗する。WUIには、トークンの対象を確かめるよう出る（spec `web` の「クローンが無いときの案内」）
- ［`inbox serve` とエージェントが同時に同じファイルを書く］→書くのは `inbox serve` だけで、エージェントは読むだけである。手の編集との衝突は、D1のハッシュの比較で検出する
- ［10秒ごとの `stat` の負荷］→1つのファイルの `stat` だけで、中身は変わったときにしか読まない

## Migration Plan

- 既存の設定は変更なしで読める。`<inbox.project_root>` を書くまで、追加は無効のままである
- 有効にするには、`config.yaml` に `<inbox.project_root>` を足し、エージェントと `inbox serve` を1回再起動する（読み直しの部品を含む版にするため）
- 戻すときは、`<inbox.project_root>` を消せば追加は無効になる。足した項目は `config.yaml` に残るので、要らなければ手で消す
