# Proposal

## Why

Webの受信箱は、参照の前にプロジェクトのアイコンを出している（project-icons）。新規issueのダイアログのプロジェクトの欄だけは、ネイティブの`<select>`に画像を置けないため、アイコンを出していなかった。issue #138で、この欄のテキストの左にもアイコンを出すよう求められた。

## What Changes

- 新規issueのプロジェクトの欄を、ネイティブの`<select>`から、ライブラリ`@radix-ui/react-select`のリストボックスに替える
- 閉じた欄と、開いた一覧の各選択肢に、アイコン、プレフィックスを外した参照（`mono-sm`）、フォージのバッジ、「（担当：端末）」を並べる。ほかの場所の参照と同じ形にする
- 読み上げの名前は、今までどおり「o/r GitHub（担当：desk）」にする
- 一覧はダイアログの中に出す。一覧が開いているときのEscapeは一覧だけを閉じ、ダイアログは閉じない
- DESIGN.mdから、ネイティブの選択肢でフォージの名前を文字で書く例外と、新規issueの選択肢にアイコンを置かない決まりを消す

## Capabilities

### New Capabilities

### Modified Capabilities
- `web`: 新規issueのプロジェクトの選択肢の出し方を変える

## Impact

- `web/ui/package.json`と`pnpm-lock.yaml`（`@radix-ui/react-select`を足す）
- `web/ui/src/components/ui/select.tsx`（新規）、`web/ui/src/components/NewIssue.tsx`
- `DESIGN.md`の参照、プロジェクトのアイコン、新規issueの項
- スマホでも、OSの選択のUIではなく、この一覧が開く
