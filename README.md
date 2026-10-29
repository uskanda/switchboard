# switchboard

GitHub / GitLab の issue を起点に、複数端末・複数プロダクトにまたがる Claude Code の作業を
「応答待ち」単位で一覧し、その場から返答できるようにする個人用ツール。

電話交換台（switchboard）のように、散らばった回線（端末・リポジトリ・セッション）を
一か所で捌くことを目的とする。

## 何を解決するか

- Claude Code のセッションが複数端末・複数リポジトリに散らばり、どれが自分の応答を待っているか分からない
- 各プロダクトの issue を毎回巡回して状況を確認したくない
- 応答を返すために、その端末・そのターミナルまで行きたくない

## 基本方針（要約）

- **issue 駆動**：作業単位・状態・応答経路はすべて issue に寄せる
- **ローカルは外向き通信のみ**：端末側に受信ポートを開けない
- **中継は呼び鈴だけ**：中継サーバーは「issue ID が更新された」ことだけを伝え、本文は必ずローカルがフォージから取り直す
- **状態は issue とリポジトリに置く**：毎回まっさらな文脈で起動できる設計を基本とし、`--resume` は最適化扱い
- **横断一覧はローカルで集約**：両フォージのラベル検索結果をマージして表示する

## ドキュメント

| ファイル | 内容 |
|---|---|
| [CLAUDE.md](CLAUDE.md) | Claude Code で本リポジトリを開発する際の指示 |
| [docs/SPEC.md](docs/SPEC.md) | 想定仕様（要件・状態モデル・CLI） |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 構成・コンポーネント・処理フロー |
| [docs/SECURITY.md](docs/SECURITY.md) | 脅威モデルと対策 |
| [docs/DECISIONS.md](docs/DECISIONS.md) | 設計判断の記録（ADR） |
| [docs/ROADMAP.md](docs/ROADMAP.md) | 段階的な実装計画と検証項目 |
| [docs/PRIOR_ART.md](docs/PRIOR_ART.md) | 調査した類似事例・OSS |
| [config/switchboard.example.yaml](config/switchboard.example.yaml) | 設定ファイル例 |

## ステータス

設計段階。実装は [docs/ROADMAP.md](docs/ROADMAP.md) の Phase 0（検証スパイク）から始める。
