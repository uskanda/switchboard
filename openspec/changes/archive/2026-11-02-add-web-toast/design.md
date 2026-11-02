# Design

## Context

動機はproposal.mdの「Why」を参照。今のコードで、この設計を縛る点は次のとおり。

- WUIの部品はshadcn/uiの形（`components/ui`）で作る。色や余白は、DESIGN.mdから書き出した`tokens.css`のTailwindのクラスだけで付ける
- 新規issueの作成の知らせは`NewIssue`の中で、設定の画面の成功の知らせは各欄の中で、状態として持っている
- 外部の仕様に依存する部分は薄い層に隔離する（AGENTS.md）

## Goals / Non-Goals

**Goals:**

- どの画面からも1行で呼べる、汎用のトーストを作る
- sonnerを使うファイルを2つだけにする
- 時間で消える知らせを、読む前やリンクを押す前に消さない（WCAG 2.2.1）

**Non-Goals:**

- 誤りをトーストで出すこと（欄ごとの誤り、ダイアログの中の誤り、画面の上の誤りのバナーは変えない）
- 通知の履歴

## Decisions

### D1. sonnerを見た目なしで使い、見た目はDESIGN.mdのトークンで付ける

sonnerは、✗のボタン、種類、位置、縦に広げた積み方を標準で持つ（Q7）。`toastOptions`の`unstyled`でsonnerの見た目を外す。`classNames`には、種類ごとのクラスとトークンのTailwindのクラスを当てる。種類ごとのクラスは`toast-info`、`toast-success`、`toast-warning`、`toast-error`である。積み方と位置の計算はsonnerのものを使う。

### D2. sonnerをimportするのは`lib/toast.ts`と`components/ui/toaster.tsx`だけ

呼び出す側は`toast.success(message, { description, action, duration })`のように、`lib/toast.ts`の関数だけを使う。ライブラリを替えるときに直す範囲を、この2つに閉じる。

### D3. 表示時間のタイマーは`lib/toast.ts`が持つ

sonnerは、マウスを載せている間はタイマーを止めるが、フォーカスがトーストの中にある間は止めない。そこでsonnerには自動で消えないトースト（`duration: Infinity`）を渡し、残り時間は`lib/toast.ts`が数える。`Toaster`を包む要素は、マウスとフォーカスの出入りを`setHovered`と`setFocused`で知らせる。どちらかの間は、全部のトーストのタイマーを止める。時間が来たら`dismiss`で閉じる。

トーストがすべて消えると、マウスやフォーカスが外れたという知らせは届かないことがある。そのときは止めた状態を戻し、次のトーストのタイマーを動かす。

### D4. 5個を超えたら古いものを閉じる

sonnerの`visibleToasts`は、超えた分を見えなくするだけで、DOMとタブの順には残す。`lib/toast.ts`は出した順を覚え、6個目を出したときに最も古いものを`dismiss`する。`visibleToasts`も5にしておく。

### D5. 読み上げの役割は、トーストの本文に付ける

sonnerはトーストの`li`に役割を付けない（一覧全体が`aria-live="polite"`）。`lib/toast.ts`は本文を包む要素に、誤りでは`role="alert"`、それ以外では`role="status"`を付ける。説明の文とリンクも、その要素の中に置く。

### D6. 位置と幅

右下に出し、画面の端から`lg`（16px）だけ空ける。幅は360pxとする。sonnerが狭い画面に切り替える幅は600pxである。幅1024px未満の全幅は、`Toaster`のクラス（`max-lg:inset-x-lg!`、`max-lg:w-auto!`）と、トーストの`w-full!`で上書きする。

## Risks / Trade-offs

- sonnerは自前のCSSを`head`に足し、見た目なしでも位置と動きのCSSは残る → 見た目のCSSは`data-styled=true`の要素だけを選ぶ。DESIGN.mdの見た目とはぶつからない
- sonnerの内部の振る舞い（クラスの付け方、`onDismiss`の呼び方）が版によって変わる → `toast.test.tsx`で、種類のクラス、役割、✗のボタン、時間、止まり方、個数を確かめる
