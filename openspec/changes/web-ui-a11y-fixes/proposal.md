# Proposal

## Why

Web UIの使いやすさの監査で、アクセシビリティの細かい不足が4つ見つかった（#88）。コメントの中のMarkdownの`#`がh1として描かれ、ページの見出しの階層を乱す。計画の質問の題がh4で、h3を飛ばす。マージの確認ダイアログを開くと、最初のフォーカスが「やめる」ではなくPR/MRのリンクに当たる。一覧で、選んでいるカードとフォーカスしているカードが同じ見た目になる。

## What Changes

- 本文とコメントのMarkdownの見出しを2段ずつ下げて描く。h1はh3、h2はh4、h3はh5、h4からh6はh6にする
- 計画の質問の題をh3の見出しにする
- マージの確認ダイアログを開いたとき、「やめる」のボタンにフォーカスを当てる
- 一覧のカードのフォーカスの輪郭を、カードから2px離して描く。選んでいるカードの枠と見分けられるようにする
- DESIGN.mdの該当の項に、上の描き方を書き足す

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: Markdownの見出しの段、計画の質問の題の見出し、マージのダイアログの最初のフォーカス、一覧のカードのフォーカスの輪郭

## Impact

- `web/ui/src/components/Markdown.tsx`、`web/ui/src/index.css`
- `web/ui/src/components/PlanQuestions.tsx`
- `web/ui/src/components/MergeButton.tsx`
- `web/ui/src/components/InboxList.tsx`
- `web/ui/src`の各テスト
- `DESIGN.md`の「本文とコメントの書式」「計画の質問のカード」「マージ」「一覧の行（カード）」の項
