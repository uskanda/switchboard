# Tasks

## 1. 実装の実行だけの指示

- [ ] 1.1 `internal/instructions`に種類`implement`を足す。`ParseKind("implement")`と`implement.md`のパスのテストが通ることで確かめる
- [ ] 1.2 `work`で、直接の実装と承認のあとの実装にだけ`implement.md`の指示を`run.md`のあとに足し、`run.started`に`implement`の鍵でハッシュを残す。直す実行と計画の実行に入らないことを含め、`internal/agent`のテストで確かめる
- [ ] 1.3 `switchboard instructions edit --implement`を足し、`--run`との同時の指定を誤りにする。`cmd/switchboard`のテストで確かめる
- [ ] 1.4 `inboxserve`の指示の`kind`に`implement`を足し、読み出しで返す。`inboxserve`のテストで確かめる
- [ ] 1.5 `web/ui`の設定の画面に実装の実行だけの節を足し、DESIGN.mdの指示の節を合わせる。`web/ui`のテストで確かめる

## 2. 画像の置き場所

- [ ] 2.1 <prompt.WithScreenshots>として取り決めの節を足し、直接の実装と承認のあとの実装にだけ入れる。`internal/prompt`と`internal/agent`のテストで確かめる
- [ ] 2.2 `workspace`に、excludeの行を1つだけ足して置き場所を空にする準備を足す。2回呼んでも行が1つであることをテストで確かめる
- [ ] 2.3 <workspace.CollectScreenshots>で、拡張子、5MB、10枚、シンボリックリンク、下のディレクトリを扱う。境界のテストで確かめる
- [ ] 2.4 `CheckOutput`で、置き場所の下のファイルを含む差分を問題として返す。テストで確かめる

## 3. 添付

- [ ] 3.1 <forge.Attacher>とfakeの実装を足す。fakeのテストで確かめる
- [ ] 3.2 GitHubの`gh`の層を足す。偽の`gh`をPATHに置いたテストで、引数、環境、標準入力の本文を確かめる
- [ ] 3.3 GitLabのuploads APIの層を足す。httptestで確かめる
- [ ] 3.4 `internal/marker`に状態`attachment`を足し、`Transient()`に含める。`internal/marker`のテストで確かめる
- [ ] 3.5 `finish`と`writeOut`に、画像の収集と画像のコメントを足す。添付する、やり直しで二重にならない、失敗しても`agent:done`のまま、画像が無ければ何もしない、の4つをfakeのテストで確かめる
- [ ] 3.6 `switchboard doctor`で`gh`の有無と版を確かめる。版の読み取りのテストで確かめる

## 4. 文書

- [ ] 4.1 ADR-047を`docs/DECISIONS.md`に足し、`docs/SPEC.md`とREADMEの指示の節を更新する。`make verify`が通ることで確かめる
- [ ] 4.2 `docs/examples/screenshots.md`に`implement.md`の文面と`allow_tools_extra`の例を書く
- [ ] 4.3 V43・V44を`docs/ROADMAP.md`の検証項目に足す。V43はタグ`realgh`のテストで再現できるようにする

## 5. 全体の確認

- [ ] 5.1 `make verify`と`openspec validate add-implement-instructions-and-screenshots --strict`が通ることを確かめる
