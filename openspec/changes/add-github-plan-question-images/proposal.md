# Proposal

## Why

ADR-069で、GitLabのプロジェクトの計画の質問には、画面のスクリーンショットや説明の画像を付けられるようになった。GitHubのプロジェクトは、本文の好きな位置に画像を置く公開のAPIが無いので、Mermaidの図だけのままだった。`gh:uskanda/smarternews` でgrillingを試しても、画像は出なかった。`gh issue comment --attach` がエージェントのトークンで画像を付けられることは、実際の添付で分かった（V47）。そこで、付いた画像のURLを本文の位置へ移す方法で、GitHubでも同じ質問を出せるようにする。

## What Changes

- GitHubのプロジェクトの計画の実行にも、GitLabと同じ画像の項を渡す。渡すのは、ブラウザが見つかり、`gh` が `--attach` を持つ（2.99.0以上）ときだけとする
- 画像を持つ質問のコメントは、画像の位置に目印を置いた本文を `gh issue comment --attach` で投稿する。そのあと、末尾に付いた画像の記法を読み取り、コメントを編集して目印の位置へ移す。コメントは1件で、マーカーは最後の行に保つ
- 画像を移す編集に失敗したら、1度だけやり直す。それでも失敗したら、そのコメントを消し、画像なしで投稿し直す。投稿そのものに失敗したときも、画像なしで投稿し直す。どちらの場合も、問いに画像を付けられなかったことを1行書き、監査ログに `<plan.image_dropped>` を残し、図は残す
- 受信箱の `/v1/issue-image` は、GitHubのissueでは、`https://github.com/user-attachments/assets/<uuid>` の画像を返す。転送の先は、GitHubの画像の置き場所のホストだけを許す。Web表示は、GitLabと同じく、問いのカードの中にその画像を描く
- 非公開のリポジトリでも画像は付ける。Web表示で読めなければ、今までどおりリンクとして出す。読めるかどうかは検証項目に残す
- **BREAKING**（spec上）：GitHubのプロジェクトには画像を付けない、画像の項を入れない、という `plan-mode` の要件を改める

## Capabilities

### New Capabilities

新しい機能は無い。

### Modified Capabilities

- `plan-mode`: 画像の指示を渡す条件、質問のコメントの画像の置き方（GitHubの添付と編集、失敗のときの投稿し直し）
- `inbox-api`: 質問の画像の取得で、GitHubの添付の画像を返す
- `web`: 問いの画像で、GitHubの添付の画像を描く

## Impact

- `internal/forge`：添付の画像を目印の位置に置いて投稿する新しいインターフェース。`internal/forge/github/attach.go` が実装し、fakeにも足す
- `internal/agent`：`planimages.go` は、画像を渡す条件とGitHubでの画像の扱いを変える。`planrun.go` は、目印を置いた本文を組み立てる。質問のコメントを投稿するところも変える
- 画像の読み取り：`internal/inboxserve/issueimages.go` と `internal/forge/github` に足す。`web/ui/src/components/QuestionImage.tsx` は説明を直す
- `docs/DECISIONS.md`：ADR-069のGitHubの扱いを改める新しいADR
- `docs/ROADMAP.md`：非公開のリポジトリの画像を読めるかの検証項目
- 利用者向けの文書：`docs/GUIDE.md`、`docs/GUIDE.ja.md`、`docs/SECURITY.md`、`docs/SECURITY.ja.md`
