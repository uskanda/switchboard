# Design

## Context

WUIは、APIの`ref`（`gh:o/r#12`）を、そのまま画面に出していた。参照は一覧、1件の表示、設定の画面、新規issueのダイアログ、トーストに出る。DESIGN.mdは、色の付いた要素を、待ちの理由、誤り、トーストの種類に限っていた。

## Goals / Non-Goals

**Goals:**
- 画面のどこでも、参照を「プレフィックスの無い参照＋フォージのバッジ」の同じ形で出す
- バッジの色を、既存の待ちのアンバー、誤りの赤、成功の緑と見分けられるようにする

**Non-Goals:**
- CLIの出力、入力の形式、URL、APIの`ref`を変えること

## Decisions

- 参照を分ける処理を`lib/ref.ts`の`splitRef`に置く。`gh:`はgithub、`gl:`はgitlabとし、どちらでもない文字列はフォージ無しで返す。`<option>`や保存のトーストのように文字しか置けない場所では、`refText`の作る「o/r GitHub」を使う
- 描画は`components/IssueRef.tsx`の`IssueRef`と`ForgeBadge`にまとめる。参照は`mono-sm`、バッジは`badge-github`と`badge-gitlab`にする。`ref`の値、リンク先、`key`、選択の判定は元の`ref`のまま使う
- 色はDESIGN.mdのcolorsに足す。名前は`forge-github`、`forge-gitlab`と、それぞれの`-container`、`dark-`の値である。`pnpm tokens`で`tokens.css`を書き出し、暗いテーマの差し替えは`index.css`に足す。どの組も文字と地のコントラストは4.5:1を超える
- 指示の欄のラベルにバッジを入れると、欄のアクセシブルな名前は「o/r GitHub」になる。テストはこの名前で欄を探す

## Risks / Trade-offs

- DESIGN.mdの「色の付いた要素」の決まりが広がる → Doの文にフォージのバッジを明記し、ほかの用途で色を足さないようにする
- 画面の参照とCLIの参照の形が異なる → CLIの参照は、コピーして引数に渡すためのものである。形を変えると貼れなくなる。画面の形は読むためのものとして分けた
