# Design

## Context

GitLabの質問の画像（ADR-069）は、`attachPlanImages` が画像を検査してから `forge.Uploader` でアップロードする。返った記法は `imageMD` と `optionMD` に持つ。`questionText` がそれを本文に置き、書き戻しが `outcome` の `Body` として投稿する。画像の指示を渡すかは `planImagesOf` が決め、今はGitLabで `Uploader` を持つクライアントのときだけ渡す。

GitHubの `AttachComment`（ADR-051）は、`gh issue comment --attach` で投稿する。そのあと、APIでコメントを読み、マーカーが最後でなければ編集して戻す。`gh` が付ける記法は `![<拡張子を除いたファイル名>](https://github.com/user-attachments/assets/<uuid>)` である。記法は `--attach` の順に本文の末尾に並ぶ。`gh` の版は `GHVersion` と `GHAttaches` で分かり、今は `doctor` だけが使う。

書き戻しは、`outcome` をJSONで実行のディレクトリに残してから投稿する。投稿に失敗した書き戻しは、あとでやり直される。受信箱の `/v1/issue-image` は、GitLabのアップロードだけを `forge.UploadReader` で読み、ディスクにキャッシュする。動機はproposal.mdの「Why」を参照する。

## Goals / Non-Goals

**Goals:**

- GitHubの質問のコメントは1件で、画像は問いと選択肢の位置にあり、最後の行はマーカーである
- 添付、編集、読み取りのどこで失敗しても、質問は画像なしで投稿され、待ちの判定は崩れない
- 受信箱のトークンは `github.com` にだけ送る

**Non-Goals:**

- GitHub Enterprise Serverの添付（`gh` の `--attach` の対応と、添付のURLの形を確かめていない）
- 実装のdoneのスクリーンショットのコメント（ADR-051）の置き方の変更
- 非公開のリポジトリの画像を、Web表示で必ず読めるようにすること

## Decisions

### 目印を置いて添付するインターフェース

`forge` に `PlacedAttacher` を足す。

```go
type PlacedImage struct{ Mark, Path string }
type PlacedAttacher interface {
	AttachPlaced(ctx context.Context, ref IssueRef, body string, images []PlacedImage) (*Comment, error)
}
```

`body` は、画像を置く位置に、`Mark`（`<!-- switchboard-image:3 -->` の形のHTMLのコメント）を1行ずつ持つ。GitHubのクライアントは、`gh issue comment --attach` で `images` の順にファイルを付けて投稿し、APIでコメントを読む。末尾に並んだ画像の記法を、順に `images` と対応させる。各 `Mark` の行をその記法に置き換え、末尾の記法を消し、マーカーを最後にした本文で編集する。対応は順で取り、記法の名前が拡張子を除いたファイル名と合うかも確かめる。合わなければ編集の失敗として扱う。

編集に失敗したら、1度だけやり直す。それでも失敗したら、コメントを消して、誤りを返す。投稿そのものの失敗も誤りを返す。消したかどうかは誤りの値で分かるようにする。消せなかったときは、その旨を監査ログに残す。

`Uploader` で記法を先に得るGitLabの流れは変えない。`AttachComment` は、`AttachPlaced` の画像を目印なしで末尾に置く場合として、内部の処理を共有する。

代わりの案は、画像だけのコメントを先に付けてURLを借りる方法と、専用のブランチにコミットする方法だった。前者はコメントが2件になり、後者はリポジトリが大きくなり、pushの権限も要る（grillingのQ1）。

### 質問の組み立てと書き戻し

`attachPlanImages` は、検査までをGitLabと共有する。GitHubでは、検査を通った画像に番号の目印を割り当て、`imageMD` と `optionMD` に目印を入れる。`questionText` は変えずに、目印が画像の位置に入る。

`outcome` に2つの欄を足す。`Placed`（目印と、実行のディレクトリの中の画像のパス）と、`Plain`（画像なしで投稿し直すときの本文）である。`Plain` は、画像をすべて捨てた扱いで `questionText` を組み直したものである。画像を付けられなかった旨の1行と、捨てずに残した図を持つ。書き戻しは、`Placed` があり、フォージのクライアントが `PlacedAttacher` のときに `AttachPlaced` を呼ぶ。誤りなら、`Placed` の画像ごとに `plan.image_dropped` を残し、`Plain` を `PostComment` で投稿する。`Plain` が要るのは、図を捨てたかどうかが画像を付けられたかで変わるためである。

書き戻しのやり直しで、前の試みのコメントが残っていると、質問が2件になる。`AttachPlaced` が誤りを返すときにコメントを消しておくのはこのためである。消せなかったときは、マーカーの付いたコメントが残るので、今の書き戻しと同じく、投稿済みとして扱う。

### 画像の指示を渡す条件

GitHubでは、`planImagesOf` は次の3つを満たすときに画像の項を渡す。クライアントが `PlacedAttacher` であること、`gh` の版が `GHAttaches` を満たすこと、ブラウザが見つかることである。`gh` の版は、エージェントの起動のときに1度だけ調べて持つ。画像の項の文はGitLabと同じにする（grillingのQ6）。

### 受信箱の読み取り

`IssueImages` に、GitHubの読み取り（`forge.UploadReader` とは別の小さなインターフェース）を足す。URLは `https://github.com/user-attachments/assets/<uuid>` の形だけを受ける。`<uuid>` は16進とハイフンに限る。`github.com` への要求には受信箱のトークンを付ける。転送は手で1回ずつ追う。転送の先は、inbox-apiの仕様に挙げたGitHubの画像の置き場所のホストだけを許す。その先からは、トークンを付けずに取る。形式の検査とキャッシュはGitLabと同じにし、キャッシュのキーは `<uuid>` とする。

公開のリポジトリでは、トークンが無くても読める。非公開のリポジトリで読めるかは、ROADMAPの検証項目に残す。読めなければ404を返し、Web表示はリンクとして出す。

### Web表示

`QuestionImage` は、受信箱のAPIに任せて画像を取る。Web側の変更は、GitHubの添付のURLも描く対象にすることと、説明の文の更新に留める。

### ADR

ADR-069のGitHubの扱いを改める新しいADRを足す。GitHubでは `gh --attach` と編集で画像を置くこと、失敗のときの投稿し直し、受信箱のトークンを送る先を書く。V47は確認済みにした。

## Risks / Trade-offs

- `gh` の `--attach` は非公開の仕組みに頼る → 版を確かめ、失敗したら画像なしで投稿するので、質問そのものは必ず出る。`gh` の出力の形が変わったら、名前の照合で編集の失敗として扱う
- 編集が終わるまでの数秒、画像が末尾に並ぶ → 受け入れる。中継は `edited` を転送するが、自分のコメントの編集で困ることは今も起きていない
- 編集と削除の両方に失敗すると、画像を末尾に並べたままの質問が残る。マーカーが最後の行でないので、待ちの判定が崩れうる → 起きるのは、GitHubのAPIが続けて失敗したときだけである。監査ログに `<plan.images_left>` を残し、同じ質問を投稿し直して2件にすることはしない
- 非公開のリポジトリの画像を、Web表示で読めない → リンクとして出す。GitHubの画面では、閲覧者のログインで見える
