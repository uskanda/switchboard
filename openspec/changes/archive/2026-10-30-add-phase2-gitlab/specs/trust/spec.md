## MODIFIED Requirements

### Requirement: ユーザーIDによる判定
`switchboard` は、`trusted_authors` と `agent_identity` をログイン名からユーザーIDに解決し、IDで判定しなければならない（MUST）。
解決は、フォージごとに、そのフォージの `trusted_authors` と、そのフォージのプロジェクトが使う `agent_identity` について行う。
あるフォージで解決したIDは、別のフォージの判定に使ってはならない（MUST NOT）。
解決したIDは、フォージを区別して状態ディレクトリに記録しなければならない（MUST）。
記録したIDと、同じフォージの同じログイン名を新たに解決したIDが異なるとき、`switchboard agent run` は起動してはならない（MUST NOT）。

#### Scenario: ログイン名の持ち主が変わった
- **WHEN** `trusted_authors` のログイン名が、記録時とは別のユーザーIDに解決される
- **THEN** `switchboard agent run` は0以外の終了コードで終わり、IDが変わったログイン名を報告する

#### Scenario: 両フォージで同じログイン名
- **WHEN** GitHubとGitLabの `trusted_authors` に同じログイン名 `uskanda` がある。IDは、GitHubでは13825、GitLabでは1である
- **THEN** GitHubのissueはID 13825で、GitLabのissueはID 1で判定され、どちらの記録も残る

### Requirement: エージェントの投稿の識別
エージェントは、次の2つを満たすコメントだけを、エージェント自身の投稿として扱わなければならない（MUST）。作成者が、そのプロジェクトの `agent_identity` のユーザーIDと一致する。末尾の単独の行にマーカーを持つ。
引用ブロックの中のマーカーは、エージェント自身の投稿の印として扱ってはならない（MUST NOT）。

#### Scenario: 人間が引用したマーカー
- **WHEN** 信頼済みのユーザーが、エージェントのコメントを引用ブロックで引用して返答する
- **THEN** その返答は人間の指示として扱われ、エージェント自身の投稿としては扱われない

#### Scenario: プロジェクトごとのbot
- **WHEN** プロジェクトAの `agent_identity` がbotのユーザーで、プロジェクトBはフォージの `agent_identity` を使う
- **THEN** エージェント自身の投稿として扱われるのは、プロジェクトAではbotのマーカー付きのコメントだけである。プロジェクトBでは、フォージの `agent_identity` のマーカー付きのコメントだけである

## ADDED Requirements

### Requirement: 編集者が分からないとき
issue本文が編集されていて、最後の編集者をフォージから取れないとき、エージェントは、信頼済みでないユーザーが編集したものとして扱い、実行を始めてはならない（MUST NOT）。
コメントが編集されていて、最後の編集者をフォージから取れないとき、エージェントは、そのコメントを指示として採用してはならない（MUST NOT）。
どちらの場合も、エージェントは、編集者が分からないことを理由として監査ログに記録しなければならない（MUST）。

#### Scenario: 本文の編集を確かめきれない
- **WHEN** GitLabのissueのノートが取得の範囲を超えていて、本文の変更を示すシステムノートの有無を確かめきれない
- **THEN** 実行は始まらず、編集者が分からないという理由が監査ログに記録される

### Requirement: 公開範囲の数え方
`switchboard` は、GitLabの `internal` のプロジェクトを、公開のプロジェクトとして数えなければならない（MUST）。この数え方は、`include_untrusted` の判定と `doctor` の報告の両方に使う。
非公開として数えるのは、GitHubの `private` のリポジトリと、GitLabの `private` のプロジェクトだけとする。

#### Scenario: internalのプロジェクト
- **WHEN** GitLabの `internal` のプロジェクトに `include_untrusted: true` が設定されている
- **THEN** そのプロジェクトのissueは実行されず、設定の誤りが報告される
