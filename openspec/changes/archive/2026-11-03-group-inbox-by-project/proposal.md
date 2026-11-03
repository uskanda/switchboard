# Proposal

## Why

WUIの一覧は、節ごとに古い順で行を並べるだけなので、プロジェクトが増えると、どのプロジェクトに待ちが溜まっているかを行を1つずつ読まないと分からない（issue #104）。各節の中をプロジェクトでまとめれば、見出しと件数だけで見渡せる。

## What Changes

- 一覧の4つの節（返答待ち、実行中、順番待ち、完了）の中を、プロジェクトごとのサブグループにまとめる。サブグループには、参照、フォージのバッジ、件数の見出しを付ける
- サブグループどうしは、各サブグループで最も古い行の順に並べる。中の行は今のまま古い順にする
- 一覧のペインの先頭、「新規issue」の左に、トグルボタン「プロジェクトでまとめる」を置く。初めはオンで、切り替えた状態はブラウザに覚えさせる
- 受信箱のサービスの並べ方、`/api/inbox`の形、CLI `switchboard inbox`の表示と行番号は変えない

## Capabilities

### New Capabilities

新しい能力は無い。

### Modified Capabilities

- `web`: 一覧の並べ方の要件を、まとめるときとまとめないときに分けて書き直す。要件を2つ足す。プロジェクトのサブグループと、まとめる表示の切り替えである

## Impact

- `web/ui/src/components/InboxList.tsx`、`web/ui/src/components/NewIssue.tsx`
- 新しい`web/ui/src/lib/group.ts`と`web/ui/src/lib/groupPref.ts`、それぞれの単体テスト
- 新しい画面のテスト`web/ui/src/pages/grouping.test.tsx`
- `DESIGN.md`の「一覧の節」と「新規issue」の項
- Goのコード、API、CLIは変わらない
