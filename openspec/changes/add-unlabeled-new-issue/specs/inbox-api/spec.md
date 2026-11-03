# Spec Delta

## MODIFIED Requirements

### Requirement: issueを作れる先の一覧
APIは、設定にあるすべてのプロジェクトを、設定の順に、担当の端末の名前とともに返さなければならない（MUST）。`switchboard inbox serve` が動く端末の担当でないプロジェクトも含める。
APIは、開始ラベルの種別（`ready`、`plan`、`go`）と、それぞれのラベル名を返さなければならない（MUST）。設定でラベル名が空の種別は返さない。
APIは、開始ラベルの種別の最後に、ラベルなしを表す種別 `none` を、空のラベル名で常に返さなければならない（MUST）。
設定の読み直しで現れたプロジェクトは、再起動しなくても一覧へ出なければならない（MUST）。

#### Scenario: ほかの端末の担当プロジェクト
- **WHEN** `laptop` の担当の `gl:nas/sb` を含む設定で動く、端末 `usknas` の受信箱のサービスで、一覧を求める
- **THEN** `gl:nas/sb` が担当の端末 `laptop` とともに返る

#### Scenario: planのラベル名が空
- **WHEN** `labels.plan` が空の設定で、一覧を求める
- **THEN** 開始ラベルの種別として、`ready`、`go`、`none` がこの順に返り、`none` のラベル名は空である

### Requirement: issueの作成
APIは、`ref`、タイトル、本文、開始ラベルの種別を受けて、そのプロジェクトに、受信箱のトークンでissueを作らなければならない（MUST）。種別が `none` でなければ、issueには種別に当たる設定のラベルを1つだけ付ける。
種別が `none` のとき、APIはissueにラベルを1つも付けてはならない（MUST NOT）。
APIは、作ったissueの参照（`gh:owner/repo#n` の形）とフォージのページのURLを返さなければならない（MUST）。
作ったあと、APIは一覧のキャッシュを捨て、フォージの読み直しを促さなければならない（MUST）。

#### Scenario: 計画から始めるissue
- **WHEN** `labels.plan` が `agent:plan` の設定で、`gl:nas/sb` に種別 `plan` のissueを作る
- **THEN** `gl:nas/sb` に、受信箱のトークンの持ち主が作者で、`agent:plan` だけが付いたissueができ、その参照とURLが返る

#### Scenario: ラベルなしのissue
- **WHEN** `gh:o/r` に種別 `none` のissueを作る
- **THEN** `gh:o/r` に、受信箱のトークンの持ち主が作者で、ラベルの無いissueができ、その参照とURLが返る

#### Scenario: 種別を送らない
- **WHEN** 種別を空にして送る
- **THEN** issueは作られず、`invalid` の誤りが返る
