# Spec Delta

## ADDED Requirements

### Requirement: 画像の置き場所の取り決め
直接の実装と承認のあとの実装のsystem promptには、画像の置き場所の節をいつも入れなければならない（MUST）。
その節は、issueに画像を付けたいときは `.switchboard/screenshots/` の直下に置くこと、そこをコミットしないことを伝える。
撮れなかったときは理由をsummaryに書くことも伝える。
計画の実行と直す実行のsystem promptには、この節を入れてはならない（MUST NOT）。

#### Scenario: 直す実行
- **WHEN** CIの失敗を直す実行を始める
- **THEN** system promptに `.switchboard/screenshots/` の節は無い

### Requirement: 置き場所の準備
直接の実装と承認のあとの実装の前に、エージェントは、`.git/info/exclude` に `/.switchboard/screenshots/` の行を足さなければならない（MUST）。
その行がすでにあるとき、エージェントは、行を増やしてはならない（MUST NOT）。
エージェントは、実行を始める前に、置き場所に残った前の実行のファイルを消さなければならない（MUST）。

#### Scenario: 2回目の実行
- **WHEN** 同じissueの2回目の実装の実行を始める
- **THEN** `.git/info/exclude` の `/.switchboard/screenshots/` の行は1つだけで、置き場所は空である

### Requirement: 画像の集め方
実行がdoneで終わり、pushを済ませたとき、エージェントは、置き場所の直下の通常のファイルを画像として集めなければならない（MUST）。
対象の拡張子は `.png`、`.jpg`、`.jpeg`、`.webp` とする。
集めた画像は、runのディレクトリの `screenshots/` へ移す。書き戻しのやり直しでは、ここから読む。
1枚が5MBを超えるもの、名前の順で10枚を超えたもの、拡張子が違うものは付けず、その名前と理由を控えなければならない（MUST）。
シンボリックリンクと下のディレクトリは、無視しなければならない（MUST）。
画像が条件に合わなくても、pushを止めてはならない（MUST NOT）。

#### Scenario: 大きすぎる画像
- **WHEN** 置き場所に3MBの `a.png` と6MBの `<b.png>` がある状態で、doneで終わる
- **THEN** `a.png` だけが添付の対象になり、`<b.png>` は大きすぎるとして控えられる

#### Scenario: シンボリックリンク
- **WHEN** 置き場所に、ほかのファイルを指す `<link.png>` がある
- **THEN** `<link.png>` は集められず、指す先のファイルも読まれない

### Requirement: 画像のコミットの拒否
置き場所のファイルが、pushする差分に入っているとき、エージェントは、pushせずに検査の失敗（`waiting`、reason `verify`）で終わらなければならない（MUST）。

#### Scenario: 置き場所をgit addした
- **WHEN** 実装の実行が `.switchboard/screenshots/a.png` をコミットし、doneで終わる
- **THEN** pushされず、issueは `verify` の理由で待つ

### Requirement: 画像のコメント
doneの投稿とラベルの付け替えのあと、添付する画像か控えた画像があれば、エージェントは、画像のコメントをissueに1件投稿しなければならない（MUST）。
GitHubでは `gh issue comment --attach` で、GitLabではuploads APIで上げた画像をノートに並べて付ける。
画像のコメントには、状態 `attachment` のマーカーを付けなければならない（MUST）。
doneの投稿には、スクリーンショットが次のコメントにあることを1行書く。
次の実行では、画像のコメントを自分の以前の投稿として扱い、待ちやdoneの判定には数えない。
添付する画像と控えた画像のどちらも無いとき、エージェントは、画像のコメントを投稿してはならない（MUST NOT）。
waitingで終わった実行の画像は付けず、runのディレクトリと一緒に片付ける。

#### Scenario: 画像を付ける
- **WHEN** 実装の実行が置き場所に2枚の画像を置いて、doneで終わる
- **THEN** doneの投稿のあとに、2枚の画像と `state=attachment` のマーカーを持つコメントが1件付く

#### Scenario: 書き戻しのやり直し
- **WHEN** 画像のコメントを投稿したあとで書き戻しが途中で止まり、やり直す
- **THEN** 画像のコメントは2件目が付かない

### Requirement: 添付の失敗
添付が失敗したとき、エージェントは、画像を付けずに理由を書いたコメントを投稿しなければならない（MUST）。
エージェントは、添付の失敗でdoneの状態とラベルを変えてはならない（MUST NOT）。

#### Scenario: ghが無い
- **WHEN** GitHubのプロジェクトで、端末に `gh` が無い状態で、画像のある実行がdoneで終わる
- **THEN** issueは `agent:done` のままで、添付できなかった理由を書いた画像のコメントが1件付く

### Requirement: 添付に渡すもの
GitHubの添付では、エージェントは、`GH_TOKEN` にエージェントのトークンを、`GH_CONFIG_DIR` に空の一時ディレクトリを渡さなければならない（MUST）。
本文は標準入力から渡し、ほかの環境変数は最小限にする。
エージェントは、Claudeの実行に `gh` の許可を足してはならない（MUST NOT）。

#### Scenario: 渡す環境
- **WHEN** GitHubのプロジェクトで画像のコメントを投稿する
- **THEN** `gh` は、`GH_TOKEN` と空の `GH_CONFIG_DIR` を持つ環境で呼ばれる。引数は `issue comment <N> -R owner/repo --body-file - --attach <file>` である

### Requirement: ghの確認
`switchboard doctor` は、設定にGitHubのプロジェクトがあるとき、`gh` の有無と版を確かめなければならない（MUST）。
`gh` が無いか、v2.99.0より古いとき、`switchboard doctor` は、画像を付けられないことを警告として示さなければならない（MUST）。

#### Scenario: 古いgh
- **WHEN** GitHubのプロジェクトがあり、端末の `gh` がv2.40.0である
- **THEN** `switchboard doctor` は、画像を付けられないことを警告として示す
