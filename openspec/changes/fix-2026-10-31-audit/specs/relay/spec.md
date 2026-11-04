# Spec Delta

## ADDED Requirements

### Requirement: 空のシークレットを拒む
webhookのシークレットが空のとき、中継は起動してはならない（MUST NOT）。検証のときにも、空の鍵で署名を確かめてはならない（MUST NOT）。

#### Scenario: 空のシークレット
- **WHEN** 中継の設定が指すwebhookのシークレットが空である
- **THEN** 中継は起動時に誤りを返す

### Requirement: 配信IDの無いwebhookはログを残して捨てる
配信IDの無いwebhookが届いたとき、中継は、ログを残してから捨てなければならない（MUST）。

#### Scenario: 配信IDの無いwebhook
- **WHEN** 署名の正しいwebhookが、配信IDのヘッダ無しで届く
- **THEN** 中継はイベントを送らず、捨てたことをログに書く

### Requirement: SSEのIDは再起動をまたいで増える
中継は、再起動の後に振るSSEのIDを、再起動の前に振ったどのIDよりも大きくしなければならない（MUST）。

#### Scenario: 再起動の後の再接続
- **WHEN** 中継が再起動し、エージェントが古い `Last-Event-ID` で再接続する
- **THEN** 新しい回の最初のイベントから送られる
