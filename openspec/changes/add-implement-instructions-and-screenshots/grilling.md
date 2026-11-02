# grilling 記録: add-implement-instructions-and-screenshots

issue `gh:uskanda/switchboard`#94の計画の段階で、switchboardの計画の実行がissueのコメントで質問し、issueの持ち主が答えた。この記録はそのやりとりを写したものである。issueの要望は、UIを変えた実装でスクリーンショットを撮り、issueに付けることである。WebならPlaywright、アプリならエミュレーターや実機で撮り、撮れる環境が無いときはその旨を書く。

## 決定

| 決定 | 選択 | 出典 |
|---|---|---|
| オプションの置き場所と形 | 当初はプロジェクトの設定に `screenshots: true` を置くとした（A）。のちに Q10・Q11 で、撮影専用の項目は作らないことに置き換わった | 第1ラウンド Q1 |
| 撮影のコマンドの許可 | 当初はプロジェクトの設定に `screenshot_commands` を置くとした（A）。のちに Q9 で、既存の `allow_tools_extra` に置き換わった | 第1ラウンド Q2 |
| 画像をissueに付ける手段 | 当初は画像をissueのブランチにコミットし、SHAで固定したURLを埋め込むとした（A）。のちに Q7 で置き換わった | 第1ラウンド Q3 |
| 画像の検査と上限 | 置き場所の直下の `.png`・`.jpg`・`.webp` だけを扱う。1枚5MB、1回の実行で10枚までとし、超えたものは付けずにその旨を書く。pushは止めない | 第1ラウンド Q4 |
| どの実行で撮るか | 直接の実装（`agent:ready`・`agent:go`）と、承認のあとの実装だけで撮る。UIを変えたときだけ撮り、撮れないときは理由をsummaryに書く。直す実行（CI・push・衝突）と計画の実行では頼まない | 第1ラウンド Q5 |
| ADR | 新しいADRを足し、SPEC.mdと設定の例も更新する。mainでADR-045・046が使われたので、番号はADR-047、検証項目はV43から振る | 第1ラウンド Q6 |
| `gh --attach` の調査と添付の手段（Q3の見直し） | switchboard自身が、doneを書き戻したあとに画像だけのコメントを1件付ける。GitHubは `gh issue comment --attach`（`GH_TOKEN` にエージェントのトークン、空の `GH_CONFIG_DIR`）、GitLabは uploads API。画像はコミットせず、置き場所は `.git/info/exclude` に入れる。添付に失敗したら理由をコメントに書き、doneは保つ | 第2ラウンド Q7 |
| 画像のコメントの置き方 | doneの投稿のあとに、画像だけのコメントをissueに1件足す。このコメントにもマーカーを付け、次の実行では自分の以前の投稿として扱う。doneの投稿には「スクリーンショットは次のコメント」と1行書く。PR/MRには付けない | 第2ラウンド Q8 |
| `screenshot_commands` を `allow_tools_extra` で置き換えるか | 置き換える。撮影のコマンドは `projects[].allow_tools_extra` に書き、例を `docs/examples/screenshots.md` に置く | 第3ラウンド Q9 |
| `screenshots: true` を残すか（Q10） | 持ち主から「実装時のみ」などを指定できる、より一般的なプロンプトの仕組みと比べ直すよう求められ、Q11で決め直した | 第3ラウンド Q10 |
| 撮影の指示を渡す仕組み（Q10の見直し） | 指示に3つめの種類 `implement.md`（全体用とプロジェクト用）を足す。入るのは直接の実装と承認のあとの実装だけ。撮影の方針は利用者がこのファイルに書く。置き場所 `.switchboard/screenshots/` の取り決め、その exclude、添付は switchboard の組み込みとし、実装の実行のsystem promptに短い節をいつも入れる。`screenshots` の設定の項目は作らない | 第4ラウンド Q11 |

## 調べた事実（第2ラウンド）

- GitHub CLIはv2.99.0から `--attach` に対応する。対象は `gh issue comment` などで、画像は `user-attachments` のURLとして本文に入る。1回のコマンドで50件まで。リポジトリへのpushの権限が要る。GHESには対応しない
- 中で使うアップロードの仕組みは、公開されたREST APIではない
- エージェントのトークン（fine-grained PAT）で通るかは確かめていない。検証項目V43として残す

## 後回しにしたもの

- 直す実行だけに入る指示の種類（要望が無い）
- PR/MRへの画像の添付

## 状態

frontierは空。計画は2026-11-02にissueのコメント6584241523で示し、コメント6584258507で承認された。
