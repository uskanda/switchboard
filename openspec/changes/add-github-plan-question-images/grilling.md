# grilling 記録: add-github-plan-question-images

GitHubのプロジェクトの計画の質問にも、ADR-069と同じく、画面のスクリーンショットや説明の画像を付ける計画である。きっかけは、`gh:uskanda/smarternews` でgrillingを何度か試したところ、Mermaidの図しか出なかったことである。これはADR-069の決定どおりの動きで、GitHubには本文の好きな位置に画像を置く公開のAPIが無いため、画像の指示を渡していなかった。

面談の前に確かめた事実は次のとおりである。

- ADR-051の `gh issue comment --attach` は、エージェントのトークンと空の `GH_CONFIG_DIR` で、実際に画像を付けている。例は `gh:uskanda/smarternews` #118、#146、`gh:uskanda/switchboard` #104である。V47は未確認のままだったが、通っている
- 付いた画像は、`![名前](https://github.com/user-attachments/assets/<uuid>)` の形で本文の末尾に並ぶ。今の `AttachComment` は、投稿のあとで本文を編集し、マーカーを末尾に戻している
- smarternewsとswitchboardは公開リポジトリである。`user-attachments` のURLは、認証なしでS3の署名付きURLへ転送され、PNGを読めた。転送の先は `github-production-user-asset-*.s3.amazonaws.com` である。非公開のリポジトリで読めるかは確かめていない
- 中継は `issue_comment` の `edited` を転送する。今の `AttachComment` の編集で困ったことは起きていない
- smarternewsは `usknas` のホストで動く。画像を付けるには、そのホストにも `agent.browser` の設定と新しいバイナリが要る

## 決定

| 決定 | 選択 | 出典 |
|---|---|---|
| 画像を問いの位置に置く方法 | A：質問のコメントを `gh --attach` で投稿し、末尾に付いた画像のURLを読み取って、コメントを編集して問いと選択肢の位置へ移す。コメントは1件にする | 第1ラウンド Q1 |
| Web表示で画像を読む方法 | A：受信箱の `/v1/issue-image` を通す。読むのは `https://github.com/user-attachments/assets/<uuid>` だけで、転送の先はGitHubの画像の置き場所だけを許し、形式を検査してキャッシュする。CSPは変えない | 第1ラウンド Q2 |
| 非公開のリポジトリ | A：公開か非公開かを問わず付ける。Web表示で読めなければリンクとして出す。非公開で読めるかは検証項目に残す | 第1ラウンド Q3 |
| 付けられないときの扱い | AとB：`gh --attach` が使えないと事前に分かるときは画像の指示を渡さない。投稿のときに失敗したら、GitLabと同じく、画像なしで投稿し、問いに1行書き、`plan.image_dropped` を残し、図は残す | 第1ラウンド Q4 |
| 画像を移す編集に失敗したとき | A：1度だけやり直し、それでも失敗したら、そのコメントを消して画像なしで投稿し直す。問いに1行書き、図は残す | 第2ラウンド Q5 |
| GitHubでの図と画像の選び方 | A：GitLabと同じ。1つの問いにMermaidか画像のどちらか1つ、画面の変化は画像、流れや状態はMermaidを目安に実行が選ぶ。画像の項もGitLabと同じものを渡す | 第2ラウンド Q6 |

## 後回しにしたもの

- 非公開のリポジトリの画像を、受信箱のトークンで読めるかの確認（検証項目として残す）
- `usknas` のホストへの `agent.browser` の設定とデプロイ（設定の作業で、仕様は変えない）

## 状態

frontierは空。共有理解は2026-11-12に確認済み。
