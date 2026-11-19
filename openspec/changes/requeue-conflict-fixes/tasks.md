# Tasks

## 1. マーカーのキー

- [ ] 1.1 `marker_test.go`に、`requeue=`の書き出しと読み取りの失敗するテストを書き、`marker.Marker.Requeue`を足して通す。冒頭のコメントにも書く。`go test ./internal/marker/`が通ることを確かめる

## 2. 入り直しと再開

- [ ] 2.1 `conflict_test.go`に、`max_fix_attempts: 1`で上限を超えると、`state=queued reason=conflict run=`のコメントが付き、`agent:ready`になり、順番が来て`reason=fix requeue=1`の衝突を直す実行が「（1/1）」で始まるテストを書き、失敗するのを確かめる
- [ ] 2.2 スロットが埋まっているとき、ほかの順番待ちのissueの後に始まるテストを書く
- [ ] 2.3 入り直しが上限に達したあとは`waiting(reason=conflict)`になり、本文に巡の数が書かれるテストと、`max_fix_attempts: 0`では入り直さないテストを書く
- [ ] 2.4 信頼済みの人の返答で入り直しの回数が0に戻るテスト、再起動の後に印から再開できるテスト、CIの失敗の上限超えは`waiting(ci)`のままのテストを書く
- [ ] 2.5 `fixes()`と`requeues()`、`requeueOutcome`、`limitResume`、`decide`、`launch`、`claim`、着手の文（英日）を実装し、2.1〜2.4のテストと`go test ./internal/agent/...`が通ることを確かめる

## 3. 受信箱の表示

- [ ] 3.1 `queueText.test.ts`に`reason=conflict`の文の失敗するテストを書き、`queueText`に足して通す。`inboxapi`のコメントにreasonの値を足す

## 4. 文書

- [ ] 4.1 `docs/DECISIONS.md`にADR-077を足し、ADR-054と衝突の待ちのADRから参照する
- [ ] 4.2 `docs/DESIGN.md`のマーカーの表、`docs/GUIDE.md`と`docs/GUIDE.ja.md`の直す回数と順番待ちの節、`docs/SPEC.md`の該当箇所を直す

## 5. 全体の確認

- [ ] 5.1 `make verify`と`openspec validate requeue-conflict-fixes --strict`が通ることを確かめる
