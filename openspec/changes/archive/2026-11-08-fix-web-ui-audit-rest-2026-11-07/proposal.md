# Proposal

## Why

2026-11-07のWeb受信箱の監査で見つかった課題のうち、`fix-web-ui-audit-2026-11-07` で直さなかったものを、issue #171〜#185に起こした。そのうち、決める論点が無いか、実装の後に確かめれば足りる小さな修正を、この変更でまとめて直す。通知から開いた1件は、ログインを挟むと失われる。作成中のダイアログをEscapeで閉じると、作成の失敗が見えなくなる。どちらも利用者の操作を無駄にする。

## What Changes

- ログインの画面から、開こうとしたページへ戻す（#171）。`/auth/login` は、この受信箱のパスだけをcookieに覚え、`/auth/callback` はセッションを作った後にそこへ戻す
- 1件を開いたら先頭から表示し、幅1024px未満で一覧へ戻ったら、一覧を離れたときのスクロールの位置に戻す（#172）
- 新規issueのダイアログは、作成の応答を待つあいだEscapeで閉じない（#173）
- このタブで作ったばかりのissueを開いたとき、一覧から消えたとは言わず、作ったばかりであることを伝える（#174）
- 設定の画面は、読み直しが成功したら前の誤りを消す。管理者でない利用者には、設定を変えられるのは管理者だけだと伝え、何も読まない（#175）
- ログアウトとセッション切れのときに、「処理中」の印を消す（#176）
- 自動マージの結果は、他のタブが伝えたものを伝え直さない（#177）
- 理由のバッジに、理由の語に続けて日本語の短い名前を出す（#178）
- 幅640〜1024pxの表の列の幅を固定し、節ごとの列の位置をそろえる（#179）
- 計画の質問の補足の欄を、選択肢を選んでいるあいだは畳む（#180）

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`：スクロールの位置、作成中のダイアログ、作ったばかりのissue、設定の画面の誤りと権限、自動マージの結果のタブの間の重複、理由のバッジの名前、表の列、補足の欄の要件を足す
- `web-auth`：ログインの後の戻り先と、ログアウトのときの印の要件を足す

## Impact

- `web/internal/server/auth.go`
- `web/ui/src/pages/LoginPage.tsx`、`InboxLayout.tsx`、`SettingsPage.tsx`
- `web/ui/src/components/IssuePane.tsx`、`NewIssue.tsx`、`ProjectsSection.tsx`
- `web/ui/src/components/ReasonBadge.tsx`、`InboxList.tsx`、`PlanQuestions.tsx`
- `web/ui/src/lib/auth.tsx`、`autoMergeResults.ts`、新しい `lib/listScroll.ts`
- `DESIGN.md`
- 受信箱のサービスとAPIは変えない
