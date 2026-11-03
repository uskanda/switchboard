# Design

## Context

WUIは、参照を`IssueRef`で`o/r#12`とフォージのバッジの形に出している。switchboard-webのCSPは`default-src 'self'`で、フォージの画像のURLをそのまま`<img>`に入れると読めない。フォージのトークンを持つのはinboxサービスだけで、switchboard-webはトークンを持たない（ADR-030の線引き）。

## Goals / Non-Goals

**Goals:**
- 参照の前に、プロジェクトのアイコンを1か所の変更で揃えて出す
- CSPを緩めず、トークンをフォージ以外のホストへ送らない
- フォージへの問い合わせを、保存と失敗の記憶で抑える

**Non-Goals:**
- 新規issueの選択肢（ネイティブの`<select>`）にアイコンを出すこと
- CLIの出力にアイコンを出すこと
- 中継に流すものを変えること

## Decisions

- フォージの層に`forge.Avatars`の`ProjectAvatar(ctx, owner, repo) ([]byte, error)`を足す。無いときは`ErrNotFound`を返す。画像の検めは`forge.ImageType`にまとめ、PNG、JPEG、GIF、WebPを先頭のバイトで見分ける。256KiB（`forge.MaxImageBytes`）を超える画像とSVGは`forge.ErrBadImage`にする
- 画像を取る処理は`forge.GetImage`にまとめる。認証のヘッダーは、要求のホストが認めたホストのときだけ付ける。リダイレクトで別のホストへ移るときも外す。Goの標準のクライアントは`Authorization`を外すが、GitLabの`PRIVATE-TOKEN`は外さないためである
- GitHubは`GET /users/{owner}`の`avatar_url`に`s=64`を付けて取る。トークンを付けるのはAPIのホストだけにする
- GitLabは`GET /projects/:id/avatar`をトークン付きで取る。404なら`GET /projects/:id`の`namespace.avatar_url`を取る。相対のURLはインスタンスに対して解く。トークンはインスタンスと同じホストにだけ付ける
- inboxサービスは`inboxserve.Icons`で`/v1/project-icon`と`/v1/project-icons`を出す。設定は`Server.Reload`で差し替える
  - `icon`がURLなら、トークンを付けずに取る。ファイルなら毎回読む。どちらも同じ検めを通す
  - 保存は、状態の置き場の下の`project-icons/`に、参照と`icon`の値のSHA-256をファイル名にして置く。`icon`を書き換えると別のファイルになり、古い画像は使われない。メモリにも持ち、ディスクのファイルの更新時刻で24時間を測る
  - 失敗はメモリにだけ1時間覚える。要求が途中で取り消されたときは覚えない
  - フォージへの問い合わせは要求から切り離し、15秒で打ち切る
- `inboxapi.Client`に`ProjectIcon`（中身と`Content-Type`）と`ProjectIcons`（絵文字の対応）を足す
- switchboard-webは2つのルートをそのまま渡す。画像の答えでは`Content-Type`と`Cache-Control`をinboxサービスから写す。`X-Content-Type-Options: nosniff`とCSPは、ほかの答えと同じく先に付ける
- 設定の`icon`は、読むときに形を検める。書記素を数える依存は足さず、絵文字の続き（異体字セレクタ、肌の色、ZWJ、国旗の2文字、キーキャップ、タグ文字）だけを1つの書記素として扱う。`~/`はプロジェクトのパスと同じくホームに広げる
- WUIは`ProjectIcon`の部品を`IssueRef`の中で参照の前に置く。部品は`aria-hidden`にし、画像は`alt=""`にする。参照の読み上げとテストが探す名前は変わらない
  - 頭文字の丸を下に敷き、画像が読めたら丸を隠す。読めなかったら画像を外す。読み込み中も丸が見える
  - 絵文字の一覧は`lib/projectIcons.ts`で最初の1回だけ読み、`useSyncExternalStore`で部品に配る。読めなかったら空の一覧にする
- DESIGN.mdに`project-initial`（地`muted`、文字`on-primary`、角の丸みは`full`）を足す。暗いテーマでは`muted`と`on-primary`が入れ替わるので、組の明るさの差は保たれる

## Risks / Trade-offs

- GitHubでは同じオーナーのリポジトリが同じアイコンになる → `icon`で上書きできる
- 書記素の判定は近似である → 絵文字の主な並びは通る。漏れた並びは2文字以上と判定され、読み込みが止まって気付ける
- `icon`のURLは端末が取りに行く → `https://`だけを認め、トークンは付けない。中身は画像の検めを通ったものだけを返す
- 保存した画像は消さない → 1プロジェクトあたり256KiBまでで、`icon`を変えたときだけ増える
