# アーキテクチャ

## 1. 全体像

```
            ┌──────────── マスター（スマホ / PC）────────────┐
            │  switchboard inbox / reply   または フォージUI │
            └───────────────┬────────────────────────────────┘
                            │ issue コメント投稿
                            ▼
                 ┌─────────────────────┐   webhook（署名付き）   ┌──────────────────┐
                 │  GitHub / GitLab     │ ─────────────────────▶ │  中継（relay）    │
                 │  issue・ラベル・コメント│                       │  署名検証・重複排除 │
                 └─────────▲───────────┘                        │  ID通知のみ        │
                           │                                     └────────┬─────────┘
          本文はローカルが │ 自分のトークンで再取得                         │ SSE / WebSocket
          コメント・ラベル │ （エージェントが書き戻す）                    │ （ローカルから外向き接続）
                           │                                              ▼
            ┌──────────────┴──────────── 各端末（systemd 常駐）──────────────────────┐
            │  ┌───────────────────────┐  起動   ┌────────────────────────────┐   │
            │  │ ローカルエージェント       │ ─────▶ │ Claude Code（sandbox）       │   │
            │  │ 投稿者検証・issue対応表   │ ◀───── │ claude -p（必要時 --resume）│   │
            │  │ push・PR・コメント・ラベル │  終了   └────────────────────────────┘   │
            │  └───────────────────────┘  （宣言の JSON と worktree を検査）         │
            └──────────────────────────────────────────────────────────────────┘
```

## 2. コンポーネント

### 2.1 中継（relay）— Phase 4 以降
- 役割：フォージからの webhook を受け、検証済みイベントの **ID だけ** を購読中のエージェントへ流す
- 入力：GitHub webhook（`issues`, `issue_comment`, `label`）、GitLab webhook（Issue Hook, Note Hook）
- 処理：
  1. 署名検証（GitHub: `X-Hub-Signature-256` の HMAC-SHA256、GitLab: `X-Gitlab-Token` の定数時間比較）
  2. 配信IDで重複排除（GitHub: `X-GitHub-Delivery`、GitLab: `X-Gitlab-Event-UUID`）
  3. ペイロードから `forge / project / issue番号 / event種別 / delivery_id` だけを抽出し、残りは破棄
  4. 購読中のエージェントへ配信（SSE を第一候補）
- 保持しないもの：本文・コメント・タイトル・ユーザー名・フォージのトークン
- 配置：Tailscale 内の常時稼働機（自宅サーバー等）。webhook 受信口のみ公開が必要なため、Tailscale Funnel や Cloudflare Tunnel の利用を検討（ADR-007）

### 2.2 ローカルエージェント（agent）
- 役割：担当 issue の更新を検知し、検証のうえ Claude Code を実行する
- 更新検知：
  - ポーリングモード（既定）：対象リポジトリの issue を一定間隔で取得
  - 中継モード：中継の SSE を購読し、届いた ID について取得
  - 中継モードでも低頻度ポーリングを併用し、取りこぼしを照合（reconcile）する
- 処理：
  1. フォージ API から issue とコメントを **自分のトークンで** 取得
  2. 新着コメントの投稿者が `trusted_authors` に含まれるか検証。含まれなければ無視（ログのみ）
  3. issue → リポジトリ / 作業ディレクトリ / 端末 の対応を設定から解決。自端末の担当でなければ無視
  4. ラベルを `agent:running` に付け替え
  5. プロンプトを組み立て `claude -p` を実行（3 章）
- 排他：同一 issue の同時実行を防ぐロック（端末内はファイルロック、端末間は担当端末の固定で回避）

### 2.3 書き戻し（エージェント）
- Claude Code のフックは使わない（ADR-060）。`claude -p` は人の入力を待って止まらず（ADR-019）、プロセスの終了が実行の終わりになる
- `claude -p` が終わると、エージェントが結果を書き戻す：
  - 待ち → issue に質問をコメントし `agent:waiting` に付け替え、隠しマーカーを付与
  - 完了 → push 前の検査を通してから push し、draft の PR/MR を作り、URL をコメントして `agent:done` に付け替え
- 実行の終わり方は、`--json-schema` の構造化出力（宣言）で受け取る。CLI の出力形式への依存は `internal/claudecode` に置く
- フォージへの書き込みはエージェントだけが行い、Claude の環境にはフォージのトークンも push の資格情報も渡さない（ADR-011）

### 2.4 受信箱（inbox）
- 役割：全プロダクト横断の `agent:waiting` 一覧と、そこからの返答
- 取得：
  - GitHub：Search API（`is:issue is:open label:agent:waiting` に `user:` / `org:` を付加）
  - GitLab：`GET /issues?scope=all&state=opened&labels=agent:waiting`
- 結果をマージし、コメント内の隠しマーカーから端末名・待ち開始時刻を抽出して表示
- 返答は issue コメント投稿のみ（応答経路の一本化）
- 中継には依存しない。中継が止まっていても受信箱は動く
- Androidのアプリ（`app/`）は、受信箱のWeb表示をWebViewで開く。`inbox.notify.fcm` があるとき、受信箱のサービスは画面が無くても一覧を取り直し、新しく待ちか完了になったissueをFCMでアプリに知らせる。通知に載せるのは参照、理由、タイトルだけである。計画の承認待ちの通知の「承認」は、アプリを開かずに承認の語を投稿する（ADR-047）

## 3. Claude Code の起動方針

- 基本は **毎回まっさらな文脈で起動**し、状態は issue（本文・コメント）とリポジトリ内のファイルから読ませる
- 同じ端末で直前のセッションが新しい場合のみ `--resume <session-id>` を使う（最適化）。使えない・失敗した場合は通常起動にフォールバック
- 起動時に渡すもの：
  - issue の本文と、信頼済み投稿者のコメント（それ以外は除外、または「参考データ」として明示的に区切る）
  - リポジトリの CLAUDE.md
  - switchboard 用の追加指示（待ちになる条件、質問の書き方、禁止操作）
- 権限：許可ツールを明示した設定ファイルを実行ごとに作る。main への push・マージ・リリースは、権限の拒否の規則ではすり抜けられる（V5）ので、それに頼らない。push はエージェントだけが issue のブランチに行い、Claude の環境には資格情報を渡さない。防壁は sandbox（ADR-057）と、push 前の検査（保護パス、秘密、ブランチ）である（ADR-060）
- 作業場所：issue ごとに git worktree（`switchboard/issue-<n>` ブランチ）を切る

## 4. 処理フロー

### 4.1 着手
1. 人間が issue に `agent:ready` を付ける
2. エージェントが検知 → 投稿者・担当端末を検証 → `agent:running`
3. worktree を作成し `claude -p` を実行

### 4.2 応答待ち
1. Claude が人間の判断を必要とし、質問を宣言して終了
2. エージェントが質問をコメントし `agent:waiting` に付け替え
3. 受信箱に表示される

### 4.3 応答
1. マスターが `switchboard reply` またはフォージ UI で返答
2. webhook → 中継 → エージェント（またはポーリング）で検知
3. エージェントが本文を再取得・投稿者検証 → `agent:running` → 再実行

### 4.4 完了
1. Claude が issue のブランチにコミットし、完了を宣言して終了
2. エージェントが push 前の検査を通して push し、draft の PR/MR を作り、URL をコメントして `agent:done`
3. マージは人間が行う。受信箱のWeb表示の「マージ」のボタンからも行え、そのときは受信箱のサービスが受信箱のトークン（人の身元）でdraftを解除してマージする。issueのcloseはPR/MRの `Closes #N` に任せる（ADR-043）
4. 押したマージがPR/MRとbaseの衝突で断られたときは、受信箱のサービスが依頼を状態ディレクトリの `inbox/automerge.json` に覚える。エージェントの直す実行が新しいdone（マーカーに `fix=<ci|push|conflict>`）を投稿し、PR/MRのheadがそのコミットなら、受信箱のサービスがそのコミットを指定してマージする。依頼は `inbox.refresh_interval` ごとに、画面がつながっていなくても調べる。押したあとに人の指示の実行が入ったとき、issueやPR/MRが閉じたとき、`waiting` に移ったとき、人がpushしたときは打ち切る。WUIの詳細のペインから取り消せる（ADR-048）

### 4.5 CIとpushの失敗（ADR-041）
1. 完了のコメントのマーカーに、pushしたコミット（`sha`）と変更要求の番号（`cr`）を書く
2. エージェントは `poll_interval` ごとに、そのコミットのCI（GitHub Actionsのjobとcommit status、GitLabのpipelineのjob）を読む。見張りの対象は端末に記録せず、照合のたびに `agent:done` のissueのマーカーから作り直す
3. CIが全部終わって失敗があれば、失敗したjobのログの末尾を非信頼のデータとして渡し、直す実行を始める（`agent:done` → `agent:running`）。上限を超えたら `waiting(reason=ci)`
4. pushがnon-fast-forwardで拒否されたら、リモートのブランチをfetchし、同じ実行の枠のまま直す実行に引き継ぐ。権限と規則による拒否は `waiting(reason=push)`

## 5. ディレクトリ構成

```
switchboard/
├── AGENTS.md
├── README.md
├── config/
│   └── switchboard.example.yaml
├── docs/
├── cmd/switchboard/            # CLI エントリ（agent / inbox / reply / labels / relay / service / doctor）
├── inboxapi/                   # 受信箱のサービスと Web 表示の間の API（Unix ソケット）
├── internal/
│   ├── forge/                  # フォージの抽象と GitHub / GitLab / 偽のクライアント
│   ├── agent/                  # 検知・検証・実行・書き戻し
│   ├── claudecode/             # Claude Code の CLI・stream-json・設定の生成
│   ├── trust/                  # 投稿者と編集者の検証
│   ├── workspace/              # worktree、実行前と push 前の検査、push
│   ├── inbox/、inboxserve/     # 横断一覧の集約と受信箱のサービス
│   ├── relay/、relayclient/    # 中継サーバーとその購読
│   ├── marker/                 # 隠しマーカーの生成・解析
│   └── …                       # config、state、secret、service、doctor など
├── web/                        # 受信箱の Web 表示（switchboard-web、別モジュール）
└── app/                        # Android のアプリ（Web 表示を WebView で開く）
```
