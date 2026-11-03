## MODIFIED Requirements

### Requirement: issueを作れる先の一覧
APIは、設定にあるすべてのプロジェクトを、設定の順に、担当の端末の名前とともに返さなければならない（MUST）。`switchboard inbox serve` が動く端末の担当でないプロジェクトも含める。
APIは、各プロジェクトについて、設定の`plan_first`の値を`plan_first`として返さなければならない（MUST）。
APIは、開始ラベルの種別（`ready`、`plan`、`go`）と、それぞれのラベル名を返さなければならない（MUST）。設定でラベル名が空の種別は返さない。
設定の読み直しで現れたプロジェクトは、再起動しなくても一覧へ出なければならない（MUST）。

#### Scenario: ほかの端末の担当プロジェクト
- **WHEN** `laptop` の担当の `gl:nas/sb` を含む設定で動く、端末 `usknas` の受信箱のサービスで、一覧を求める
- **THEN** `gl:nas/sb` が担当の端末 `laptop` とともに返る

#### Scenario: 計画から始めるプロジェクト
- **WHEN** `plan_first: true` の `gh:o/r` を含む設定で、一覧を求める
- **THEN** `gh:o/r` の項目の `plan_first` が `true` で返る

#### Scenario: planのラベル名が空
- **WHEN** `labels.plan` が空の設定で、一覧を求める
- **THEN** 開始ラベルの種別として、`ready` と `go` だけが返る
