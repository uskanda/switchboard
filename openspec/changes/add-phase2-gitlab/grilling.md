# grilling 記録: add-phase2-gitlab

## 決定

| 決定 | 選択 | 出典 |
|---|---|---|
| 対象のGitLab | セルフホストのGitLabだけを対象にする | 第1ラウンド Q1 |
| OpenSpecの変更の切り方 | Phase 2を1つの変更（`add-phase2-gitlab`）にする | 第1ラウンド Q2 |
| 「複数リポジトリ・worktree運用」で足すもの | GitHubとGitLabを混ぜた設定で両方をポーリングすることと、終わったworktreeの片付けを入れる。プロジェクトごとの同時実行数は入れない | 第1ラウンド Q3 |
| ADR-017（GitLabのwebhookの検証方式） | Phase 4まで保留にする。V16もPhase 4の検証項目にする | 第1ラウンド Q4 |
| GitLabのURL | `https://gl.uskan.dev`（CE 19.5.1、usknasの上で動く） | 第2ラウンド Q5 |
| GitLabのトークンの種類 | 自分の環境では、エージェント用も受信箱用も自分のPAT（`api` スコープ）を使う。ただし、公開する製品として、プロジェクト単位でもトークンを設定できるようにする | 第2ラウンド Q6 |
| 設定でのGitLabのプロジェクトの書き方 | `gl:group/sub/project` のパスで書き、起動時にAPIでプロジェクトIDを引く | 第2ラウンド Q7 |
| worktreeを片付ける契機 | issueが閉じられたとき。worktreeにコミットしていない変更があれば消さず、ログに残す | 第2ラウンド Q8 |
| 受信箱でのマージ表示 | 待ち始めた時刻の古い順に並べる。`--forge github\|gitlab` で絞り込める。届かないフォージがあれば、届いた側だけを表示し、警告を出して0以外の終了コードで終わる | 第2ラウンド Q9 |
| プロジェクト単位のトークン設定の形 | `forges.<forge>.token_ref` と `agent_identity` を既定にし、`projects[]` の `token_ref` と `agent_identity` で上書きできる。上書きできるのはエージェント側だけで、`inbox_token_ref` はフォージに1つ。GitHubのプロジェクトでも同じく上書きできる | 第3ラウンド Q11 |
| GitLabのインスタンスの数と既定のURL | `<forges.gitlab.base_url>` の既定値を `https://gitlab.com` にする。設定できるGitLabは1つだけ | 第3ラウンド Q12 |
| 完了条件の確かめ方 | 偽のGitLabを使った結合テストをCIで回す。加えて、`gl.uskan.dev` とGitHubの検証用リポジトリで、1つの受信箱に両方の待ちが出て、両方に返答してMRとPRができるまでを実物で通し、ROADMAP.mdに記録する | 第3ラウンド Q13 |
| 検証用のGitLabのユーザー名とプロジェクト | ユーザー名はglabから調べた（`uskanda`）。プロジェクトはglabで作れる `nas/switchboard-sandbox` に置く | 第3ラウンド Q10、第4ラウンド Q14 |
| 検証で使うトークン | `uskanda` のPAT（`api`）を1つ作り、`switchboard/gitlab` と `switchboard/gitlab-inbox` の両方に登録する。設定のキーは分けたままにする | 第4ラウンド Q15 |
| プロジェクト単位の上書きの確認 | 実物でも確かめる。`nas/switchboard-sandbox` のProject Access Tokenで、`token_ref` と `agent_identity` をbotに上書きし、botとして投稿するまで通す | 第4ラウンド Q16 |
| 編集者が分からないときの扱い | 信頼済みでない人が編集したものとして扱う。本文なら実行を始めず、コメントなら指示に採用しない。理由を監査ログに残す | 第5ラウンド Q17 |
| GitLabのinternalの公開範囲 | 公開として数える。`include_untrusted: true` を拒否し、`doctor` も非公開ではないと報告する | 第5ラウンド Q18 |
| ADRの追加 | GitLabの扱い（インスタンスは1つ、既定は `https://gitlab.com`、パスで書いてIDを引く）と、トークンの上書きの2つをADRに足す。ADR-017の論点を「Phase 4で決める」に書き直す | 第5ラウンド Q19 |

## 後回しにしたもの

- 複数のGitLabインスタンスを並べて設定すること（要望が出てから）
- プロジェクトごとの同時実行数の上限
- 受信箱をGraphQLの1リクエストで取る形への変更（V14。Phase 5のWeb表示の前に決める）
- GitLabのwebhookの検証方式（ADR-017）とV16（Phase 4）

## 状態

frontierは空。共有理解は2026-10-30に/okで確認済み。
