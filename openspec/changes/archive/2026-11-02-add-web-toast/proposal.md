# Proposal

## Why

WUIで新規issueを作ると、作ったissueの知らせが一覧の上に残り、閉じるまで一覧を押し下げる。設定の画面の「保存しました」や「追加しました」も、欄の中に残り続ける。成功の知らせは、少しの間だけ見えれば足りる。ほかの知らせでも使える汎用のトーストを作り、成功の知らせをそこへ移したい（issue #66）。

## What Changes

- WUIに汎用のトーストを足す。種類は通常、成功、警告、誤りの4つで、種類ごとに色と印を付ける。✗のボタンで閉じられ、既定は5秒で消える。マウスを載せている間と、フォーカスがトーストの中にある間は、時間を止める
- トーストは右下に出し、重ねずに縦へ並べる。同時に出すのは5個までとし、6個目を出すと古いものから消す。幅1024px未満では下の端に全幅で出す
- 新規issueを作れたときの知らせを、一覧の上の知らせから、成功のトーストに替える
- 設定の画面で、追加の指示の保存と、プロジェクトの追加に成功したときの知らせを、成功のトーストに替える。webhookの登録コマンドと、欄ごとの誤りは、今のまま欄に残す
- DESIGN.mdに成功の色のトークンとトーストの部品を足す
- web/uiの依存にsonnerを足す。呼び出す側は`lib/toast.ts`だけを使う

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: トースト、issueの作成の結果の表示、設定の画面の保存の結果、プロジェクトの追加の結果の表示

## Impact

- `web/ui/src/lib/toast.ts`（新規）：トーストを出す薄い層と、表示時間のタイマー
- `web/ui/src/components/ui/toaster.tsx`（新規）：sonnerの`Toaster`にDESIGN.mdの見た目を当てる
- `web/ui/src/App.tsx`：受信箱と設定の両方の画面に`Toaster`を置く
- `web/ui/src/components/NewIssue.tsx`、`ProjectsSection.tsx`、`pages/SettingsPage.tsx`：成功の知らせをトーストに替える
- `DESIGN.md`、`web/ui/src/tokens.css`、`web/ui/src/index.css`：成功の色とトースト
- `web/ui/package.json`、`pnpm-lock.yaml`：sonnerを足す
- 核（Go）と、受信箱のサービスのAPIは変えない
