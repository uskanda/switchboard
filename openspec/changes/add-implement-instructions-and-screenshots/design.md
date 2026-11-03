# Design

## Context

指示は`internal/instructions`の`Kind`（`plan`・`run`）で分かれている。保存の検査、監査ログ、CLI、受信箱のAPI、設定の画面は、どれも種類ごとに同じ作りである。`internal/agent/run.go`の`work`は、計画の実行に`plan.md`を、それ以外の実行に`run.md`を足す。実行の種類は`plan`の`kind`で分かれる。直接の実装、承認のあとの実装（`approvedRun`）、計画（`planRun`）、直す実行（`fixRun`）がある。

書き戻し（`writeOut`）は、同じrunと同じ状態のマーカーを持つ投稿があれば投稿しない。これで、やり直しても同じ投稿が2件にならない。マーカーの状態のうち`Transient()`が真のもの（claim、進捗、順番待ち）は、待ちやdoneの判定で読み飛ばされる。

動機はproposal.mdのWhyを参照する。

## Goals / Non-Goals

**Goals:**
- 指示の種類を1つ足すだけで、検査、記録、画面の道筋をそのまま使う
- 画像の置き場所の取り決めと添付は、利用者の文面に頼らずswitchboardのコードで行う
- 添付の失敗が、doneの書き戻しとpushを止めない

**Non-Goals:**
- 撮影そのもの（Playwrightやエミュレーターの起動）。これは利用者の`implement.md`と`allow_tools_extra`に任せる
- PR/MRへの画像の添付、直す実行だけの指示の種類
- GHESでの添付（`gh --attach`が対応しない）

## Decisions

### D1. 取り決めの節は`prompt.WithScreenshots`で足す
`System`は直す実行の`FixSystem`からも呼ばれる。ここに節を入れると、直す実行にも入ってしまう。そこで`work`が、直接の実装と承認のあとの実装でだけ`prompt.WithScreenshots`を呼ぶ。system promptの規則はほかの節と同じく英語で書き、出力の言語だけを設定に合わせる。

### D2. 添付のコメントのマーカーは状態`attachment`とし、`Transient()`に含める
`Transient()`の使い手は3つある。`hasReply`、CIの見張りの再開、受信箱の`latestPost`である。どれも「issueがなぜ待つか、runがどう終わったか」を探している。画像のコメントはどちらも語らないので、読み飛ばされるのが正しい。別の判定を足すより、使い手を1つも変えずに済む。

### D3. 添付は`forge.Attacher`の型アサーションで選ぶ
`forge.Forge`に足すと、テストの偽物をすべて直すことになる。`forge.Attacher`を別のインターフェースにする。`forgeOf(ref)`の返すものがこれを満たすときだけ添付する。満たさなければ、「このフォージには添付できない」を理由として書く。

### D4. GitHubは`gh`を`internal/forge/github`の中で呼ぶ
引数は`gh issue comment <N> -R owner/repo --body-file - --attach <file>...`とする。環境は次のものだけにする。
- `GH_TOKEN`：クライアントのトークン
- `GH_CONFIG_DIR`と`HOME`：空の一時ディレクトリ
- `PATH`、`GH_PROMPT_DISABLED=1`、`GH_NO_UPDATE_NOTIFIER=1`

コメントのIDは、`gh`が標準出力に返すコメントのURL（`#issuecomment-<id>`）から読む。RESTで読み直す案より呼び出しが1回で済み、別の投稿と取り違えない。URLが読めないときはIDを0とし、添付は成功として扱う。

マーカーは本文の最後の行でないと読まれない。`gh`が画像を本文のどこに入れるかは確かめていない（V46）。そこで、作られたコメントをRESTで読み直す。最後の行がマーカーでなければ、マーカーを末尾に移すよう編集する。

### D5. GitLabはuploads APIを`internal/forge/gitlab`に足す
`POST /projects/:id/uploads`に、画像を1枚ずつmultipartで上げる。返った`markdown`を本文のあとに並べ、ノートを1件作る。

### D6. 置き場所のexcludeは`git rev-parse --git-path info/exclude`で決める
worktreeでは`.git`がファイルなので、パスを自分で組み立てず、gitから得る。共通のディレクトリの`info/exclude`は、同じクローンのほかのworktreeでも読まれる。置き場所の名前はswitchboardの取り決めなので、差し支えない。

### D7. 画像はpushのあとでrunのディレクトリへ移す
`finish`は、pushとPR/MRを済ませてから、`internal/workspace`の`CollectScreenshots`で画像を`runs/<run>/screenshots/`へ移す。控えた名前と理由は`outcome`に入れ、`outcome.json`に残す。書き戻しのやり直しと、再起動のあとの回復は、`outcome.json`とrunのディレクトリから読む。

### D8. 画像のコメントは、マーカーのrunと状態で二重の投稿を防ぐ
`writeOut`は、doneの投稿とラベルの付け替えのあとで添付する。同じrunで状態`attachment`の投稿があれば、何もしない。添付に失敗したら、理由だけのコメントを同じマーカーで投稿する。このため、やり直しても2件目は付かない。理由のコメントの投稿まで失敗したら、書き戻し全体の失敗とし、次のpollでやり直す。

### D9. コミットされた画像は`CheckOutput`で拒否する
差分の名前に置き場所の下のものがあれば、問題として返す。`finish`はこれをverifyの待ちにする。保護するパスの一覧に入れる案もある。ただ、「保護されたパスに触れた」という文面では、何を直せばよいかが分からない。

## Risks / Trade-offs

- 【`gh`の`--attach`は非公開のアップロードの仕組みに頼る】→`internal/forge/github`の1つのファイルに閉じ込め、失敗は理由のコメントで示す。`switchboard doctor`で版を確かめる
- 【エージェントのfine-grained PATで`--attach`が通るかは未確認（V46）】→タグ付きのテスト`-tags realgh`で再現できるようにする。結果はROADMAP.mdの検証項目に書く。通らなくてもdoneは保たれ、理由のコメントが付く
- 【GitLabのuploadsのmarkdownが画像として表示されるかは未確認（V47）】→同じく検証項目に残す
- 【画像は1枚5MB、10枚までissueに残る】→上限で抑える。コミットはしないので、リポジトリは大きくならない

## Migration Plan

設定の変更は無い。`implement.md`が無く、置き場所に画像が無ければ、今までと同じ動きになる。戻すときは、前のバージョンのバイナリに戻せばよい。残った`implement.md`と`info/exclude`の行は、前のバージョンでは使われない。

## Open Questions

- V46・V47の結果。どちらも、結果によって仕様は変わらない（失敗は理由のコメントになる）
