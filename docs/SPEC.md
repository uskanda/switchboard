# 想定仕様

## 1. 目的

複数端末・複数プロダクト（リポジトリ）にまたがる Claude Code の作業について、

- 今自分が応答しなければいけないものを一か所で一覧でき
- そこからすぐに応答を返せる

ようにする。作業の起点と応答経路は GitHub / GitLab の issue とする。

## 2. 前提と制約

- Claude Code は個人の Pro/Max サブスクリプションで利用する
- 2026-10 時点で、サブスクリプションで動く全セッションを横断取得する公式 API、および任意のリモートセッションへ CLI/API から応答を投げる公式手段は存在しない（詳細は PRIOR_ART.md）
- 使える公式部品：Claude Code の hooks（Stop / Notification 等）、`claude -p`（ヘッドレス実行）、`--resume`、フォージの webhook / API
- 利用者は 1 名（自分）。チーム利用は対象外

## 3. 機能要件

### F1. issue による作業起動・継続
- 対象ラベル（例：`agent:ready`）が付いた issue、または待ち状態の issue への信頼済みコメントを契機に、対応する端末で Claude Code を実行する
- issue とリポジトリ・作業ディレクトリ・実行端末の対応は設定ファイルで決める
- 実行に足す利用者の指示は3種類ある。計画の実行への `plan.md`、ファイルを書くすべての実行への `run.md`、直接の実装と承認のあとの実装だけへの `implement.md` である（ADR-038・045・047）
- 直接の実装と承認のあとの実装が `.switchboard/screenshots/` に置いた画像は、doneの投稿のあとに画像だけのコメントとしてissueに付ける。画像はコミットしない。付けられなかったときは理由を書き、doneは保つ（ADR-047）

### F2. 待ち状態の可視化
- Claude Code が人間の応答を必要として止まったら、issue にコメントを投稿し、ラベルを `agent:waiting` に付け替える
- コメントには隠しマーカーで端末名・セッションID等のメタデータを埋め込む

### F3. 横断受信箱（inbox）
- GitHub と GitLab の両方について、`agent:waiting` が付いた open issue をラベル検索で取得し、マージして表示する
- 表示項目：フォージ / プロダクト / issue 番号とタイトル / 実行端末 / 待ち始めからの経過時間 / 最後の質問の冒頭
- 並び順：待ち時間の長い順（既定）
- `agent:done` でopenのissueは、closeされるまで「完了（close待ち）」の節に出す。各行には端末、完了からの経過時間、PR/MRのリンクを出し、完了した時刻の古い順に並べる。行番号は待ちの行に続けて振る（ADR-042）

### F4. 受信箱からの応答
- 受信箱の項目を指定して返答すると、その issue へのコメントとして投稿する
- 返答を受け付けるのは、openで `agent:waiting` か `agent:done` のissueとする。doneのissueへの返答で、同じPR/MRでの作業を再開する（ADR-042）
- 応答経路は issue コメントの一本に統一し、受信箱専用の注入経路は作らない
- 受信箱のWeb表示から、設定にあるどのプロジェクトにも新しい issue を作れる。開始ラベルは `labels.ready`・`labels.plan`・`labels.go` から1つ選び、既定は ready とする。issue は受信箱のトークン（本人の身元）で作るので、エージェントがそのまま着手する（ADR-040）

### F5. 通知の受け取り（中継）
- フォージの webhook を中継が受け、署名検証・重複排除のうえ、issue ID のみをローカルエージェントへ通知する
- 中継を使わないポーリングモードを持つ（既定はポーリング、中継は後のフェーズ）

## 4. 状態モデル

ラベルで issue の状態を表す。全リポジトリで同名ラベルを使う。

| ラベル | 意味 | 付与者 |
|---|---|---|
| `agent:ready` | 着手してよい | 人間 |
| `agent:running` | 実行中 | エージェント |
| `agent:waiting` | 人間の応答待ち | フック |
| `agent:done` | 完了（PR 作成済み等） | エージェント / フック |

```
ready ──▶ running ──▶ waiting ──(信頼済みコメント)──▶ running ──▶ done
                         ▲                                 │       │
                         └─────────────────────────────────┘       │
                                   running ◀──(信頼済みコメント)──┘
```

- doneからrunningへ：openのまま `agent:done` のissueに、エージェントのdoneの投稿より後で信頼済みのコメントが付くと、作業を再開する。PR/MRは開いたものを使い続ける。closeすると受信箱から消える（ADR-042）
- ラベルは常に 1 つだけ付く（付け替えはエージェント側で原子的に行う）
- `agent:done` のissueでも、エージェントがpushしたコミットのCIが失敗したら、エージェントが `agent:running` に戻して直す（ADR-041）。上限まで直しても失敗したら `agent:waiting`
- 人間が `agent:ready` を外す・issue を close した場合は実行を止める

## 5. メタデータ埋め込み

エージェント／フックが投稿するコメントの末尾に隠しマーカーを付ける。

```
<!-- switchboard v=1 host=<端末名> session=<セッションID> state=waiting at=<ISO8601> -->
```

- 受信箱はこのマーカーで端末名と待ち開始時刻を得る
- 画像だけのコメントは `state=attachment` を持つ。待ちやdoneの判定では読み飛ばす（ADR-047）
- 人間のコメントにマーカーが含まれていても無視する（投稿者がエージェント用アカウント/自分のトークンであることも確認する）

## 6. CLI（想定）

```
switchboard inbox [--forge github|gitlab] [--json]   # 横断受信箱を表示
switchboard show <ref>                                # 待ちの詳細（最後の質問全文）
switchboard reply <ref> "返答"                        # issue にコメント投稿
switchboard reply <ref> --editor                      # エディタで返答を書く
switchboard agent run                                 # 端末常駐エージェントを起動
switchboard agent status                              # この端末の担当 issue と状態
switchboard labels sync                               # 全対象リポジトリにラベルを作成・統一
switchboard relay serve                               # 中継サーバー（Phase 4 以降）
```

`<ref>` は受信箱の行番号、または `gh:owner/repo#123` / `gl:group/project#45` 形式。

## 7. 非機能要件

- 端末側は受信ポートを開けない。例外は受信箱のWeb表示で、LANのアドレスに限る（ADR-036）
- 中継が停止してもポーリングで機能が継続する
- 一覧取得は 10 リポジトリ程度で数秒以内
- 設定・トークンは OS のキーチェーン等に保存し、平文の設定ファイルに書かない

## 8. 対象外

- チームでの共同利用、権限管理
- Claude Code アプリ（claude.ai/code）のセッション一覧との統合
- 稼働中の対話セッションへの割り込み注入（Channels 等）。必要になった場合は別 ADR で検討する

## 9. 未決事項

ROADMAP.md の「検証項目」を参照。
