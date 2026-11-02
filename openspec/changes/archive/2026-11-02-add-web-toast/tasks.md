# Tasks

## 1. 見た目の決まり

- [x] 1.1 `DESIGN.md`に、成功の色と4種類のトーストの部品を足す。色は`success`、`on-success-container`、`success-container`で、`success`と`success-container`には`dark-`の値も足す。トーストの項を足し、新規issue、プロジェクトの節、指示の節の知らせをトーストに書き換える。Do'sの色の一文を改める。`designmd lint DESIGN.md`で0 errorsを確かめる
- [x] 1.2 `pnpm run tokens`で`src/tokens.css`を作り直し、`src/index.css`の暗いテーマに成功の色を足す

## 2. トーストの部品（`web/ui`）

- [x] 2.1 sonnerを依存に足し、`pnpm-lock.yaml`を更新する
- [x] 2.2 `toast.test.tsx`を先に書き、失敗を確かめる。種類ごとのクラスと役割、✗のボタン、既定の5秒と指定した時間、マウスとフォーカスの間に止まること、6個目で古いものが消えることを確かめる
- [x] 2.3 `lib/toast.ts`と`components/ui/toaster.tsx`を作る。jsdomに無い`setPointerCapture`を`test-setup.ts`で補う
- [x] 2.4 `App.tsx`で、受信箱と設定の両方の画面に`Toaster`を置く

## 3. 知らせの移し替え

- [x] 3.1 `NewIssue`：一覧の上の知らせを取り除き、成功のトーストで参照とフォージへのリンクを出す。`newIssue.test.tsx`を更新する
- [x] 3.2 `SettingsPage`：「保存しました」を成功のトーストに替え、誤りは欄に残す。`plan.test.tsx`を更新する
- [x] 3.3 `ProjectsSection`：「追加しました」を成功のトーストに替え、webhookのコマンドは一覧の下に残す。`projects.test.tsx`を更新する
- [x] 3.4 `make verify`を通す

## 4. 実物での確認

- [ ] 4.1 `pnpm dev`で、右下の積み方、幅1024px未満での全幅の表示、暗いテーマの色、✗のボタンとキーボードでの操作を見る
- [x] 4.2 `add-web-issue-create`をアーカイブしたあとで、この変更をアーカイブする
