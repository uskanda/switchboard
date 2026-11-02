# Design

## Context

一覧は`InboxProvider`が受け取る`Inbox`をそのまま節に分けて描く。返答とマージのあとは受信箱のサービスがすぐ読み直すが、エージェントが実行を始めるまで状態は変わらない。WUIには、押した操作を一覧が届くまで覚えておく仕組みが無い。動機はproposal.mdのWhyを参照。

## Goals / Non-Goals

**Goals:**
- 押した本人のタブで、操作の直後から一覧の行に処理中を出す
- 一覧のすべての更新で、行の移動を動きで見せる

**Non-Goals:**
- ほかのタブや端末に処理中を見せること。受信箱のサービスと`inboxapi`の型は変えない
- 消える行のフェードアウト

## Decisions

### 印は`lib/pending.ts`の純粋な関数で扱う

印は`{ ref, kind: "merge" | "reply" | "create", at, base }`とする。`base`は依頼の時点の状態の鍵で、stateと、その状態の始まりの時刻（`waiting_since`か`done_since`）をつないだ文字列である。新規issueの`base`は`null`（一覧に無い）とし、仮の行のためのタイトル、URL、project、番号を持たせる。

- `settle(marks, items, now)`は、印を「残す」「終わった」「時間切れ」に分ける。次のどれかに当たる印は終わったとする：鍵が`base`と違う／一覧から消えた／新規issueが一覧に現れた。鍵に始まりの時刻を含めるのは、計画の実行が次のポーリングまでに済み、ふたたび待ちに戻った場合も「変わった」と扱うためである
- `overlay(items, marks)`は、画面に出す行を作る。replyとcreateの行はstateを`running`に置き換え、`pending`を付ける。mergeの行はstateを変えずに`pending`を付ける。createの印で一覧に無いものは、仮の行として足す
- sessionStorageの鍵は`switchboard.pending`。形の合わない値は捨てる

純粋な関数にすると、状態の組み合わせを単体テストで確かめやすい。

### `InboxProvider`が印を重ねた行（`view`）を配る

`InboxState`に`view`（印を重ねた`Item[]`、一覧が届く前は`null`）と`markPending`を足す。一覧が届くたびに`settle`を通し、時間切れの印にはトーストを出す。一覧が届かない間も時間切れになるよう、いちばん早い期限に合わせて`setTimeout`を張る。一覧（`InboxList`）、ヘッダ、タブのタイトルは`view`から数える。1件の画面（`IssuePane`）は今のとおり`inbox`を使い、既存の「送りました」の表示を残す。

代わりに各部品で印を重ねる案は、数え方が部品ごとにずれるので採らない。

### 並べ替えはFLIPを自前で書く（`lib/flip.ts`）

`useFlip(containerRef)`は、各行の要素の`data-flip-key`で位置を覚える。`useLayoutEffect`で描画の後に`getBoundingClientRect`で測り、前回の位置との差を、要素の`animate`に渡す最初のフレームの`transform`にする。約250msで元の位置へ戻す。前回に無かった鍵は、不透明度を0から1へ動かす。

表（640〜1023px）とカードは両方ともDOMにあり、CSSでどちらかを隠している。幅と高さが0の要素（隠れている側）は測らずに飛ばす。一覧全体を囲む要素で測るので、節をまたぐ移動も同じ仕組みで動く。`prefers-reduced-motion`のときと、`animate`が無い環境（jsdomなど）では何もしない。

View Transitions APIはページ全体の描画が遷移の間固まり、返答を書いている最中の欄にも影響する。ライブラリは依存と静的ファイルが増える。どちらも採らない。

### 明滅はCSSのクラスで付ける

`index.css`に`pending-pulse`（2秒周期で不透明度を1から0.5へ上下させる）を足す。既存の`prefers-reduced-motion`の指定がすべてのアニメーションを止めるので、止まったときは不透明度1のまま読める。Spinnerも同じ指定で止まる。

## Risks / Trade-offs

- エージェントの端末が止まっていると、印が外れない。対策：10分で外し、トーストで伝える
- 別のタブや端末では処理中が見えない。対策：この変更の目的は押した本人への反応なので受け入れる
- 完了のissueにコメントしたあと、別の理由で`done_since`が変わると、印が早めに外れる。対策：外れても実際の状態が出るだけで、誤った表示にはならない
- jsdomには`animate`が無く、レイアウトも計算しない。対策：テストでは`getBoundingClientRect`と`animate`を差し替えて確かめる
