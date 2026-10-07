# chat-tool-ui-preview

AIエージェント（Claude Codeなど）がSlackに投稿する**前に**、その投稿を人が画面で確認するための小さなローカルツール。Go製の単体バイナリ `chat-preview`。

表示はSlackの画面を写したものではなく、本ツール独自のチャット風レイアウトです。書式（太字・引用・メンション・ブロックの並び）と文脈（宛先・スレッド返信かどうか・ボットとしてか）の**意味**が正しく伝わることを目的にしています。Slackのロゴ・配色・フォント・画面構成は使いません。本ツールはSlack Technologies, LLCが制作・提携・支援するものではありません。

## 何ができるか

- 入力形式3つ: 標準Markdown（Claude CodeのSlack連携の`message`）、Slack記法（mrkdwn）、Block Kit JSON
- 書式の描画: 太字／斜体／打消／コード／コードブロック／引用／リンク／メンション／日付／絵文字／箇条書き／表、Block Kitの section・header・divider・context・image・actions・rich_text
- 文脈の表示: 宛先、スレッド返信とチャンネルにも表示、送信主体（ユーザー／ボット）
- 自動検査の警告: 表示名が分からないメンション、未知の絵文字、長さ超過、不正・未対応のブロック
- Claude Codeの**PreToolUseフック**として動き、Slack送信ツールの呼び出しに自動で割り込む。プレビューをブラウザで開いたうえで、判断はClaude Codeの通常の許可ダイアログに委ねる

やらないこと: Slackへの送信そのもの、Slack APIへの問い合わせ（トークンを持たない）、常駐、投稿内容の保存。画像やリンク先は読み込まない（投稿前に相手サーバーへアクセスが飛ぶのを避ける）。

## インストール

`chat-preview`は実行時の依存を持たない単一の静的バイナリです。`PATH`の通った場所に置いてください。シェルも、Claude Codeのフックも、MCPクライアントも、同じ`chat-preview`コマンドを呼びます。

```sh
# macOS／Linux
curl -fsSL https://raw.githubusercontent.com/gridhra/chat-tool-ui-preview/main/scripts/install.sh | sh
# Windows（PowerShell）。CIでクロスビルドはしていますが、Windowsの実機では未検証です
irm https://raw.githubusercontent.com/gridhra/chat-tool-ui-preview/main/scripts/install.ps1 | iex
```

スクリプトは、最新の[GitHub Release](https://github.com/gridhra/chat-tool-ui-preview/releases)からOSとCPUに合うアーカイブを選び、そのSHA-256がリリースの`checksums.txt`と一致しない限り何もインストールせず、`chat-preview`を`~/.local/bin`に置きます。`sudo`は求めません。置き場所は`CHAT_PREVIEW_INSTALL_DIR`、バージョンは`CHAT_PREVIEW_VERSION`で指定できます。更新はもう一度実行するだけです。

アーカイブが「このリポジトリのリリース用ワークフローが、タグの付いたソースからビルドしたもの」であることを確かめるには:

```sh
gh release download --repo gridhra/chat-tool-ui-preview --pattern '*darwin_arm64.tar.gz' --pattern checksums.txt
shasum -a 256 -c --ignore-missing checksums.txt
gh attestation verify chat-preview_*_darwin_arm64.tar.gz --repo gridhra/chat-tool-ui-preview
```

Goのツールチェーン（1.24以上）があるなら:

```sh
go install github.com/gridhra/chat-tool-ui-preview/cmd/chat-preview@latest
```

コンテナイメージと`npx`ランチャーは用意しません。ブラウザを開く必要があるので、ホストで動かすのが前提です。

### Claude Desktopに入れる（MCP Bundle）

各リリースには、OSとCPUごとの`chat-preview_<版>_<os>_<arch>.mcpb`も付いています。MCP Bundle（`.mcpb`）は、`manifest.json`とバイナリを1つのzipにした配布形式で、Claude Desktop（macOS／Windows）がそのまま取り込めます。[Releases](https://github.com/gridhra/chat-tool-ui-preview/releases)から自分のOSとCPUに合う`.mcpb`を落とし、ダブルクリック（またはClaude Desktopの設定 → エクステンション → 詳細設定 → エクステンションをインストール）で入ります。中身は上のアーカイブと同じバイナリで、`checksums.txt`と`gh attestation verify`で同じように確かめられます。

macOSのClaude Desktopには、取り込んだバイナリの実行権限を落とす不具合（modelcontextprotocol/mcpb issue #294、2026-10-06時点で未修理）があるため、macOS用のバンドルは`/bin/sh`経由で起動して実行権限を付け直してから`chat-preview mcp`を実行します。Windows用は`.exe`を直接起動します。

同じバンドルを[公式MCPレジストリ](https://registry.modelcontextprotocol.io)に`io.github.gridhra/chat-tool-ui-preview`として登録しています。レジストリから入れられるクライアントでは、その名前で探せます。

## 使い方

```sh
# 入力JSONをプレビューしてブラウザで開く
chat-preview preview input.json
echo '{"format":"mrkdwn","text":"*hi* <@U0AAA>","context":{"channel_id":"C0GEN","names":{"U0AAA":"aquila"}}}' | chat-preview preview

# 書式要素の見本を1枚で見る
chat-preview gallery
```

入力JSONの形:

```json
{
  "target": "slack",
  "format": "markdown | mrkdwn | blocks",
  "text": "本文（markdown / mrkdwn のとき。blocks のときは通知用の代替文）",
  "blocks": [ ... ],
  "context": {
    "channel_id": "C0123", "channel_name": "general",
    "thread_ts": "1700000000.000100", "reply_broadcast": false,
    "as": "user | bot", "sender_name": "Deploy Bot",
    "names": { "U0123": "aquila", "C0123": "general" }
  },
  "source": "claude-slack-mcp | slack-web-api | manual"
}
```

`format`は必須で、推定しません（標準MarkdownとSlack記法は `*bold*` の意味が違うため）。`names`はID→表示名の対応表で、無いIDは「未解決」として警告に出ます。

## Claude Codeとの連携（フック）

`~/.claude/settings.json`（または対象プロジェクトの`.claude/settings.json`）に:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "mcp__claude_ai_Slack__slack_send_message|mcp__claude_ai_Slack__slack_schedule_message|mcp__claude_ai_Slack__slack_send_message_draft",
        "hooks": [{ "type": "command", "command": "chat-preview hook claude", "timeout": 600 }]
      }
    ]
  }
}
```

動き: エージェントがSlack送信ツールを呼ぶ → フックがプレビューHTMLを一時ファイル（本人のみ読める権限）に書いてブラウザで開く → Claude Codeの許可ダイアログに「プレビューを開いた。確認が必要な点: …」と出る → 人が許可／拒否を選ぶ。文面を直したいときは拒否して理由に書くと、エージェントが書き直して再びプレビューが開く。一時ファイルは1時間で自動的に消える。

## MCPサーバーとして使う

`chat-preview mcp` はstdioのMCPサーバーで、ツールは `preview_message` の1本です。入力はCLIの入力JSONと同じ項目（`format` `text` `blocks` `channel_id` `thread_ts` `names` など）で、出力はプレビューのパス、ブラウザを開けたか、警告一覧、次にすべきこと（「ユーザーに確認し、はいと言うまで送らない」）です。送信もSlackへの問い合わせも行いません。

Claude Codeに登録する例（ユーザースコープ）:

```sh
claude mcp add --scope user chat-tool-ui-preview -- chat-preview mcp
```

フックとMCPの使い分け: フックはエージェントが呼び忘れる余地がなく、Claude CodeのSlack送信ツールに自動で割り込みます。MCPは、他のクライアントや、送信ツールを経由しない場面（Web APIを直接叩くスクリプトを書いている最中など）でエージェントが任意に呼ぶためのものです。両方登録して構いません。

## 開発

```sh
go test -race ./...                       # 全テスト
go test ./internal/render -update         # 描画のゴールデンファイルを更新（差分を見てから）
go run ./cmd/chat-preview gallery         # 見本ページ
```

設計と判断の記録は `docs/DESIGN.md`、進捗と予定は `docs/ROADMAP.md`。

## ライセンス

MIT。絵文字の短縮名→Unicodeの対応は kyokomi/emoji（MIT）、Markdownの構文解析は goldmark（MIT）を使っています。絵文字の描画はOSのフォントに任せ、画像は同梱しません。
