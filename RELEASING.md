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

4. `.github/workflows/release.yml` が動く。やること: タグのソースをテスト → goreleaserで6つのアーカイブ（darwin/linux/windows × amd64/arm64）と `checksums.txt` を下書きリリースに作る → `chat-preview --version` が版を名乗ることを検査 → ビルド来歴（attestation）を付ける → 公開
5. 公開後に確かめる

   ```sh
   CHAT_PREVIEW_VERSION=0.1.0 sh scripts/install.sh      # 本物のリリースから入る
   chat-preview --version
   gh attestation verify chat-preview_0.1.0_darwin_arm64.tar.gz --repo gridhra/chat-tool-ui-preview
   ```

## 2. 失敗したとき

- 公開前（下書き）で失敗したら、原因を直して同じタグで `workflow_dispatch`（入力 `tag`）から再実行する。下書きは置き換えられる
- 公開後は資産を変えられない。次の版を切る

## 3. 配布物の名前を変えるとき

アーカイブ名 `chat-preview_<版>_<os>_<arch>` と `checksums.txt` は、`.goreleaser.yaml`、`scripts/install.sh`、`scripts/install.ps1`、READMEの手動インストール手順が揃って参照している。同時に直す。

## 4. MCPレジストリ・ディレクトリ

- **公式MCPレジストリ（registry.modelcontextprotocol.io）は未登録**。port-keeper-mcpと同じ判断。受け付けるパッケージ形式（npm、PyPI、NuGet、oci、mcpb）に「GitHub Releaseの単体バイナリ」が無く、npxランチャーもコンテナも配らない方針のため。レジストリに提案中の`go`形式が入ったら再検討する。`server.json` はそのときのために置いてある（`version` はリリース時に手で合わせる）
- **Glama**（MCPサーバーのディレクトリ）への掲載は、Glamaのサイトのフォームから人が行う。port-keeper-mcpの `RELEASING.md` §8 と同じ手順。CLIからは行えない

## 5. リポジトリの設定（初回）

- `main` を保護し、CI（`test (ubuntu-latest)`、`test (macos-latest)`）の成功を必須にする
- Releases を immutable にする（公開後の資産差し替えを禁止）
- Actions の `Workflow permissions` は `Read repository contents`（必要な権限はワークフロー側で宣言している）
