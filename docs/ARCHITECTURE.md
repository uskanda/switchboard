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
          コメント・ラベル │ （フックが書き戻す）                          │ （ローカルから外向き接続）
                           │                                              ▼
            ┌──────────────┴──────────── 各端末（tmux / systemd 常駐）───────────────┐
            │  ┌───────────────────────┐        ┌────────────────────────────┐   │
            │  │ ローカルエージェント       │ ─────▶ │ Claude Code                 │   │
            │  │ 投稿者検証・issue対応表   │        │ claude -p（必要時 --resume）│   │
            │  └───────────────────────┘        └──────────────┬─────────────┘   │
            │                                                  │ Stop / Notification│
            │                                       ┌──────────▼───────────┐      │
            │                                       │ フック                 │      │
            │                                       │ コメント＋ラベル付け替え│      │
            │                                       └──────────────────────┘      │
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

### 2.3 フック（hooks）
- Claude Code の Stop / Notification フックから呼ばれる小さなコマンド
- 役割：
  - 応答待ち → issue に質問をコメントし `agent:waiting` に付け替え、隠しマーカーを付与
  - 完了 → 結果（PR URL 等）をコメントし `agent:done` に付け替え
- issue との対応は環境変数（エージェントが起動時に設定する `SWITCHBOARD_ISSUE_REF` 等）で受け取る
- フックの入力スキーマは Claude Code 側で変わりうるため、解析処理は adapters 層に置く

### 2.4 受信箱（inbox）
- 役割：全プロダクト横断の `agent:waiting` 一覧と、そこからの返答
- 取得：
  - GitHub：Search API（`is:issue is:open label:agent:waiting` に `user:` / `org:` を付加）
  - GitLab：`GET /issues?scope=all&state=opened&labels=agent:waiting`
- 結果をマージし、コメント内の隠しマーカーから端末名・待ち開始時刻を抽出して表示
- 返答は issue コメント投稿のみ（応答経路の一本化）
- 中継には依存しない。中継が止まっていても受信箱は動く

## 3. Claude Code の起動方針

- 基本は **毎回まっさらな文脈で起動**し、状態は issue（本文・コメント）とリポジトリ内のファイルから読ませる
- 同じ端末で直前のセッションが新しい場合のみ `--resume <session-id>` を使う（最適化）。使えない・失敗した場合は通常起動にフォールバック
- 起動時に渡すもの：
  - issue の本文と、信頼済み投稿者のコメント（それ以外は除外、または「参考データ」として明示的に区切る）
  - リポジトリの CLAUDE.md
  - switchboard 用の追加指示（待ちになる条件、質問の書き方、禁止操作）
- 権限：許可ツールを明示し、main への push・マージ・リリースを拒否する設定ファイルを使う
- 作業場所：issue ごとに git worktree（`switchboard/issue-<n>` ブランチ）を切る

## 4. 処理フロー

### 4.1 着手
1. 人間が issue に `agent:ready` を付ける
2. エージェントが検知 → 投稿者・担当端末を検証 → `agent:running`
3. worktree を作成し `claude -p` を実行

### 4.2 応答待ち
1. Claude が人間の判断を必要として停止
2. フックが質問をコメントし `agent:waiting` に付け替え
3. 受信箱に表示される

### 4.3 応答
1. マスターが `switchboard reply` またはフォージ UI で返答
2. webhook → 中継 → エージェント（またはポーリング）で検知
3. エージェントが本文を再取得・投稿者検証 → `agent:running` → 再実行

### 4.4 完了
1. Claude が PR を作成して終了
2. フックが PR URL をコメントし `agent:done`
3. マージは人間が行う

## 5. ディレクトリ構成（想定）

```
switchboard/
├── CLAUDE.md
├── README.md
├── config/
│   └── switchboard.example.yaml
├── docs/
├── cmd/ または src/cli/        # CLI エントリ（inbox / reply / agent / labels / relay）
├── internal/ または src/
│   ├── forge/                  # GitHub / GitLab クライアント
│   ├── inbox/                  # 横断一覧の集約
│   ├── agent/                  # 検知・検証・実行
│   ├── relay/                  # 中継サーバー
│   ├── marker/                 # 隠しマーカーの生成・解析
│   └── adapters/claudecode/    # Claude Code の CLI・hooks 依存部分
└── hooks/                      # Claude Code に登録するフックスクリプト
```

技術スタック確定後（ADR-010）に具体化する。
