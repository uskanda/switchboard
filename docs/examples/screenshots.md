# UIを変えた実装でスクリーンショットを付ける設定の例

UIを変えた実装では、変わった画面をissueの上で確かめたい。switchboardが受け持つのは、画像の置き場所の取り決めと、issueへの添付だけである（ADR-048）。撮影の方針と、撮影に要るコマンドの許可は、利用者が次の2つで書く。1つは実装の実行だけへの追加の指示（`implement.md`）で、もう1つは追加で許可するツール（`allow_tools_extra`）である。

## 指示

次の内容を、`switchboard instructions edit --implement gh:YOUR_GITHUB_USER/your-repo` で書く。`implement.md` は、直接の実装と承認のあとの実装にだけ渡る。CIの失敗などを直す実行には渡らないので、直す実行で撮影に時間を使うことはない。

```markdown
When you change the user interface, take screenshots of the changed screens and of the steps the issue names.

- Web: start the development server, then drive the pages with Playwright and save the screenshots.
- App: run the app on an emulator or a simulator and save the screenshots.
- When nothing here can run the app, take no screenshots and say why in the summary.
```

置き場所（`.switchboard/screenshots/`）と上限は、switchboardがsystem promptで伝える。指示に書き直す必要は無い。

## 許可

撮影に使うコマンドを、そのプロジェクトにだけ許す。WebでPlaywrightを使う例は次のとおりである。

```yaml
projects:
  - ref: gh:YOUR_GITHUB_USER/your-web-app
    allow_tools_extra: ["Bash(pnpm dev)", "Bash(pnpm exec playwright *)"]
```

Androidのエミュレーターで撮る例は、`adb` を許す。

```yaml
projects:
  - ref: gh:YOUR_GITHUB_USER/your-android-app
    allow_tools_extra: ["Bash(adb *)", "Bash(./gradlew installDebug)"]
```

iOSのシミュレーターで撮る例は、`xcrun simctl` を許す。

```yaml
projects:
  - ref: gh:YOUR_GITHUB_USER/your-ios-app
    allow_tools_extra: ["Bash(xcrun simctl *)"]
```

`npx` のように、任意のパッケージを取ってきて動かせるコマンドは許さない。

## 端末の準備

GitHubのプロジェクトでは、端末に `gh` のv2.99.0以上を入れる。`switchboard doctor` で版を確かめられる。`gh` にログインしておく必要は無い。switchboardは、エージェントのトークンだけを `gh` に渡す。

## 確かめ方

UIを変えるissueを1つ実装まで進める。doneの投稿のあとに、画像だけのコメントが1件付いていればよい。issueのブランチに `.switchboard/screenshots/` が入っていないことも確かめる。
