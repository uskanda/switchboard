## ADDED Requirements

### Requirement: 質問の画像の取得
受信箱のAPIは、issueと画像のURLを受け取り、その画像を返す口を持たなければならない（MUST）。
返してよいのは、そのissueがGitLabのプロジェクトのもので、URLがそのフォージのホストの、そのプロジェクトのアップロード（`/uploads/` の下）を指すときだけとする。ほかのURLには、404を返さなければならない（MUST）。
受信箱のAPIは、画像を受信箱のトークンで取り、PNG、JPEG、GIF、WebPのどれかで、5MB以下であることを確かめてから返さなければならない（MUST）。確かめられないものは返さない。

#### Scenario: プロジェクトのアップロード
- **WHEN** `gl:team/app` #12と、`https://gitlab.example.com/team/app/uploads/abc/shot.png` を渡して画像を求める
- **THEN** 受信箱のトークンで取ったPNGが返る

#### Scenario: 別のホストの画像
- **WHEN** `gl:team/app` #12と、`https://evil.example.net/x.png` を渡して画像を求める
- **THEN** 404が返り、そのホストへの要求は出ない
