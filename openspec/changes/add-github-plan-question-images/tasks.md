# Tasks

## 1. 目印の位置に置く添付

- [x] 1.1 `forge` に `PlacedImage` と `PlacedAttacher` を足し、fakeに実装する。fakeは、目印を画像の記法に置き換えた本文を残し、投稿と編集の失敗を起こせるようにする
- [x] 1.2 GitHubの `AttachPlaced` を、失敗するテストから書いて実装する。偽の `gh` が末尾に記法を並べたコメントを、目印の位置へ移してマーカーで終わる本文に編集することを確かめる
- [x] 1.3 `AttachPlaced` が、記法の名前がファイル名と合わないときと、編集が2度失敗したときに、コメントを消して誤りを返すことをテストで確かめる。消せなかったことが誤りの値で分かることも確かめる
- [x] 1.4 `AttachComment` を、`AttachPlaced` と処理を共有する形に直し、今のテストが通ることを確かめる

## 2. 質問の組み立てと書き戻し

- [x] 2.1 `planImagesOf` を、GitHubで `PlacedAttacher` と `gh` の版（`GHAttaches`）とブラウザがそろうときに画像の項を渡すように、失敗するテストから書いて直す。`gh` の版はエージェントの起動のときに1度だけ調べる
- [x] 2.2 `attachPlanImages` を、GitHubでは検査を通った画像に目印を割り当てるように直す。問いと選択肢の位置に目印が入ることを `questionText` のテストで確かめる
- [x] 2.3 `outcome` に `Placed` と `Plain` を足し、書き戻しが `AttachPlaced` で投稿するように直す。fakeで、画像の並びとマーカーが最後の行であることを確かめる
- [x] 2.4 `AttachPlaced` が誤りを返したときの書き戻しをテストで確かめる。`Plain` を投稿し、画像ごとに `plan.image_dropped` を残し、図が残ること、である。書き戻しのやり直しで質問が2件にならないことも確かめる

## 3. 受信箱とWeb表示

- [x] 3.1 `/v1/issue-image` が、GitHubのissueでは `user-attachments` のURLだけを受けるようにする。ほかのURLには要求を出さずに404を返すことを、失敗するテストから書いて実装する
- [x] 3.2 `/v1/issue-image` の転送の扱いを、`httptest` のサーバーで確かめる。転送を1回ずつ追い、許したホストの先からはトークンを付けずに取り、ほかのホストへの転送では404を返す。キャッシュのキーが `<uuid>` であることも確かめる
- [x] 3.3 Web表示が、GitHubのissueの問いのカードで `user-attachments` の画像を受信箱のAPIから描くことを、部品のテストで確かめる。取れないときにリンクを出すことも確かめる。`QuestionImage` の説明を直す

## 4. 文書

- [x] 4.1 `docs/DECISIONS.md` に、ADR-069のGitHubの扱いを改める新しいADRを足す
- [x] 4.2 `docs/ROADMAP.md` に、非公開のリポジトリの添付の画像を受信箱のトークンで読めるかの検証項目を足す
- [x] 4.3 `docs/GUIDE.md` と `docs/GUIDE.ja.md` の質問の画像の項に、GitHubの条件（`gh` 2.99.0以上）と、非公開のリポジトリの扱いを書く
- [x] 4.4 `docs/SECURITY.md` と `docs/SECURITY.ja.md` のT13とトークンの説明に、GitHubの添付と、受信箱のトークンを `github.com` にだけ送ることを書く
- [x] 4.5 `make verify` を通す

## 5. 実物での確認

- [ ] 5.1 `gh:` のsandboxのプロジェクトで、画面の変更を問う計画の実行を動かす。問いと選択肢の画像が、GitHubのコメントの位置に出て、Web表示にも描かれることを確かめ、ROADMAP.mdに書く
