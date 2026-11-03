# Design

## Context

新規issueのダイアログ（`NewIssue.tsx`）は、ネイティブの`<dialog>`を`showModal()`で開く。プロジェクトの欄はネイティブの`<select>`で、選択肢を「o/r GitHub（担当：desk）」の文字で出していた。`<option>`には画像とバッジを置けないため、project-iconsでは、この欄にアイコンを出さないと決めていた。参照の前のアイコンは`IssueRef`と`ProjectIcon`が出す。web/uiには、これまで部品のライブラリは無かった。

## Goals / Non-Goals

**Goals:**
- 閉じた欄と開いた一覧の両方に、ほかの場所と同じ形の参照（アイコン、参照、バッジ）を出す
- 読み上げの名前を「o/r GitHub（担当：desk）」のまま保つ
- キー操作で開き、選べるようにする。Escapeでダイアログまで閉じないようにする

**Non-Goals:**
- ほかの`<select>`やラジオボタンを置き換えること
- プロジェクトを検索して絞り込むこと

## Decisions

- `@radix-ui/react-select`を使う（issue #138 Q4 B）。キー操作、頭文字での移動、フォーカスの戻り、ARIAの役割をライブラリに任せる。`components/ui/select.tsx`に、shadcn風の薄い包みを置く
  - 引き金は`Input`と同じ見た目にする。高さ44px、`surface-raised`の地に`field-outline`の枠で、フォーカスでは`primary`の2pxの輪郭を出す。右端に開くことを示す印を置く
  - 一覧は`position="popper"`で引き金の下に、引き金と同じ幅で出す。`surface-raised`の地に`outline`の枠で描き、DESIGN.mdに従い影は付けない。各行は高さ44px以上にする。選んでいる行には`primary`の印を左に付け、キーで当たっている行は`surface`の地と`primary`の輪郭で示す
- 一覧はダイアログの中へportalする。`showModal()`で開いたダイアログの外側はinertになり、bodyに出すと一覧を操作できないためである。`NewIssue`はダイアログの要素をコールバックのrefで状態に持ち、`SelectContent`の`container`に渡す
- 一覧が開いている間は、ダイアログの`cancel`を`preventDefault`で止める。Escapeは一覧だけを閉じる。ブラウザが一覧を閉じたkeydownの後に`cancel`を送る場合に備え、止めるのは一覧が閉じてから1タスク後まで続ける。開いているかは、包みの`Select`が開閉の状態を持ち、contextで`SelectContent`へ渡す。Radixは閉じている間も選択肢を描くため、`SelectContent`が描かれていることでは判断できない
- 読み上げの名前は、各選択肢の`aria-label`に「o/r GitHub（担当：desk）」を明示する。Radixは選択肢の名前を`aria-labelledby`で中身の文字から取る。この方法では、読み上げから外したアイコンの頭文字も名前に入り、中身から取るとバッジの前後に空白が入るためである。包みの`SelectItem`は`aria-labelledby`を外す
- 閉じた欄の表示は、Radixの`SelectValue`が選んだ選択肢の中身を写す。欄の読み上げの名前は`aria-labelledby`でラベルの「プロジェクト」にする
- project-iconsの差分の仕様にある「新規issueのプロジェクトの選択肢には使わない」の1文は、この変更で覆す。project-iconsはmainにマージ済みで、まだアーカイブしていない。アーカイブするときに、この1文を外す

## Risks / Trade-offs

- スマホでOSの選択のUIが使えなくなる → 一覧の各行を44px以上にし、高さは画面に収まるだけに抑えてスクロールさせる
- 依存とバンドルが増える → Radix Selectの関連のパッケージだけで、使うのは1か所である
- jsdomでは`<dialog>`のEscapeを再現できない → `cancel`のイベントを直接送り、止められることをテストで確かめる。実際のブラウザでの動きは手で確かめる
