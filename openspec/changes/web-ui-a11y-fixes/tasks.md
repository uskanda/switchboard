# Tasks

## 1. 見出し

- [ ] 1.1 `Markdown.test.tsx`に、`#`から`######`の見出しが2段下げて描かれるテストを足し、失敗を確かめる
- [ ] 1.2 `Markdown.tsx`の`components`に`h1`から`h6`を足し、`index.css`の`.md`の見出しの書式を`h3`から`h6`に当てる
- [ ] 1.3 `planAnswers.test.tsx`に、計画の質問の題がh3で、本文の`##`がh4で描かれるテストを足し、失敗を確かめる
- [ ] 1.4 `PlanQuestions.tsx`の題を`<h3>`にする

## 2. マージのダイアログ

- [ ] 2.1 `merge.test.tsx`に、ダイアログを開くと「やめる」にフォーカスがあるテストを足し、失敗を確かめる
- [ ] 2.2 `MergeButton.tsx`で、`showModal()`の直後に「やめる」へフォーカスを移す

## 3. 一覧のカード

- [ ] 3.1 `InboxList`のテストに、カードのリンクへ`focus-visible:outline-offset-2`が付くテストを足し、失敗を確かめる
- [ ] 3.2 `InboxList.tsx`のカードのリンクに`focus-visible:outline-offset-2`を足す

## 4. DESIGN.mdと通しの確認

- [ ] 4.1 `DESIGN.md`の「本文とコメントの書式」「計画の質問のカード」「マージ」「一覧の行（カード）」の項に書き足す
- [ ] 4.2 `make verify`を通す
- [ ] 4.3 `switchboard-web`を幅1024px以上の画面で開き、左のペインのカードにTabでフォーカスを当てる。離した輪郭が箱の端で切れないことと、選んでいるカードで枠と輪郭が二重に見えることを目で確かめる
