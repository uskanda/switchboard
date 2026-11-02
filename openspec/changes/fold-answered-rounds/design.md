# Design

## Context

1件の表示のスレッドは、`scrollTarget.ts`の`shown`が選んだコメントを、`Thread.tsx`が平らな`<ol>`に描く。各カードの`<li>`は元のコメントのindexを`data-index`に持ち、`IssuePane.tsx`の`scrollToCard`がそれを探してスクロールする。知らせを出すかは、`shown`をもとにした`cardsOf`と`addedCards`で決める。コメントのkindでは、計画の質問と計画の提示を区別できない。どちらも`waiting`である。

## Goals / Non-Goals

**Goals:**

- 古い待ちのラウンドを1行に畳み、押すと開けるようにする
- 開いた直後のスクロールと知らせの動きを変えない

**Non-Goals:**

- 開いた状態をissueをまたいで覚えること
- 本文の解析（`planQuestions`）で畳む対象を選ぶこと

## Decisions

### 並びはlibの関数で作る

`threadOf(comments)`を足し、`shown`の各項目と、畳んだラウンド`{ kind: "round", question, replies }`の並びを返す。ラウンドの中の項目も元のindexを持つ。ラウンドは、最新の待ちより前のエージェントの`waiting`から始まる。続く人のコメントを取り込み、次のエージェントの投稿の手前で終わる。着手や順番待ちの1行と進捗も、エージェントの投稿に数える。Thread.tsxの中で組むより、純粋な関数のほうがテストを書きやすい。

`cardsOf`と`addedCards`は`shown`のままにする。描くカードの集まりは畳んでも変わらないので、知らせを出す条件も変わらない。`targetOf`も変えない。待ちのissueの行き先は最新の待ちで、これは畳まない。完了のissueの行き先は完了の報告か最後のカードで、どちらもラウンドの中に入らない。

### 開け閉めはネイティブの`<details>`

ラウンドは`<li><details><summary>…</summary><ol>カード</ol></details></li>`にする。`<details>`はキーボードの操作と読み上げをはじめから持ち、開いた状態をDOMが持つ。Reactから`open`を渡さず、keyをラウンドの質問のindexにする。同じissueの読み直しでは要素が作り直されないので、開いた状態が残る。別のissueへ移ったときは、IssuePaneが`<Thread key={detail.ref}>`で作り直し、畳み直す。

ボタンと`useState`で作り、issueごとに覚える案もあった。古いラウンドを開くのは一時的に見返すときだけなので、覚える仕組みは要らないと判断した（grilling.mdのQ5）。

### 行き先が畳んだ中にあれば開く

`revealAndScroll(root, index)`をlibに切り出す。`data-index`の要素を探し、閉じた`<details>`の中にあれば`open`にしてから`scrollIntoView`を呼ぶ。開いた直後のスクロールと、知らせを押したときの両方がこれを使う。jsdomにはスクロールが無いので、テストでは`scrollIntoView`をstubにする。

固定の見出しの下で止める`scroll-mt`は、`[&>li]`から`[&_[data-index]]`に付け替える。入れ子のカードも見出しの下で止まる。

## Risks / Trade-offs

- 知らせの行き先の監視：畳んだラウンドの中のカードは見えないので、IntersectionObserverは交わりを知らせない → 知らせを押せば開いてスクロールし、知らせは消える
- ブラウザのページ内検索：閉じた`<details>`の中は、ブラウザによっては検索で開かない → 古いラウンドは見返すときだけ開く前提なので受け入れる
