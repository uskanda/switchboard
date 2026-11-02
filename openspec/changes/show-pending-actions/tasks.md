# Tasks

## 1. 処理中の印

- [ ] 1.1 `lib/pending.ts`に印の型、`settle`、`overlay`、sessionStorageの読み書きを作る。`lib/pending.test.ts`で、印が外れる場合を確かめる（状態の変化、始まりの時刻だけの変化、一覧から消えた、新規issueが現れた）。10分の時間切れと、形の合わない保存値を捨てることも確かめる
- [ ] 1.2 `lib/inbox.tsx`に`view`と`markPending`を足し、一覧が届くたびの`settle`、期限のタイマー、時間切れのトーストを作る。`pnpm typecheck`が通ることを確かめる

## 2. 呼び出しと表示

- [ ] 2.1 `MergeButton`、`IssuePane`、`NewIssue`で、依頼の成功のあとに`markPending`を呼ぶ。処理中のマージのボタンは押せなくする
- [ ] 2.2 `InboxList`、`Header`、`lib/title.ts`を`view`から作り、処理中の行にSpinnerと明滅する文言、説明の行、`role="status"`を出す。`index.css`に`pending-pulse`を足す
- [ ] 2.3 `pages/pending.test.tsx`で、返答、マージ、新規issue、依頼の失敗、10分の時間切れ、読み込み直しを確かめる

## 3. 並べ替えの動き

- [ ] 3.1 `lib/flip.ts`に`useFlip`を作り、`InboxList`の行に`data-flip-key`を付ける。`lib/flip.test.tsx`で、位置の差の分の`transform`で`animate`が呼ばれる、新しい行はフェードインする、reduced-motionでは呼ばれないことを確かめる

## 4. 方針と全体の確認

- [ ] 4.1 `PRODUCT.md`の動きの方針に、処理中の表示と並べ替えの例外を1行足す
- [ ] 4.2 `make verify`が通ることを確かめる
- [ ] 4.3 `openspec validate show-pending-actions --strict`が通ることを確かめる
