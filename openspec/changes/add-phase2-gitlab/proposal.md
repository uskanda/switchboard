# Proposal

## Why

Phase 1で、GitHubの1リポジトリについて、着手から待ち、返答、PRの作成までが回るようになった（ROADMAP.mdのV20）。しかし、作業の多くはセルフホストのGitLabにもあり、今の `switchboard` はGitLabのプロジェクトを設定に書くと起動しない。ROADMAP.mdのPhase 2の完了条件は、両フォージの待ちが1つの一覧に出て、どちらにも返答できることである。

## What Changes

- GitLabのクライアントを足し、`gl:group/sub/project` の形のプロジェクトを受け付ける。対象のGitLabは `<forges.gitlab>` の `base_url` で指定する。既定は `https://gitlab.com` とし、設定できるGitLabは1つとする
- **BREAKING** 設定で `gl:` のプロジェクトを拒否する要件を外す。代わりに、`<forges.gitlab>` が無いまま `gl:` のプロジェクトを書いた設定を誤りとして報告する
- エージェントは、プロジェクトごとにフォージを選んで動く。GitHubとGitLabのプロジェクトを混ぜた設定で、両方をポーリングする
- エージェントのトークンと `agent_identity` は、プロジェクトの設定で上書きできる。受信箱のトークンは、フォージに1つのままとする。上書きはGitHubのプロジェクトにも使える
- 状態ラベル、信頼判定、書き戻しを、GitLabでも同じ規則で行う。完了したときは、draftのMRを作る
- GitLabで本文やコメントの最後の編集者が分からないときは、信頼済みでない人の編集として扱う。GitLabの `internal` のプロジェクトは、公開のプロジェクトとして数える
- 受信箱は、両フォージの待ちを1つの一覧にまとめる。`--forge github|gitlab` で絞り込める。片方のフォージに届かないときは、届いた側だけを表示し、警告を出して0以外の終了コードで終わる
- issueが閉じられたら、エージェントは、そのissueのworktreeとローカルのブランチを消す。コミットしていない変更が残っていれば消さず、ログに残す
- `doctor` と `labels sync` が、GitLabのプロジェクトも扱う
- DECISIONS.mdに、GitLabの扱いと、トークンの上書きのADRを足す。ADR-017（GitLabのwebhookの検証方式）は、Phase 4で決めると書き直す

## Capabilities

### New Capabilities

- `workspace-cleanup`: 閉じられたissueのworktreeとローカルのブランチの片付け

### Modified Capabilities

- `config`: GitLabの設定（`base_url`、トークン、`agent_identity`）、`gl:` のプロジェクトの受け付け、プロジェクトごとのトークンと `agent_identity` の上書き
- `trust`: フォージごとのユーザーIDの解決、プロジェクトごとの `agent_identity`、編集者が分からないときの扱い、`internal` の数え方
- `run-outcome`: GitLabではdraftのMRを作る。issueを閉じる参照と保護パスを、フォージに合わせる
- `agent-run`: GitLabでは、待ちのissueのノートの数も毎回見て、返答を検知する
- `inbox`: 両フォージの待ちの一覧、`--forge` での絞り込み、届かないフォージの報告、`gl:` の形の `<ref>`

## Impact

- `internal/forge` に、GitLabのクライアント（`internal/forge/gitlab`）と、偽のGitLabを足す。`forge.IssueRef` にフォージの種別を持たせる
- フォージを複数扱えるように、`internal/config`、`internal/agent`、`internal/trust`、`internal/inbox` を変える。`internal/doctor`、`internal/labels`、`cmd/switchboard` も同じく変える
- 外部依存は増やさない。GitLabのAPIは、標準ライブラリのHTTPクライアントで呼ぶ
- `config/switchboard.example.yaml`、README.md、DECISIONS.md、ROADMAP.mdを更新する
- 実物での確認に、`gl.uskan.dev` の検証用プロジェクト `nas/switchboard-sandbox` と、GitLabのPATとProject Access Tokenを使う
