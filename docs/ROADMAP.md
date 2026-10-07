# ROADMAP

更新: 2026-10-07

## いまの状態

M0（核）、M1（Claude Codeフック v0）、MCPサーバー v0（`preview_message`。開いて警告を返すまで）が実装済み。`go test -race ./...` が通る。配布の仕組み（goreleaser、インストールスクリプト、CI、server.json）はport-keeper-mcpから移植済み。GitHub `gridhra/chat-tool-ui-preview` に公開し、`main` の保護・タグのルールセット・immutable releases・Actionsの読み取り専用権限を設定済み（2026-10-06）。初回リリース `v0.1.0` を同日に発行。Glamaに審査を申請済み（承認待ち）。`v0.1.1`（2026-10-07）で `.mcpb` 6つを付け、公式MCPレジストリに `io.github.gridhra/chat-tool-ui-preview` 0.1.1 として登録済み（リリース時に自動。`RELEASING.md` §4。初回は `description` の長さで一度落ち、`main` を直して再実行した）。

- 核: Slack記法／標準Markdown／Block Kit → 内部モデル → 静的HTML。警告欄。`gallery`で見本5種
- CLI: `preview` `gallery` `hook claude` `mcp` `version`
- MCP: `chat-preview mcp`（stdio）。ツール `preview_message`。in-memory transport のテストと、stdioの手動疎通（initialize → tools/list）を確認済み
- フック: `hook claude` は PreToolUse の stdin を読み、プレビューを開いて `permissionDecision: "ask"` を返す
- 確認済み: 端末から `hook claude` を実行するとブラウザが開く（macOS、2026-10-06）。**Claude Codeのフックとして実際に割り込めるかは未確認**（次項）

## 次にやること

1. **実機でフックを検証**: `~/.claude/settings.json` にREADMEの設定を入れ、Claude CodeにSlack送信を頼み、許可ダイアログにプレビューの案内が出てブラウザが開くことを確かめる。開かなければ `permissionDecisionReason` のパスを手で開く案内に落とす
2. M2: ローカルサーバー版（ページ上で承認／修正／却下。フックは`updatedInput`で本文を差し替え、MCPの`preview_message`は判断を待って結果を返す）
3. 公開版 `v0.1.1` の `.mcpb` をmacOSのClaude Desktopで取り込み直して動作を見る（スナップショット版では2026-10-06に確認済み。`RELEASING.md` §4.1）。Windows用バンドルは実機未確認
4. Glamaの承認メールが届いたら、Dockerfileの設定とチェックを通す（`RELEASING.md` §4.2）
5. 細部: Markdownの表をSlackがどう扱うかの実測、`rich_text`の未対応要素、Discord／Teamsの`target`

## やらないと決めたこと

`docs/DESIGN.md` §1.3（非ゴール）と §2（権利面）。Slackの見た目の再現、Slack APIへの問い合わせ、常駐、投稿内容の保存。
