# RELEASING

port-keeper-mcpと同じ方式。タグを打つだけで、GitHub Actionsがビルド・検証・署名・公開まで行う。人が手で配布物を作ることはない。

## 1. 手順

1. `main` でCIが通っていることを確認する（`go test -race ./...`、`sh scripts/install_test.sh`）
2. `docs/ROADMAP.md` の「いまの状態」を更新する
3. タグを打って push する。版は `vX.Y.Z`（セマンティックバージョン）

   ```sh
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin v0.1.0
   ```

4. `.github/workflows/release.yml` が動く。ジョブ `release`: タグのソースをテスト → goreleaserで6つのアーカイブ（darwin/linux/windows × amd64/arm64）、6つのMCP Bundle（`.mcpb`。`scripts/mcpb.sh`がビルド後フックで作る）、`checksums.txt` を下書きリリースに作る → `chat-preview --version` が版を名乗ること、バンドルの中身（`scripts/mcpb.sh check`）を検査 → ビルド来歴（attestation）を付ける → 公開。続くジョブ `registry`: 公開された `checksums.txt` から `server.json` を描画（`scripts/registry.sh render`）し、`mcp-publisher` の GitHub OIDC 認証で公式MCPレジストリに登録する（§4）
5. 公開後に確かめる

   ```sh
   CHAT_PREVIEW_VERSION=0.1.0 sh scripts/install.sh      # 本物のリリースから入る
   chat-preview --version
   gh attestation verify chat-preview_0.1.0_darwin_arm64.tar.gz --repo gridhra/chat-tool-ui-preview
   sh scripts/registry.sh exists 0.1.0                   # レジストリに載ったか
   ```

   macOSのClaude Desktopがある環境では、`chat-preview_0.1.0_darwin_arm64.mcpb` をダブルクリックで取り込み、Claude Desktopのチャットで `preview_message` ツールが見えることも確かめる（実行権限の回避策が効いているかは、これでしか分からない）

## 2. 失敗したとき

- 公開前（下書き）で失敗したら、原因を直して同じタグで `workflow_dispatch`（入力 `tag`）から再実行する。下書きは置き換えられる
- 公開後は資産を変えられない。次の版を切る
- ジョブ `registry` だけが失敗したら、原因を直して `main` に push し、同じタグで `workflow_dispatch`（入力 `tag`、ブランチは `main`）から再実行する。ジョブ `release` は公開済みを検出して何もせず、`registry` は `main` の `server.json` と `scripts/registry.sh` で描画し、未登録なら登録し、登録済みなら何もしない。由来: `v0.1.1` で `server.json` の `description` がレジストリの上限（100文字）を超えて422で落ちた。`scripts/registry.sh check` が上限を検査するようにした

## 3. 配布物の名前を変えるとき

アーカイブ名 `chat-preview_<版>_<os>_<arch>`、バンドル名 `chat-preview_<版>_<os>_<arch>.mcpb`、`checksums.txt` は、`.goreleaser.yaml`、`scripts/install.sh`、`scripts/install.ps1`、`scripts/mcpb.sh`、`scripts/registry.sh`、READMEの手動インストール手順が揃って参照している。同時に直す。

## 4. MCPレジストリ・ディレクトリ

### 4.1 公式MCPレジストリ（registry.modelcontextprotocol.io）

**登録している**。名前は `io.github.gridhra/chat-tool-ui-preview`。形式は `mcpb`（MCP Bundle。`manifest.json`とバイナリを1つのzipにしたもの。GitHub Releaseの資産として置き、URLとSHA-256で登録する）。OSとCPUごとに6つのパッケージを並べる。

- 仕組み: `.goreleaser.yaml` のビルド後フックが `scripts/mcpb.sh pack` で各バイナリを `dist/mcpb/chat-preview_<版>_<os>_<arch>.mcpb` にし、`checksums.txt` と来歴の署名に含めてリリースに載せる。公開後、`release.yml` のジョブ `registry` が `scripts/registry.sh render` で `server.json`（リポジトリのものは雛形。版と `packages` はこのとき入る）を描画し、`mcp-publisher login github-oidc` → `publish` で登録する。レジストリは登録時に各URLへHEADを送って存在を確かめるので、リリース公開前には登録できない
- 認証: GitHub Actions の OIDC トークン。名前空間 `io.github.gridhra` はリポジトリ所有者のもので、秘密情報は置かない。`mcp-publisher` の版とtar.gzのSHA-256は `release.yml` の `env` に固定してあり、上げるときは両方を一緒に直す
- macOSの回避策: macOSのClaude Desktopはバンドルを展開するとき実行権限を落とす（`modelcontextprotocol/mcpb` issue #294、2026-10-06時点で未修理）。そのため darwin 用の `manifest.json` は `command` を `/bin/sh`、`args` を `-c 'chmod u+x "$0" && exec "$0" mcp' <バイナリ>` にしている。issueが閉じたら `scripts/mcpb.sh` の darwin の分岐を windows／linux と同じ直接起動に戻してよい。確認済み（2026-10-06、Claude Desktop 2.19675.0、macOS arm64）: スナップショット版の `.mcpb` を取り込むと、展開された `manifest.json` と `LICENSE` は0600（不具合の再現）、バイナリは回避策の `chmod` で0700になり、ログ `~/Library/Logs/Claude/mcp-server-chat-tool-ui-preview.log` に `initialize` と `tools/list` の成功が残った
- port-keeper-mcp と判断が違う理由: port-keeperは「クライアントの作業ディレクトリ」と「ホーム配下の台帳」を要し、Claude Desktopにはどちらも無いので mcpb を見送った（同リポジトリ `docs/DESIGN.md` §9.2）。本ツールはブラウザを開くだけで作業ディレクトリに依存しないので、その理由が当たらない。「`.mcpb`を取り込めるクライアントはClaude Desktopだけ」という点は同じで、Claude Code／VS Code／Cursor の利用者は引き続き `install.sh` で入れる
- 手元での確認: `goreleaser release --snapshot --clean` で `dist/mcpb/` に6つできる。`sh scripts/mcpb_test.sh` が pack／check／再現性／darwin の chmod 回避／`server.json` の描画を試す（CIでも走る）。`server.json` の描画を単体で見るなら `sh scripts/registry.sh render 0.1.0 checksums.txt`

### 4.2 Glama

Glama（https://glama.ai 。MCPサーバーの登録・評価サイト）への掲載は、Glamaのサイトのフォームから人が行う。port-keeper-mcpの `RELEASING.md` §8 と同じ手順。CLIからは行えない。

状態（2026-10-06）: 「Add Server」から審査に出した（名前 `chat-tool-ui-preview`、リポジトリURL）。承認のメールが届いたら、port-keeperの §8.1 以降（Dockerfileの設定、チェック、Glama上のリリース、Auto-Releaseをオフ）を進める。Glamaのコンテナは `initialize` と `tools/list` を取るだけの動作確認用で、配布ではない（本ツールはコンテナ内ではブラウザを開けない）。

## 5. リポジトリの設定（初回）

- `main` を保護し、CI（`test (ubuntu-latest)`、`test (macos-latest)`）の成功を必須にする
- Releases を immutable にする（公開後の資産差し替えを禁止）
- Actions の `Workflow permissions` は `Read repository contents`（必要な権限はワークフロー側で宣言している）
