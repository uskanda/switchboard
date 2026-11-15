## ADDED Requirements

### Requirement: マージの前の状態のbaseのCI
マージの前の状態を返すとき、APIは、そのissueのプロジェクトのbaseのブランチの先頭のコミットのCIで、失敗したチェックの名前とURLを返さなければならない（MUST）。失敗の数え方は、エージェントのCIの見張りと同じとする。
baseの先頭のCIを読めなかったとき、APIは、baseのCIについて何も返さず、ほかの状態を今までどおり返さなければならない（MUST）。

#### Scenario: mainが赤い
- **WHEN** smarternewsのmainの先頭のコミットで、job `verify` が失敗している。そのプロジェクトの完了のissueの、マージの前の状態を求める
- **THEN** 答えのbaseの失敗したチェックに、`verify` の名前とURLがある

#### Scenario: mainが緑
- **WHEN** mainの先頭のコミットのCIが成功している
- **THEN** 答えのbaseの失敗したチェックは空である

#### Scenario: baseを読めない
- **WHEN** baseのブランチを読む問い合わせが、フォージの一時的な誤りで失敗する
- **THEN** 答えにbaseのCIは無く、PR/MRの衝突とCIは今までどおり返る
