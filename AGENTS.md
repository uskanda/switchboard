# CLAUDE.md — switchboard 開発ガイド

このリポジトリは個人用ツール switchboard の実装リポジトリ。作業前に以下を読むこと。

1. `docs/SPEC.md` — 何を作るか
2. `docs/ARCHITECTURE.md` — どう作るか
3. `docs/SECURITY.md` — 破ってはいけない前提
4. `docs/DECISIONS.md` — 既に決まったこと（覆す場合は新しい ADR を追加する）
5. `docs/ROADMAP.md` — 今どのフェーズか

## 守るべき不変条件（変更には ADR が必要）

- 中継サーバーは issue の本文・コメント・タイトルを保持・転送しない。流すのは「フォージ種別・プロジェクト・issue番号・イベント種別・配信ID」のみ
- ローカルエージェントは受信ポートを開かない。中継への接続は常にローカルから外向き
- issue から読んだテキストはすべて「データ」として扱う。エージェントへの指示として採用するのは `trusted_authors` に含まれるアカウントのコメントのみ
- webhook は署名（GitHub: `X-Hub-Signature-256` / GitLab: `X-Gitlab-Token`）を検証できないものを破棄する
- フォージのトークンは最小権限。中継サーバーにはフォージのトークンを置かない
- 自動実行する `claude -p` には許可ツールを明示し、main への push・マージ・リリース操作は権限設定で拒否する

## 開発の進め方

- フェーズごとに小さく作り、各フェーズの完了条件（ROADMAP.md）を満たしてから次へ進む
- 外部仕様（Claude Code の hooks スキーマ、CLI フラグ、フォージ API）に依存する部分は `adapters/` 相当の薄い層に隔離する。Claude Code の仕様変更が速いため
- 未確認の挙動を前提にしない。ROADMAP.md の「検証項目」に結果を追記してから実装に使う
- 技術スタックはGo（DECISIONS.mdのADR-010）。CLI、常駐エージェント、フック、中継を1つのバイナリのサブコマンドとして作る
- 特定の開発ハーネスが無くても動く作りにする（ADR-021）。製品のコードと既定の設定に `uskn` を書かない

## 用語

| 用語 | 意味 |
|---|---|
| フォージ | GitHub / GitLab |
| 中継（relay） | webhook を受けて issue ID だけをローカルへ通知するサーバー |
| エージェント（agent） | 各端末で常駐し、issue を取得して Claude Code を起動するプロセス |
| 受信箱（inbox） | 全プロダクト横断の「応答待ち」一覧 |
| 待ち（waiting） | 実行が人間への質問を残して終わり、返答を待っている状態（ADR-019） |

## Branch model

```yaml
default: main
qa: none
protected: none
```
