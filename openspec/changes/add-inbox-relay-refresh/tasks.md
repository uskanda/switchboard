# Tasks

## 1. 設定とトークン

- [x] 1.1 `internal/config` の `Inbox` に `Relay *AgentRelay` を足し、書いたときは `url` と `token_ref` を必須にする。読み込みと必須の検証を `internal/config` のテストで確かめる
- [x] 1.2 `relay.AddToken` に受信箱用の発行を足し、`relay token add --inbox` で呼ぶ。置き場所、`relay.hosts` の名前、`inbox.relay` の表示を `internal/relay` と `cmd/switchboard` のテストで確かめる
- [x] 1.3 `service.InboxCredentials` に `<inbox.relay>` のトークンを足す。`internal/service` のテストで確かめる

## 2. 受信箱のサービス

- [x] 2.1 `inboxapi.Inbox` に `Relay` と `RefreshIntervalSeconds` を足し、値の定数を置く
- [x] 2.2 `inboxserve` の `RelayEvent` を足す。2秒のまとめ、前回から10秒の間隔、画面が無いときのキャッシュの破棄を `internal/inboxserve` のテストで確かめる
- [x] 2.3 `RelayState` と、設定の有無を表す項目 `Relay` を足し、一覧に `relay` と `refresh_interval_seconds` を入れる。状態の変化がフォージを読まずに送られること、`relay` だけが違う2つの一覧を `same` が別のものとみなすこと、GETのキャッシュにも今の状態が入ることをテストで確かめる
- [x] 2.4 `inbox serve` が `<inbox.relay>` のトークンで中継につなぎ、つないだときと切れたときをログに出す。テスト用の配信口で確かめる

## 3. Web表示

- [x] 3.1 `Inbox` の型に2つの項目を足し、ヘッダの文言を中継の状態で分ける。3つの文言と、自動更新が切れているときは今の文言のままであることを `live.test.tsx` で確かめる

## 4. 文書

- [x] 4.1 `README.md`、`DESIGN.md` のヘッダの記述、`docs/DECISIONS.md` のADR-035への追記を書き、textlintで確かめる

## 5. 確認

- [x] 5.1 `make verify` が通ることを確かめる
- [ ] 5.2 GitLabのwebhookとローカルの中継で、手で確かめる。ラベルを付け替えると数秒で一覧が変わる。中継を止めるとヘッダが「自動更新中（30秒ごと）」に、戻すと「自動更新中（即時）」になる
