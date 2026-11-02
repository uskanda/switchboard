## ADDED Requirements

### Requirement: issueを作れる先の一覧
APIは、設定にあるすべてのプロジェクトを、設定の順に、担当の端末の名前とともに返さなければならない（MUST）。`switchboard inbox serve` が動く端末の担当でないプロジェクトも含める。
APIは、開始ラベルの種別（`ready`、`plan`、`go`）と、それぞれのラベル名を返さなければならない（MUST）。設定でラベル名が空の種別は返さない。
設定の読み直しで現れたプロジェクトは、再起動しなくても一覧へ出なければならない（MUST）。

#### Scenario: ほかの端末の担当プロジェクト
- **WHEN** `laptop` の担当の `gl:nas/sb` を含む設定で動く、端末 `usknas` の受信箱のサービスで、一覧を求める
- **THEN** `gl:nas/sb` が担当の端末 `laptop` とともに返る

#### Scenario: planのラベル名が空
- **WHEN** `labels.plan` が空の設定で、一覧を求める
- **THEN** 開始ラベルの種別として、`ready` と `go` だけが返る

### Requirement: issueの作成
APIは、`ref`、タイトル、本文、開始ラベルの種別を受けて、そのプロジェクトに、受信箱のトークンでissueを作らなければならない（MUST）。issueには、種別に当たる設定のラベルを1つだけ付ける。
APIは、作ったissueの参照（`gh:owner/repo#n` の形）とフォージのページのURLを返さなければならない（MUST）。
作ったあと、APIは一覧のキャッシュを捨て、フォージの読み直しを促さなければならない（MUST）。

#### Scenario: 計画から始めるissue
- **WHEN** `labels.plan` が `agent:plan` の設定で、`gl:nas/sb` に種別 `plan` のissueを作る
- **THEN** `gl:nas/sb` に、受信箱のトークンの持ち主が作者で、`agent:plan` だけが付いたissueができ、その参照とURLが返る

### Requirement: issueの作成の前の確認
次のどれかに当たるとき、APIはissueを作ってはならず、それぞれを区別できる誤りを返さなければならない（MUST NOT）。
- `ref` が設定に無い（`unknown_project`）
- タイトルが空白だけか、256字を超える（`invalid`）
- 本文が1MiBを超える（`too_large`）
- 種別が、一覧で返す開始ラベルの種別に無い（`invalid`）
- 本文がswitchboardのマーカー行で終わる（`invalid`）

フォージがissueを作れなかったとき、APIは `forge` の誤りを返さなければならない（MUST）。

#### Scenario: ラベル名を種別として送る
- **WHEN** 種別に `agent:done` を送る
- **THEN** issueは作られず、`invalid` の誤りが返る

#### Scenario: 設定に無いプロジェクト
- **WHEN** 設定に無い `gh:o/nope` を送る
- **THEN** issueは作られず、`unknown_project` の誤りが返る
