# Proposal

## Why

WUIは参照を`gh:o/r#12`の形で出している。プレフィックスの「gh」「gl」は略語で、一目ではフォージを読み取りにくい。issue #70で、参照の後ろにフォージの名前をラベルの形で出すよう求められた。

## What Changes

- WUIの参照から`gh:`と`gl:`のプレフィックスを外し、参照のすぐ後ろにフォージのバッジ（「GitHub」「GitLab」）を置く。対象は、一覧の表とカード、1件の表示の見出し、プロジェクトの節のカードと追加のトースト、新規issueの作成のトースト、指示の欄の名前と保存のトースト
- バッジの色はフォージごとに分ける。GitHubは青みの灰色、GitLabは紫にする。明るいテーマと暗いテーマの両方に色を足す
- 新規issueのプロジェクトの選択肢はバッジを置けないため、「o/r GitHub（担当：desk）」のように文字で書く
- CLIの出力、入力の形式（`gh:owner/repo`）、URL、APIの`ref`は変えない

## Capabilities

### New Capabilities

### Modified Capabilities
- `web`: 参照の表示の形を足す。プロジェクトの追加と指示の保存のトーストの文を、新しい参照の形に直す

## Impact

- `web/ui/src/lib/ref.ts`（新規）、`web/ui/src/components/IssueRef.tsx`（新規）、`web/ui/src/components/ui/badge.tsx`
- `InboxList.tsx`、`IssuePane.tsx`、`NewIssue.tsx`、`ProjectsSection.tsx`、`pages/SettingsPage.tsx`
- `DESIGN.md`のcolorsとcomponents、`web/ui/src/tokens.css`、`web/ui/src/index.css`
- APIとCLIには影響しない
