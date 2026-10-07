# ROADMAP

更新: 2026-10-06

## いまの状態

M0（核）、M1（Claude Codeフック v0）、MCPサーバー v0（`preview_message`。開いて警告を返すまで）が実装済み。`go test -race ./...` が通る。配布の仕組み（goreleaser、インストールスクリプト、CI、server.json）はport-keeper-mcpから移植済み。GitHub `gridhra/chat-tool-ui-preview` に公開し、`main` の保護・タグのルールセット・immutable releases・Actionsの読み取り専用権限を設定済み（2026-10-06）。初回リリース `v0.1.0` を同日に発行。Glamaに審査を申請済み（承認待ち）。公式MCPレジストリへの登録（`.mcpb` 6つを `io.github.gridhra/chat-tool-ui-preview` として、リリース時に自動で）を実装済みで、次の版 `v0.1.1` で初めて動く（`RELEASING.md` §4）。

- 核: Slack記法／標準Markdown／Block Kit → 内部モデル → 静的HTML。警告欄。`gallery`で見本5種
- CLI: `preview` `gallery` `hook claude` `mcp` `version`
- MCP: `chat-preview mcp`（stdio）。ツール `preview_message`。in-memory transport のテストと、stdioの手動疎通（initialize → tools/list）を確認済み
- フック: `hook claude` は PreToolUse の stdin を読み、プレビューを開いて `permissionDecision: "ask"` を返す
- 確認済み: 端末から `hook claude` を実行するとブラウザが開く（macOS、2026-10-06）。**Claude Codeのフックとして実際に割り込めるかは未確認**（次項）

## 次にやること

1. **実機でフックを検証**: `~/.claude/settings.json` にREADMEの設定を入れ、Claude CodeにSlack送信を頼み、許可ダイアログにプレビューの案内が出てブラウザが開くことを確かめる。開かなければ `permissionDecisionReason` のパスを手で開く案内に落とす
2. M2: ローカルサーバー版（ページ上で承認／修正／却下。フックは`updatedInput`で本文を差し替え、MCPの`preview_message`は判断を待って結果を返す）
3. `v0.1.1` を切り、`.mcpb` 付きのリリースとレジストリ登録が通ることを確かめる（macOSのClaude Desktopでスナップショット版の `.mcpb` を取り込み、サーバーが起動して `tools/list` に応答することは2026-10-06に確認済み。`RELEASING.md` §4.1）
4. Glamaの承認メールが届いたら、Dockerfileの設定とチェックを通す（`RELEASING.md` §4.2）
5. 細部: Markdownの表をSlackがどう扱うかの実測、`rich_text`の未対応要素、Discord／Teamsの`target`

## やらないと決めたこと

`docs/DESIGN.md` §1.3（非ゴール）と §2（権利面）。Slackの見た目の再現、Slack APIへの問い合わせ、常駐、投稿内容の保存。
