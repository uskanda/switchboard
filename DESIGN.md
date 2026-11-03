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
  field-outline: "#85888E"
  muted: "#5F636A"
  attention: "#8A3F06"
  attention-container: "#FBEBD6"
  error: "#B3261E"
  on-error: "#FFFFFF"
  error-container: "#FBE4E1"
  on-error-container: "#7A1A14"
  success: "#1E6B37"
  on-success-container: "#14532D"
  success-container: "#E2F2E5"
  forge-github: "#24292F"
  forge-github-container: "#E6EBF1"
  forge-gitlab: "#5B3BB0"
  forge-gitlab-container: "#EEE8FB"
  dark-surface: "#16181C"
  dark-on-surface: "#E8E6E1"
  dark-surface-raised: "#1F2227"
  dark-outline: "#33373E"
  dark-field-outline: "#6B7078"
  dark-muted: "#A3A7AE"
  dark-primary: "#E8E6E1"
  dark-on-primary: "#16181C"
  dark-attention: "#F5B35C"
  dark-attention-container: "#3A2A14"
  dark-error: "#F28B82"
  dark-error-container: "#3D1D1A"
  dark-success: "#7FD39A"
  dark-success-container: "#16301F"
  dark-forge-github: "#C9D1D9"
  dark-forge-github-container: "#252C35"
  dark-forge-gitlab: "#C4B5FD"
  dark-forge-gitlab-container: "#2A2140"
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
  badge-github:
    backgroundColor: "{colors.forge-github-container}"
    textColor: "{colors.forge-github}"
    typography: "{typography.label-md}"
    rounded: "{rounded.sm}"
    padding: "{spacing.xs}"
  badge-gitlab:
    backgroundColor: "{colors.forge-gitlab-container}"
    textColor: "{colors.forge-gitlab}"
    typography: "{typography.label-md}"
    rounded: "{rounded.sm}"
    padding: "{spacing.xs}"
  banner-error:
    backgroundColor: "{colors.error-container}"
    textColor: "{colors.on-error-container}"
    rounded: "{rounded.sm}"
    padding: "{spacing.sm}"
  toast-info:
    backgroundColor: "{colors.surface-raised}"
    textColor: "{colors.on-surface}"
    typography: "{typography.label-md}"
    rounded: "{rounded.md}"
    padding: "{spacing.md}"
  toast-success:
    backgroundColor: "{colors.success-container}"
    textColor: "{colors.on-success-container}"
    typography: "{typography.label-md}"
    rounded: "{rounded.md}"
    padding: "{spacing.md}"
  toast-warning:
    backgroundColor: "{colors.attention-container}"
    textColor: "{colors.attention}"
    typography: "{typography.label-md}"
    rounded: "{rounded.md}"
    padding: "{spacing.md}"
  toast-error:
    backgroundColor: "{colors.error-container}"
    textColor: "{colors.on-error-container}"
    typography: "{typography.label-md}"
    rounded: "{rounded.md}"
    padding: "{spacing.md}"
---

# switchboard web inbox design system

## Overview

利用者は1人で、エージェントが残した質問を読み、短く返答する。画面は作業の道具であり、飾りより読みやすさと見分けやすさを優先する。全体は墨色と灰色で静かにし、待ちの理由だけに色を付けて、注意すべき行を一目で見分けられるようにする。デスクトップとスマホの両方で同じ情報を出し、狭い幅では並べ方だけを変える。

## Colors

- Primary（#1C1D1F）：本文の墨色と、返答を送る主ボタン。画面の中で唯一の塗りのボタンにする
- Surface（#F6F5F2）：わずかに温かい灰白の地。純白（`surface-raised`、#FFFFFF）はカードと入力欄だけに使い、地から一段浮かせる
- Outline（#E2E0DB）：カードと表の区切り線。影の代わりに使う
- Muted（#5F636A）：端末の名前、経過時間、補足の文
- Attention（`#8A3F06`、地は`#FBEBD6`）：理由が`question`、`verify`、`plan-questions`、`plan`の待ち。人の判断を待っているが、実行は失敗していない
- Error（`#B3261E`、バッジは`#7A1A14`、地は`#FBE4E1`）：理由が`error`、`permission`、`push`、`ci`の待ちと、受信箱のサービスに届かないなどの誤り
- Success（`#1E6B37`、文字は`#14532D`、地は`#E2F2E5`）：成功のトーストだけに使う。作成や保存が済んだことを知らせる
- Forge：フォージのバッジだけに使う。GitHubは青みの灰色（文字は`#24292F`、地は`#E6EBF1`）、GitLabは紫（文字は`#5B3BB0`、地は`#EEE8FB`）。待ちの理由のアンバー、赤、成功の緑と重ならない淡い色にし、理由のバッジより控えめに見せる
- 暗いテーマ：`dark-`で始まる値を、OSの`prefers-color-scheme: dark`のときに使う。役割は明るいテーマと同じで、明るさだけを反転する

## Typography

書体はOSのものを使う。外のCDNからWebフォントを読み込まないためである。和文は、macOSとiOSではヒラギノ角ゴ、それ以外ではNoto Sans JPを順に探す。

- `headline-md`（20px）：画面の見出しと、1件の画面のissueのタイトル
- `body-md`（15px）：質問の全文と、返答の欄。行の長さは、デスクトップでも全角40字程度に収める
- `label-md`（13px）：バッジ、表の見出し、補足
- `mono-sm`（13px）：`o/r#12`のようなissueの参照。等幅にして、桁と区切りを揃える

## Layout

- 画面の上に全幅のヘッダを置く。ヘッダは、スクロールしても上に残す
- 幅1024px以上では、ヘッダの下を2つのペインに分ける。左は幅360pxの一覧、右は選んだissueで、どちらも別々に縦にスクロールする。ペインの境目をドラッグすると、一覧の幅を280pxから640pxの間で変えられる。右のペインには480px以上を残す。境目はフォーカスでき、左右の矢印キーで16pxずつ、HomeとEndで端まで動かせる。ダブルクリックで360pxに戻る。変えた幅はブラウザに覚えさせる。普段の境目は`outline`の1pxの線のままにする。マウスを乗せると線を`outline`の2pxにし、動かしている間とフォーカスの間は`primary`の2pxにする。右のペインの中身は、幅720pxの中央の列に収める。issueを選んでいないとき、右のペインには一覧から選ぶよう案内する。一覧の下には、キーの案内「j/k：待ちの間を移る　r：返答の欄へ」を`label-md`の`muted`で1行出す
- `j`と`k`で、一覧の「返答待ち」の節の中を1つ下と1つ上へ移り、そのissueを開く。端で止まる。issueを開いていないときは、`j`で先頭、`k`で末尾の待ちを開く。`r`で返答の欄へフォーカスを移す。入力中、修飾キーと一緒のとき、ダイアログが開いているときは受けない。幅1024px未満の画面でも同じに動く
- 幅1024px未満では、一覧と1件を別の画面にする。どちらも幅720pxの中央の列に収め、1件の画面の上に一覧へ戻るリンクを置く
- 1件の画面の上の見出し（一覧へ戻るリンク、参照、タイトル、状態の行）は、スクロールしても残す。幅1024px未満ではヘッダの直下に、幅1024px以上では右のペインの上端に固定する。スクロールで見せるカードは、固定した見出しの直下で止める
- 一覧だけの画面では、幅640px以上で一覧を表にする（参照、タイトル、理由、端末、経過時間、質問の冒頭）。それより狭い画面と、左のペインでは、1件を1枚のカードにして縦に並べる
- 設定の画面は、同じヘッダの下に、幅720pxの中央の列で出す
- 余白は`spacing`の値だけを使う。カードの内側は`lg`（16px）、カード同士は`sm`（8px）にする
- スマホでは、触る対象を高さ44px以上にする

## Elevation & Depth

影は使わない。階層は、地（`surface`）とカード（`surface-raised`）の明るさの差と、`outline`の線で表す。

## Shapes

角の丸みは2段にする。カード、ボタン、入力欄、トーストは`md`（6px）、バッジとバナーは`sm`（4px）である。1つの部品の中で2つを混ぜない。

## Components

- 参照：issueとプロジェクトの参照は、フォージのプレフィックス（`gh:`、`gl:`）を外して`mono-sm`で出し、すぐ後ろにフォージのバッジを置く（例：`o/r#12`［GitHub］）。バッジを置けないネイティブの選択肢では、「o/r GitHub」のように、フォージの名前を参照の後ろに文字で書く。入力の欄、URL、コピーして使うコマンドは、プレフィックスの付いた形のままにする
- フォージのバッジ：GitHubは`badge-github`、GitLabは`badge-gitlab`。文字は公式の綴りの「GitHub」「GitLab」にし、色は見分けの補助にとどめる
- ヘッダ：左から、名前（「SwitchBoard Inbox」。一覧へのリンク。スマホなど幅の狭い画面では「SwitchBoard」に縮める）、待ちと実行中の件数、自動更新の状態、右端に設定へのリンク。件数と状態は`label-md`の`muted`にする。幅640px未満では、設定へのリンクを名前と同じ1行目の右端に残し、件数と状態をその下に折り返す。状態の前には点を置き、つながっている間は`primary`で塗り、それ以外は`muted`の輪郭だけにする。つながっている間の文言は、受信箱のサービスと中継の状態で分ける。中継につながっていれば「自動更新中（即時）」、切れていれば「自動更新中（N秒ごと）」（Nは`inbox.refresh_interval`の秒数）、`inbox.relay`が無ければ「自動更新中」とする。点の印は中継の状態では変えない
- 一覧の行（カード）：1行目に参照（`mono-sm`）とフォージのバッジ、タイトルを出す。2行目に理由のバッジ、端末、経過時間（`muted`）、3行目に質問の冒頭（Markdownの太字とコードの記号を除いた文）を出す。実行中の行では、3行目に進捗の最後の行を出し、進捗がまだ無ければ「進捗はまだありません」とする。行全体を1件の画面へのリンクにする。左のペインで選んでいる行は、`primary`の2pxの枠で示す。フォーカスの輪郭は`primary`の2pxで、カードから2px離して描く。選んでいる行の枠と見分けるためである
- スレッド：issueの本文とコメントを、1件ずつカードにして古い順に並べる。カードの上に、書いた人と時刻を`label-md`の`muted`で出す。エージェントの投稿には、`outline`の枠の「エージェント」の印を付ける。色は付けない。エージェントの投稿では、書いた人のアカウント名は出さない。エージェントは人のアカウントで投稿することが多く、名前を並べるとその人が書いたように読めるためである。着手のコメントはカードにせず、印と時刻の1行にする。エージェントの最新の質問のカードは、`primary`の2pxの枠で目立たせる。それより前の質問は、続く人の返答と合わせて1行に畳み、`<details>`で押すと開く。この1行は着手の行と同じ見た目で、印・「答えた質問・返答N件」・時刻・`▸`を並べる
- 1件の見出し：`surface`の地で、下に`outline`の線を引く。中身は上から、一覧へ戻るリンク（幅1024px未満だけ）、参照（`mono-sm`）とフォージのバッジ、タイトル（`headline-md`）、状態の行である。状態の行には、状態か理由のバッジ、端末、経過時間、フォージで開くリンクを並べる。スクロールが一番上のときはタイトルを全文出し、それ以外では2行で切って「…」を付ける。切ったときは全文をtitle属性に持たせる
- 自動更新の知らせ：1件を開いている間の読み直しでカードが増えたとき、右のペインの中央の列の下に「自動更新がありました」のボタンを浮かせる。新規issueの知らせと同じく`surface-raised`の地に`outline`の枠を付け、色は付けない。高さは44px以上にする。押すと新しいカードへスクロールして消える。閉じるボタンは置かない
- 本文とコメントの書式：Markdownの見出しは`body-md`の太字にし、本文より大きくしない。見出しは2段下げて描く。h1はh3、h2はh4、h3はh5、h4以下はh6にする。ページの見出しの階層を乱さないためである。コードは`mono-sm`、表と引用は`outline`の線で区切る
- 理由のバッジ：`question`、`verify`、`plan-questions`、`plan`は`badge-attention`、`error`、`permission`、`push`、`ci`は`badge-error`
- 計画の待ちの案内：理由が`plan`のとき、返答の欄の上に、承認の語（`/ok`など）の使い方を`label-md`の`muted`で1行出す。理由が`plan-questions`のときは、選んで「この内容で返答」を押す答え方を出し、承認の語には触れない
- 実行の失敗の待ちの案内：理由が`error`のとき、返答の欄の上に、「再実行する」と「返答を送る」の使い分けを`label-md`の`muted`で1行出す。返答の欄が空の間は「再実行する」を`button-primary`、「返答を送る」を`button-secondary`にして、塗りのボタンを1つに保つ。欄に何か書くと「再実行する」は押せなくなる。「再実行する」は、再実行を頼む決まった文を返答として送る。ほかの失敗の理由（`verify`、`permission`、`push`、`ci`）には出さない
- 計画の質問のカード：最新の質問のコメントは、問いごとに、題（`body-md`の太字）、本文、選択肢の組、補足の欄、推奨の順に描く。題はh3の見出しにする。組は見出し（`label-md`の`muted`）の下に、ラジオボタンを1行に1つ並べ、最後に「選ばない（補足で答える）」を`muted`で置く。行の高さは44px以上にする。ラジオボタンの色は`primary`にする。推奨に当たる選択肢には、文の後ろに「（推奨）」を`label-md`の`muted`で付け、色は付けない。補足の欄は返答の欄と同じ見た目で、高さを抑える
- 送る文の表示：返答の欄の下、ボタンの上に、`surface`の地と`outline`の枠で、送る文をそのまま出す。問いを読み取れたときは「この内容で返答」を`button-primary`、「返答を送る」を`button-secondary`にして、塗りのボタンを1つに保つ
- 設定の画面：ヘッダの右の「設定」のリンクから開く。上にプロジェクトの節、その下に計画の指示の節と実装の指示の節を、この順に置く。塗りのボタンは置かない
- プロジェクトの節：この端末のプロジェクトを1つずつカードにし、1行目に参照（`mono-sm`）とフォージのバッジを出す。その下に、クローンの場所と計画から始めるかどうか、クローンの有無、足りない状態ラベルを`label-md`の`muted`で出す。クローンが無いことは誤りの色にしない。追加の直後はエージェントがまだ作っていないだけだからである。追加の欄は1枚のカードにし、参照の1行の入力欄、計画から始めるかどうかのチェックボックス（初期値はオン）、`button-secondary`の追加のボタンを置く。追加できたことは`toast-success`のトーストで知らせる。webhookの登録コマンドは、コピーして使うので、一覧の下に`mono-sm`で残す。拒まれた理由は、その欄の中に`banner-error`で出す。追加が無効なときは、欄を出さずに、有効にする方法を1行で示す
- 新規issue：一覧のペインの先頭の右に、`button-secondary`の「新規issue」を置く。押すとネイティブの`<dialog>`を開く。ダイアログは`surface`の地に`outline`の枠にする。幅は720pxまでとし、スマホの幅では画面の端から`lg`だけ空ける。項目は上から、プロジェクトの選択、タイトルの1行の入力欄（必須）、本文の欄（Markdown、任意）、開始ラベルのラジオボタンの順に並べる。プロジェクトの各選択肢は`mono-sm`にする。形は「o/r GitHub（担当：端末）」で、プレフィックスを外した参照とフォージの名前を文字で並べる。開始ラベルは1行に1つ、高さ44px以上で並べ、初期値はreadyにする。各行の後ろに、ラベル名を`mono-sm`で出す。下の右に`button-secondary`の「やめる」と`button-primary`の「作成する」を置く。タイトルが空なら作成のボタンを押せなくする。誤りはダイアログの中に`banner-error`で出し、入力は残す。作れたらダイアログを閉じ、`toast-success`のトーストを出す。トーストには、作ったissueの参照（`mono-sm`）とフォージのバッジ、フォージへのリンク（新しいタブ）を置く。一覧の上には何も残さない
- マージ：完了のissueに、`button-secondary`の「マージ」を置く。置き場所は2つある。一覧の完了の行では、PR/MRのリンクの隣に、行のリンクの外に置く。1件の表示では、状態の行の下に、PR/MRのリンクと並べて置く。マーカーにPR/MRの番号とコミットが無い古い完了には出さない。押すとネイティブの`<dialog>`を開く。見た目は新規issueのダイアログと同じにする。ダイアログには、見出し（`PR #Nをマージしますか`、GitLabでは`MR !N`）、取り消せないことの1文、PR/MRへのリンク（`mono-sm`、新しいタブ）を置く。下の右に`button-secondary`の「やめる」と`button-primary`の「マージする」を置く。ダイアログを開くと「やめる」にフォーカスを当てる。取り消せない操作なので、安全な側から始めるためである。送っている間は、どちらも押せなくし、スピナーと送信中の文言を出す。結果が返ったらダイアログを閉じる。成功は`toast-success`、断られたときは`toast-error`のトーストで、理由を2行目に出す。ほかのダイアログと違い、誤りをダイアログの中に残さない。入力が無く、直して送り直すものが無いからである
- 指示の節：計画の指示と実装の指示で、節を分ける。節ごとに、見出し、渡す先の説明、ファイルのある端末を出す。指示の欄を1つずつカードにし、プロジェクトの欄の名前は参照とフォージのバッジにする。欄ごとに`button-secondary`の保存のボタンを置く。保存できたことは、`toast-success`のトーストで知らせる。トーストの文には、欄の名前と指示の種類を入れる。誤りは、その欄の中に`banner-error`で出す
- トースト：知らせを、画面の右下へ一時的に出す。種類は4つある。通常は`toast-info`（`surface-raised`の地と`outline`の枠）、成功は`toast-success`にする。警告は`toast-warning`（`attention`の色）、誤りは`toast-error`にする。影は付けない。種類の印（文字と同じ色の線のアイコン）を左端、✗のボタン（「閉じる」、44px四方）を右端に置く。既定は5秒で消え、呼び出す側が時間を変えられる。マウスを載せている間と、フォーカスがトーストの中にある間は、時間を止める。複数あるときは重ねずに縦へ並べ、新しいものを最も下にする。同時に出すのは5個までで、6個目を出すと古いものから消す。幅は360pxで、画面の端から`lg`だけ空ける。幅1024px未満では、左右`lg`の余白を空けて、下の端の全幅で出す。誤りのトーストは`role="alert"`、ほかは`role="status"`で読み上げる。欄ごとの誤り、ダイアログの中の誤り、画面の上の誤りのバナーは、トーストにしない。直すまで見えている必要があるからである
- 1行の入力欄：返答の欄と同じ地、枠、フォーカスの輪郭で、高さは44pxにする。選択欄も同じ枠にする
- 返答の欄：`surface-raised`の地に`field-outline`の枠。入力欄の境目は地に対して3:1以上にする必要があり、`outline`では足りないためである。フォーカスでは`primary`の2pxの輪郭を付ける。空のときは送るボタンを押せなくする。欄の上の案内は、欄の説明として読み上げさせる。Ctrl+Enter（macOSでは⌘+Enter）で「返答を送る」と同じく欄の文を送る。欄の外で`r`を押すと、欄へフォーカスを移す。書きかけの文と選んだ答えは、別のissueを開いて戻っても残す。ブラウザのストレージには書かず、再読み込みで消える
- 送るボタン：`button-primary`。送っている間は押せなくし、ボタンの文言を送信中の表示に替える。送れたら、フォームの代わりに「送りました」の行を出し、その下に「次の待ちへ」を`button-secondary`で出して、フォーカスをそこへ移す。行き先は、一覧の「返答待ち」で今のissueの1つ下の待ちで、末尾から先頭に戻る。このタブで送ったばかりでまだ同じ待ちに残るissueは飛ばす。ボタンは行き先の題を読み上げさせる。行き先が無いときは、ボタンの代わりに「他に待ちはありません」を`muted`で出し、フォーカスは「送りました」の行へ移す
- 読み込み中の表示：まだ届いていない一覧、1件、設定の各節、新規issueのプロジェクトの代わりに、スピナーと「読み込んでいます」を`muted`で出す
- タブのタイトル：画面の名前（1件ではissueのタイトル、設定では「設定」）と「SwitchBoard Inbox」をつなぐ。待ちが1件以上あれば、先頭に「(N) 」と件数を付ける
- 一覧の節：待ち、実行中、順番待ち、完了の各節に見出しを付ける。待ちの節は「返答待ち」とし、「順番待ち」と取り違えないようにする
- 完了のPR/MRへのリンク：番号が分かれば「PR #N」（GitLabでは「MR !N」）と書き、URLは出さない
- 誤りのバナー：`banner-error`。届かなかったフォージ、受信箱のサービスに届かないこと、待ちでなくなったことを、画面の上に出す。届かなかったフォージと受信箱のサービスに届かないことは、ヘッダの下に、両方のペインにまたがる全幅で出す。左右には`lg`の余白を空ける。出すのは一覧と1件の画面だけで、設定の画面は各節の中に誤りを出す
- ページがありませんの画面：どのルートにも当たらないURLで出す。ヘッダの下の720pxの列に、見出し「ページがありません」、短い説明（`muted`）、一覧へのリンク「一覧へ戻る」を置く。タブのタイトルは「ページがありません」と「SwitchBoard Inbox」をつなぐ

## Do's and Don'ts

- Do: 色の付いた要素は、待ちの理由、誤り、トーストの種類、フォージのバッジだけにする
- Do: issueから来た文字列は、生のHTMLとして解釈させない。Markdownとして描くときも、HTMLのタグは文字のまま出し、画像は読み込まずにリンクとして出す。issueのテンプレートに多いHTMLのコメントは出さない
- Don't: 本文とラベルを、地に対して4.5:1未満のコントラストにしない
- Don't: 塗りのボタンを、1画面に2つ置かない
- Don't: 幅の狭い画面で、横へのスクロールを出さない
