# chat-tool-ui-preview — コーディングエージェント向けの覚え書き

まず`README.md`（何をするものか、やらないこと）を読み、次に`docs/DESIGN.md`（要件・権利面の規則・設計・判断の記録）を読む。進捗と予定は`docs/ROADMAP.md`。

## コマンド

```sh
go test -race ./...                 # 全テスト。commit前に必ず通す
go vet ./... && gofmt -l .          # gofmtの出力は空でなければならない
go test ./internal/render -update   # 描画のゴールデン更新（差分を目で見てから）
go run ./cmd/chat-preview gallery   # 書式要素の見本をブラウザで開く
go run ./cmd/chat-preview mcp       # MCPサーバー（stdio）
```

## 破りやすい規則

- **Slackの見た目も画面構成も真似ない**（docs/DESIGN.md §2.2 R2・R9）。配色・フォント（Lato）・ロゴ・アイコン・CSSを持ち込まない。アバター・名前・時刻を並べたメッセージ行、色分けしたボタン、色付き背景のメンションを描かない。画面内に「Slack」「Block Kit」「mrkdwn」の語を出さない（`internal/render`のテストが検査する）。製品名やコマンド名に「slack」を入れない
- **投稿内容を端末の外に出さない**。画像やリンク先を読み込まない（CSPは`default-src 'none'`）。リンクは`href`を持たせない。一時ファイルは0600で作り、1時間で掃除する。ログに本文を残さない
- **Slack APIに問い合わせない**。トークンを持たない。表示名は`context.names`で呼び出し側が渡す。分からないものは警告に列挙する
- **入力形式を推定しない**。`format`が無いか不正ならエラーにする
- **クライアント固有の作法はアダプタに閉じ込める**（`internal/cli/hook_claude.go`）。核（`internal/{model,parse,render}`）はどのクライアントから使われるかを知らない
- テストでは実際にブラウザを開かない（`cli.App.Open`／`mcpserver.Deps.Open`を差し替える）。実ポートにbindしない。MCPはin-memory transportで試験する
- MCPツールの省略可能な入力には`omitempty`を付ける（SDKが入力スキーマを検証する）。出力の配列は失敗時でもnilにしない（`null`になりスキーマ違反になる）

## 進め方

- 変更はまず`docs/DESIGN.md`の該当節を直し、判断は§7の表に日付つきで残す
- 日本語の文中で、半角英数字の前後に半角スペースを入れない
