---
# DESIGN.md - Google DESIGN.md format (https://github.com/google-labs-code/design.md, version alpha).
# Tokens here are the normative values; the prose below says why and how to apply them.
# Check after every edit: designmd lint DESIGN.md   (0 errors; read the warnings)
version: alpha
name: switchboard web inbox
description: 落ち着いた墨色の作業画面で、待ちの理由だけをアンバーと赤で浮かせ、素早く見分けて返答できるようにする。
colors:
  primary: "#1C1D1F"
  on-primary: "#FFFFFF"
  secondary: "#5F636A"
  on-secondary: "#FFFFFF"
  surface: "#F6F5F2"
  on-surface: "#1C1D1F"
  surface-raised: "#FFFFFF"
  outline: "#E2E0DB"
  muted: "#5F636A"
  attention: "#8A3F06"
  attention-container: "#FBEBD6"
  error: "#B3261E"
  on-error: "#FFFFFF"
  error-container: "#FBE4E1"
  on-error-container: "#7A1A14"
  dark-surface: "#16181C"
  dark-on-surface: "#E8E6E1"
  dark-surface-raised: "#1F2227"
  dark-outline: "#33373E"
  dark-muted: "#A3A7AE"
  dark-primary: "#E8E6E1"
  dark-on-primary: "#16181C"
  dark-attention: "#F5B35C"
  dark-attention-container: "#3A2A14"
  dark-error: "#F28B82"
  dark-error-container: "#3D1D1A"
typography:
  headline-md:
    fontFamily: system-ui, "Hiragino Sans", "Noto Sans JP", sans-serif
    fontSize: 20px
    fontWeight: 600
    lineHeight: 1.3
  body-md:
    fontFamily: system-ui, "Hiragino Sans", "Noto Sans JP", sans-serif
    fontSize: 15px
    fontWeight: 400
    lineHeight: 1.6
  label-md:
    fontFamily: system-ui, "Hiragino Sans", "Noto Sans JP", sans-serif
    fontSize: 13px
    fontWeight: 500
    lineHeight: 1.4
  mono-sm:
    fontFamily: ui-monospace, "SFMono-Regular", Menlo, monospace
    fontSize: 13px
    fontWeight: 400
    lineHeight: 1.4
spacing:
  xs: 4px
  sm: 8px
  md: 12px
  lg: 16px
  xl: 24px
rounded:
  sm: 4px
  md: 6px
  full: 9999px
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    typography: "{typography.label-md}"
    rounded: "{rounded.md}"
    padding: "{spacing.md}"
  button-secondary:
    backgroundColor: "{colors.surface-raised}"
    textColor: "{colors.on-surface}"
    typography: "{typography.label-md}"
    rounded: "{rounded.md}"
    padding: "{spacing.md}"
  card:
    backgroundColor: "{colors.surface-raised}"
    textColor: "{colors.on-surface}"
    rounded: "{rounded.md}"
    padding: "{spacing.lg}"
  badge-attention:
    backgroundColor: "{colors.attention-container}"
    textColor: "{colors.attention}"
    typography: "{typography.label-md}"
    rounded: "{rounded.sm}"
    padding: "{spacing.xs}"
  badge-error:
    backgroundColor: "{colors.error-container}"
    textColor: "{colors.on-error-container}"
    typography: "{typography.label-md}"
    rounded: "{rounded.sm}"
    padding: "{spacing.xs}"
  banner-error:
    backgroundColor: "{colors.error-container}"
    textColor: "{colors.on-error-container}"
    rounded: "{rounded.sm}"
    padding: "{spacing.sm}"
---

# switchboard web inbox design system

## Overview

利用者は1人で、エージェントが残した質問を読み、短く返答する。画面は作業の道具であり、飾りより読みやすさと見分けやすさを優先する。全体は墨色と灰色で静かにし、待ちの理由だけに色を付けて、注意すべき行を一目で見分けられるようにする。デスクトップとスマホの両方で同じ情報を出し、狭い幅では並べ方だけを変える。

## Colors

- Primary（#1C1D1F）：本文の墨色と、返答を送る主ボタン。画面の中で唯一の塗りのボタンにする
- Surface（#F6F5F2）：わずかに温かい灰白の地。純白（`surface-raised`、#FFFFFF）はカードと入力欄だけに使い、地から一段浮かせる
- Outline（#E2E0DB）：カードと表の区切り線。影の代わりに使う
- Muted（#5F636A）：端末の名前、経過時間、補足の文
- Attention（`#8A3F06`、地は`#FBEBD6`）：理由が`question`と`verify`の待ち。人の判断を待っているが、実行は失敗していない
- Error（`#B3261E`、バッジは`#7A1A14`、地は`#FBE4E1`）：理由が`error`と`permission`の待ちと、受信箱のサービスに届かないなどの誤り
- 暗いテーマ：`dark-`で始まる値を、OSの`prefers-color-scheme: dark`のときに使う。役割は明るいテーマと同じで、明るさだけを反転する

## Typography

書体はOSのものを使う。外のCDNからWebフォントを読み込まないためである。和文は、macOSとiOSではヒラギノ角ゴ、それ以外ではNoto Sans JPを順に探す。

- `headline-md`（20px）：画面の見出しと、1件の画面のissueのタイトル
- `body-md`（15px）：質問の全文と、返答の欄。行の長さは、デスクトップでも全角40字程度に収める
- `label-md`（13px）：バッジ、表の見出し、補足
- `mono-sm`（13px）：`gh:o/r#12`のようなissueの参照。等幅にして、桁と区切りを揃える

## Layout

- 中身は幅720pxの中央の列に収める。1件の画面も同じ幅にする
- 幅640px以上では、一覧を表にする（参照、タイトル、理由、端末、経過時間、質問の冒頭）。それより狭いと、1件を1枚のカードにして縦に並べる
- 余白は`spacing`の値だけを使う。カードの内側は`lg`（16px）、カード同士は`sm`（8px）にする
- スマホでは、触る対象を高さ44px以上にする

## Elevation & Depth

影は使わない。階層は、地（`surface`）とカード（`surface-raised`）の明るさの差と、`outline`の線で表す。

## Shapes

角の丸みは2段にする。カード、ボタン、入力欄は`md`（6px）、バッジとバナーは`sm`（4px）である。1つの部品の中で2つを混ぜない。

## Components

- 一覧の行（カード）：1行目に参照（`mono-sm`）とタイトル、2行目に理由のバッジ、端末、経過時間（`muted`）、3行目に質問の冒頭。行全体を1件の画面へのリンクにする
- 理由のバッジ：`question`と`verify`は`badge-attention`、`error`と`permission`は`badge-error`
- 返答の欄：`surface-raised`の地に`outline`の枠。フォーカスでは`primary`の2pxの輪郭を付ける。空のときは送るボタンを押せなくする
- 送るボタン：`button-primary`。送っている間は押せなくし、ボタンの文言を送信中の表示に替える
- 誤りのバナー：`banner-error`。届かなかったフォージ、受信箱のサービスに届かないこと、待ちでなくなったことを、画面の上に出す

## Do's and Don'ts

- Do: 色の付いた要素は、待ちの理由と誤りだけにする
- Do: issueから来た文字列は、テキストとしてだけ描く
- Don't: 本文とラベルを、地に対して4.5:1未満のコントラストにしない
- Don't: 塗りのボタンを、1画面に2つ置かない
- Don't: 幅の狭い画面で、横へのスクロールを出さない
