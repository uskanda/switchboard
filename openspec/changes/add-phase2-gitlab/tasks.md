# Tasks

## 1. GitLabの挙動の確認

- [x] 1.1 glabで `gl.uskan.dev` に検証用の非公開プロジェクト `nas/switchboard-sandbox` を作る。そこでissueの本文を編集し、本文の変更を示すシステムノートの文言と作成者を確かめる。ノートを編集し、GraphQLの `lastEditedBy` が編集者を返すことも確かめる。結果をROADMAP.mdの検証項目に記録する
- [x] 1.2 同じプロジェクトで、次の3つを確かめ、ROADMAP.mdの検証項目に記録する。ラベルを付けた人が `resource_label_events` の `user` に入ること、`updated_after` で更新を取れること、`Draft: ` を付けたMRを作れること

## 2. フォージの種別（`forge`）

- [x] 2.1 `forge.IssueRef` に `Forge` を足し、`String()` を `gh:owner/repo#12` の形にする。GitHubと偽物の実装と呼び出し側を追従させ、`make verify` が通ることを確かめる
- [x] 2.2 偽物を、GitLabのプロジェクト（サブグループを含むパス、`internal` の公開範囲、本文の編集者が分からない状態）も表せるように広げ、それを使うテストを1本通す

## 3. 設定（`config`）

- [x] 3.1 `gl:group/project` と `gl:group/sub/project` の受け付け、GitLabの設定が無いときの報告を、失敗するテストから実装する。`gl:` を拒否する要件のテストを外し、specのシナリオに対応するテストが通ることを確かめる
- [x] 3.2 GitLabの設定と、`trusted_authors` の `gitlab` を実装する。GitLabの設定のキーは `base_url`（既定値は `https://gitlab.com`）、`token_ref`、`inbox_token_ref`、`agent_identity` である。GitLabの設定で `search_scope` を拒否することもテストで確かめる
- [x] 3.3 プロジェクトごとの `token_ref` と `agent_identity` の上書きを実装する。片方だけの上書きを設定の誤りにすることを、テストで確かめる
- [x] 3.4 `config/switchboard.example.yaml` に、GitLabの設定と、上書きの例をコメントで足し、例が検証を通ることをテストで確かめる

## 4. GitLabのクライアント（`internal/forge/gitlab`）

- [x] 4.1 プロジェクトIDと公開範囲の解決、ユーザーIDの解決、更新されたissueの取得を実装する。HTTPのやり取りを記録した入力でテストが通ることを確かめる
- [x] 4.2 issueとノートの取得を実装する。本文の編集者はシステムノートから取り、取れなければ `Unknown` にする。ノートの編集者は `lastEditedBy` から取る。システムノートをコメントとして返さないことも、テストで確かめる
- [x] 4.3 ラベルを付けた人の取得、ラベルの作成と全置換、ノートの投稿、draftのMRの作成を実装し、テストで確かめる
- [x] 4.4 エラーの報告にトークンが出ないことと、`PRIVATE-TOKEN` ヘッダーで認証することを、テストで確かめる

## 5. 信頼判定（`trust`）

- [x] 5.1 ユーザーIDの解決と記録をフォージごとにする。Phase 1の記録を `gh:` として読み込めることと、両フォージで同じログイン名を別のIDで判定することを、テストで確かめる
- [x] 5.2 エージェントのIDをプロジェクトごとに持たせ、「プロジェクトごとのbot」のシナリオをテストで確かめる
- [x] 5.3 編集者が分からない本文とコメントの扱い、`internal` を公開として数えることを、specのシナリオに対応するテストで確かめる

## 6. エージェント（`agent`、`workspace`）

- [x] 6.1 エージェントが、プロジェクトの `ref` からフォージのクライアント、pushのトークン、エージェントのIDを引くようにする。GitHubとGitLabを混ぜた設定で、両方のプロジェクトをポーリングすることを偽物でテストする
- [x] 6.2 GitLabのプロジェクトで、ポーリングのたびに待ちのissueを一覧し、`user_notes_count` が変わったissueを評価する。issueの更新時刻が変わらない返答で再開することを、偽のGitLabでテストする
- [x] 6.3 GitLabのプロジェクトで完了したら、draftのMRを作るようにする。保護パスに `.gitlab-ci.yml` を足し、specのシナリオをテストで確かめる
- [x] 6.4 GitLabのhttpsのリモートへのpushで、`GIT_ASKPASS` のユーザー名を `oauth2` にする。ローカルのbareリポジトリへのpushのテストで、渡すユーザー名を確かめる
- [x] 6.5 閉じられたissueのworktreeとローカルのブランチの片付けを実装する。コミットしていない変更があるときに残すことと、実行中のissueを片付けないことを、実物のgitリポジトリでテストする

## 7. 受信箱（`inbox`）

- [x] 7.1 受信箱が、フォージごとの `inbox_token_ref` のクライアントから待ちを集めるようにする。一覧は1つにまとめ、待ち始めた時刻の古い順で並べる。GitLabは設定済みのプロジェクトごとに取得する。両フォージのシナリオをテストで確かめる
- [x] 7.2 `--forge` での絞り込みと、届かないフォージの報告（届いた側の一覧、標準エラーの警告、0以外の終了コード）を実装し、テストで確かめる
- [x] 7.3 `show` と `reply` が、`gl:group/project#n` の形と、`gl:` の行番号の記録を扱えるようにし、GitLabのissueへの返答をテストで確かめる

## 8. コマンドの組み立てと診断

- [x] 8.1 `cmd/switchboard` で、設定にあるフォージのクライアントだけを組み立てる。GitHubだけの設定でGitLabのトークンを求めないことを、テストで確かめる
- [x] 8.2 `doctor` が、GitLabのプロジェクトのトークン、ユーザーID、ローカルのリポジトリ、公開範囲、GitLabの版、上書きのトークンとbotのIDを調べるようにし、テストで確かめる
- [x] 8.3 `labels sync` が、GitLabのプロジェクトにも状態ラベルを作ることを、テストで確かめる

## 9. 文書

- [x] 9.1 DECISIONS.mdに、ADR-030（GitLabの扱い）とADR-031（エージェントの身元の上書き）を足し、ADR-017の論点を「Phase 4の中継で決める」に書き直す。ROADMAP.mdのV16をPhase 4の検証項目に移す
- [x] 9.2 README.mdに、GitLabのトークンの登録、設定、プロジェクトごとの上書きの使い方を書く。書いたコマンドがそのまま動くことを確かめる

## 10. 結合の確認

- [x] 10.1 偽のGitHubと偽のGitLab、実物のgitリポジトリ、偽の `claude` を使う結合テストを書く。両フォージの待ちが1つの受信箱に出て、両方に返答し、PRとMRができるまでを1本で通す。CIで通ることを確かめる
- [ ] 10.2 `uskanda` のPATを、Secret Serviceの `switchboard/gitlab` と `switchboard/gitlab-inbox` に登録してもらう。`nas/switchboard-sandbox` と、GitHubの `uskanda/switchboard-sandbox` に1つずつissueを立てる。実物の `claude` で、1つの受信箱に両方の待ちが出るところから、両方に返答してMRとPRができるまでを通す。結果をROADMAP.mdに記録する
- [ ] 10.3 `nas/switchboard-sandbox` にProject Access Tokenを作る。プロジェクトの `token_ref` と `agent_identity` を、そのbotで上書きする。botとして質問を投稿し、返答を受けてMRを作るまでを実物で通す。結果をROADMAP.mdに記録する
