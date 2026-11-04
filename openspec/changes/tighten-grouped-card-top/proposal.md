## Why

「プロジェクトでまとめる」がオンのとき、左のペインのissueのカードは、上が下より空いて見える（issue #154）。まとめる表示ではカードの1行目の参照が消え、タイトルの行の高さがカードの上の余白に足されるためである。

## What Changes

- まとめる表示のカードで、上の余白を`lg`（16px）から`md`（12px）に詰める。左右と下は`lg`のまま
- まとめない表示のカードと、640〜1024pxの表は変えない
- DESIGN.mdの「プロジェクトのサブグループ」に、この余白を書き足す

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- なし。見た目の余白だけの変更で、webの要件は変えない。そのため `.openspec.yaml` に `skip_specs: true` を置く

## Impact

- `web/ui/src/components/InboxList.tsx`（カードのリンクの余白）
- `web/ui/src/pages/grouping.test.tsx`（余白のテスト）
- `DESIGN.md`
