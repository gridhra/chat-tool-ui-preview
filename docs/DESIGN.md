# chat-tool-ui-preview — 要件・権利整理・設計（初版）

作成日: 2026-10-06 / ステータス: M0（核）・M1（Claude Codeフック v0）実装済み（2026-10-06）。進捗は`docs/ROADMAP.md`。本書は要件定義と設計。§7の判断事項のうちD1（見た目は書式忠実・中立）とD2（承認は端末の許可ダイアログから始め、必要ならページ上の承認へ）、D3（Go単体バイナリ。一度TypeScriptに決めたが、Node依存を嫌って同日中にGoへ戻した）も2026-10-06に決定済み。全判断済み。M0から着手する。

## 0. 一言で

AIエージェント（Claude Code等）がSlackなどのチャットツールに投稿しようとしたとき、**送信前にその投稿を人が画面で確認し、承認・修正・却下できる**小さなローカルツール。port-keeper-mcp（ローカル開発ポートの台帳CLI兼MCPサーバー）と同じ作りの方針（Go単体バイナリ、常駐なし、クライアント固有の作法はアダプタに隔離、秘密情報を外に出さない）で作る。

重要な前提（§2で詳述）: Slackのブランド規約は「Slackの特徴的な見た目（look and feel）を模倣しないこと」を明示している。したがって本ツールの目標は「**書式の意味がSlackと同じに見える**（太字・メンション・ブロック配置・スレッド返信かどうか等が正しく伝わる）」であって、「**Slackの画面とピクセル単位で同じ**」ではない。この線引きを設計の第一原則にする。

---

## 1. 要件

### 1.1 困りごと（背景）

- エージェントがSlackへ投稿するとき、ユーザーが見るのはツール呼び出しの引数（Markdown文字列やBlock Kit JSON）だけで、受け手にどう見えるかが分からない。太字の崩れ、メンションの誤り、長すぎる本文、スレッドではなくチャンネル直下に投稿してしまう等の事故が、送信後にしか分からない。
- Claude Codeの標準Slack連携（`mcp__claude_ai_Slack__slack_send_message`）は入力が標準Markdown（`message`）と`channel_id`・`thread_ts`で、Slack側で変換される。Slack Web API（`chat.postMessage`）直叩きの場合は入力がmrkdwn（Slack独自記法）または`blocks`（Block Kit JSON）で、形式が違う。
- Slack以外（Discord、Microsoft Teams）にも同じ困りごとがあるが、優先はSlack。

### 1.2 ゴール（v1の範囲）

| # | 要件 | 補足 |
| --- | --- | --- |
| G1 | 送信前に、投稿内容を**ブラウザのページとして表示**する | 1投稿＝1ページ。常駐しない |
| G2 | 入力形式を3つ受ける: 標準Markdown（Claude Slack MCP）、mrkdwn（Slack記法）、Block Kit JSON | 形式は呼び出し側が明示。推定はしない（§3.3） |
| G3 | 書式の意味を忠実に描く: 太字／斜体／打消／コード／コードブロック／引用／リンク（`<url|text>`）／メンション（`<@U..>` `<#C..>` `<!here>` `<!subteam^..>`）／日付（`<!date^..>`）／絵文字（`:name:`）／箇条書き | Slack公式の書式仕様（docs.slack.dev/messaging/formatting-message-text）に基づく |
| G4 | Block Kitの主要ブロックを描く: `section`（text／fields／accessory）、`header`、`divider`、`context`、`image`、`actions`（button）、`rich_text` | 入力部品（select、datepicker等）は「ここに〇〇がある」と枠で示すだけ。操作はできない |
| G5 | 投稿の**文脈**を表示する: 宛先（チャンネルID。名前は呼び出し側が渡せば表示）、スレッド返信か否か、`reply_broadcast`の有無、送信主体（人として／ボットとして） | Slack APIには問い合わせない（§3.6） |
| G6 | **解決できなかったもの**を一覧で警告する: 名前の分からないメンション、未知の絵文字、長さ超過（Slackは本文要素5000字）、不正なBlock Kit | 「確認したつもりで見落とす」を防ぐ |
| G7 | ユーザーの判断（承認／却下＋理由／修正後の本文）をエージェントに返す | v0は端末側で判断、v1はページ上で判断（§3.4） |
| G8 | Claude Codeからは、PreToolUseフック（ツール実行前に割り込む仕組み）で**自動的に**割り込める | 「エージェントがプレビューを呼び忘れる」を防ぐ。エージェント任意呼び出し用のMCPツールも持つ |
| G9 | 投稿内容を端末の外に出さない | loopbackのみ待ち受け、一時ファイルは本人のみ読める権限、判断後に削除 |

### 1.3 非ゴール

- **Slackの画面の見た目の再現**。ブランド規約で禁止されている（§2.1）。Slackの配色・フォント（Lato）・アイコン・CSSを使わない
- **Slackへの送信そのもの**。送るのはエージェントが既に持っているツール（Slack MCPやWeb API）。本ツールは判断を返すだけ
- **Slack APIへの問い合わせ**（チャンネル名・ユーザー名・カスタム絵文字の解決）。トークンを持たない。呼び出し側が名前の対応表を渡せば表示する
- **常駐デーモン・常設ポート**。1回の確認のたびに起動し、判断が出たら終わる
- **Slack側のMarkdown→mrkdwn変換の再現**。Claude Slack MCPの`message`（標準Markdown）をSlackがどう変換するかは非公開で、完全再現はできない。標準Markdownとして描き、「実際の変換はSlack側で行われる」と注記する
- **履歴・ログの保持**。投稿内容を台帳に残さない

### 1.4 既存ツールとの位置づけ（2026-10調査）

| 既存のもの | できること | 足りないこと |
| --- | --- | --- |
| Slack公式 Block Kit Builder | Block Kit JSONの公式プレビュー | ブラウザのSlackログインが要る。Markdown入力・スレッド文脈・承認の往復がない。エージェントから自動で開けない（URLにJSONを載せれば開ける） |
| Slack下書きAPI（Claude Slack MCPの`slack_send_message_draft`） | 本物のSlackに下書きを置く。見た目は100%正確 | Slackアプリを開いて探す手間。却下・修正の結果がエージェントに戻らない。チャンネル1つに下書き1つの制限 |
| slack-blocks-to-jsx（MIT、React） | Block Kit＋mrkdwnをReactで描画。「Slackに近い見た目」を売りにしている | 見た目の近さが§2の規約と衝突。Reactビルドが要る。承認の往復がない |
| block-kitchen（MIT、React） | ドラッグ＆ドロップのBlock Kit編集＋プレビュー（内部でslack-blocks-to-jsx） | 同上。編集ツールであってエージェント向けではない |
| mcp-slack-block-kit（MIT、Go） | Markdown→Block Kit JSON変換のMCP | プレビューがない。変換の部品として参考になる |

本ツールの差分は「エージェントの送信に**自動で割り込み**、文脈つきで見せ、判断を**エージェントに返す**」の3点。描画の忠実さで勝負しない。

---

## 2. 権利面で問題のない出し方

### 2.1 Slackのブランド規約（slack.com/terms-of-service/slack-brand、2026-10-06確認）

確認した条文の要旨:

- 「Slackの特徴的な『look and feel』や、Slackブランドの識別可能で固有の視覚要素を使用・模倣しないこと」
- 「Slackのロゴや類似の画像を、Slackが提供するブランドフォルダの例以外で使わないこと」
- 「Slackを名詞・動詞・複数形・所有格として使わない。Slack商標の後には一般名詞を置く」。製品名の一部に「Slack」を含めることは不可
- 「Slackが制作・支援していると示唆しないこと。曖昧なら『Slack Technologies, LLCが制作・提携・支援するものではない』と明記するよう求めることがある」
- 「Slackのスクリーンショットは教育・説明目的で可。リサイズ以外の改変は不可」

### 2.2 そこから導く規則

| 規則 | 理由 | 具体策 |
| --- | --- | --- |
| R1 製品名に「slack」を入れない | 商標規約 | `chat-tool-ui-preview`（現状のディレクトリ名）か、短く`msg-preview`／`post-preview`。ドメインも取らない |
| R2 Slackのロゴ・配色（紫 #4A154B等）・フォント（Lato）・アイコン・CSSを使わない | look and feel模倣の禁止 | 自前の中立な配色（グレー基調）。フォントはOSのシステムフォント。アイコンは文字か自作SVG |
| R3 画面は本ツール独自のチャット風レイアウトにし、画面内で特定サービスの名前や用語を使わない | 混同の回避。免責文は模倣の言い訳にならない（R9） | 画面の注記は「送信先サービスの画面ではなく、このツール独自の表示」の一文だけ。READMEに非提携の明記 |
| R4 Slackの公式画面（Block Kit Builder、Slackアプリの下書き）を**併用経路として案内**する | 本物を見たい人には本物を見せるのが最も正しく、権利上も安全 | `--open-official`でBlock Kit BuilderのURLを開く（JSONをURLに載せる公式機能）。Claude Slack MCP利用時は下書き作成を案内 |
| R5 絵文字はOSの絵文字フォントで描く。同梱しない | 画像セットの権利を持ち込まない | `:name:`→Unicodeの対応表は公開データ（Unicode CLDRの短縮名、またはiamcal/emoji-data MIT）。どうしても同梱するならNoto Emoji（OFL 1.1）。Twemoji（CC-BY 4.0）は帰属表示が要る |
| R6 SlackのBlock Kit JSONやmrkdwnの**仕様**に基づくパーサは自作してよい | 公開APIの仕様に従う実装は通常の互換実装 | 公式ドキュメントのみを参照し、Slackクライアントのコードや資産を取り込まない |
| R7 READMEに載せる比較画像でSlackの実画面を使うときは無改変 | スクリーンショット規約 | 使わないのが簡単。使うなら無改変で出典明記 |
| R8 他社OSSを使うなら許諾を確認して帰属表示する | MIT／Apache等の条件 | slack-blocks-to-jsxはMIT。使うのは**テストの比較対象**まで（§3.2） |
| R9 **画面の構成を写さない**。アバター・名前・時刻を並べたメッセージ行、色分けしたボタン、色付き背景のメンション、免責文で逃げる設計をしない | 「配色とフォントを変えれば中立」ではない。構成が同じなら模倣である。**出来事**: 初版（2026-10-06）は左にアバター、同じ行に太字の名前・「BOT」バッジ・時刻、緑と赤のボタン、水色のメンションという構成で、「Slackの画面ではない」と免責文を置いて出した。ユーザーの指摘で、構成そのものが写しであり、免責文は模倣の自認にすぎないと判明した | 投稿は左にアクセント線を持つ独自のカード。送信者は小さなラベル、時刻は出さない。ボタンは単色の輪郭線で「強調」「警告」は文字で添える。メンションは太字の文字。画面内に「Slack」「Block Kit」「mrkdwn」の語を出さない（テストで検査） |

Discord・Teamsにも同種の規約がある（未確認。対応時に確認する）。R1〜R3は他社にもそのまま当てはめる。

### 2.3 「実際の画面に近い」の読み替え

元の要望は「実際の画面に近い形で確認したい」。規約上、見た目の近さは追えないので、**近さを「書式と文脈の意味が同じに伝わること」で測る**。具体的には:

- 太字は太字に、引用は左線つきに、コードは等幅に、メンションは太字の文字に、ブロックの縦並びは同じ順に。ただし画面の構成は本ツール独自のもの（R9）
- 「スレッドへの返信」「チャンネルにも表示」「ボットとして投稿」「宛先」は、本文の上に文脈欄として明示
- 本物と違いうる箇所（Markdown変換、名前解決、カスタム絵文字）は警告欄に列挙

これで「送ってから驚く」という困りごとは解ける、というのが本書の立場。見た目まで本物が要る場面はR4の公式経路に回す。

---

## 3. 設計

### 3.1 構成（port-keeper-mcpに倣う）

| 層 | 役割 | 形 |
| --- | --- | --- |
| 核 | 入力（Markdown／mrkdwn／Block Kit）→ 内部の文書モデル（AST）→ HTML描画。警告の収集 | Goパッケージ `internal/{model,parse,render}` |
| CLI | `preview` コマンド。stdinかファイルからJSONを受け、HTMLを書き出し、ブラウザを開き、判断を待って結果をJSONで返す | `cmd/chat-preview` |
| MCPアダプタ | `preview_message` ツール（stdio）。エージェントが任意に呼ぶ。v0は開いて警告を返すまで | `chat-preview mcp`（`internal/mcpserver`。go-sdk v1.8.0） |
| クライアントアダプタ | Claude CodeのPreToolUseフック。Slack送信ツールの引数を核の入力に詰め替え、判断を`permissionDecision`に変換 | `internal/cli/hook_claude.go` |

言語はGo単体バイナリ（HTML・CSSは`embed`で同梱）。理由は、port-keeper-mcpと同じ配布手順（goreleaser、インストールスクリプト）を流用でき、Node環境なしで動くこと。TypeScript（React＋Storybook）も検討し一度は選んだが、利用側にNodeを要求する欠点が「部品をStorybookで管理できる」利点より大きいと判断して戻した（2026-10-06）。

Storybookの代わりに、`chat-preview gallery`コマンドで**書式要素ごとの見本をまとめた1枚のHTML**を出す。見本の入力は`internal/render/testdata/`に置き、ゴールデンテストの入力と兼ねる。見た目の中立性（R2・R3）はこのページで人が目視する。

描画の経路は2つで、同じテンプレートを使う:

- **静的HTML**（v0）: `html/template`で1ファイルを書き出す。JS不要、外部リソースなしで開ける
- **ローカルサーバー**（v1）: 同じHTMLに承認・修正・却下のフォームを足し、一回限りのサーバーが受ける

### 3.2 描画エンジンの選択

| 案 | 利点 | 欠点 |
| --- | --- | --- |
| A. 自前（Go側でパース→`html/template`。CSSは自作。標準Markdownだけgoldmark（MIT）を使う） | 依存が少ない、単体バイナリ、見た目を最初から中立に作れる（R2・R3）、警告収集を描画と一体で作れる | mrkdwn／Block Kitのパーサを書く手間（Block Kit主要7種＋mrkdwnで数百行規模の見込み。未計測） |
| B. slack-blocks-to-jsx（MIT）をビルドして同梱 | Block Kit全種（`rich_text`含む）の対応が最初からある。`unstyled`プロパティで既定CSSを外せるので、自前CSSを当てれば規約面はAと同等にできる | 既定の見た目が「Slackに近い」ことを売りにしており、自前CSSで覆う作業が要る。Node／Reactのビルド工程とNode依存（TSで作る場合）が増える。Markdown入力は別途変換が要る |

**A（決定）**。BはGoの単体バイナリに同梱するとReactのビルド工程が要る。Bはテストの比較対象（同じ入力で要素の有無・順序が一致するか）としてだけ使う。

### 3.3 入力の契約（核が受けるJSON）

```json
{
  "target": "slack",
  "format": "markdown | mrkdwn | blocks",
  "text": "...",                 // markdown / mrkdwn のとき
  "blocks": [ ... ],             // blocks のとき。text は通知用の代替文
  "context": {
    "channel_id": "C0123",
    "channel_name": "general",   // 任意。呼び出し側が知っていれば
    "thread_ts": "1700000000.000100",
    "reply_broadcast": false,
    "as": "user | bot",
    "sender_name": "Claude",     // 任意
    "names": { "U0123": "aquila", "C0123": "general" }  // 任意。メンション解決用
  },
  "source": "claude-slack-mcp | slack-web-api | manual"
}
```

- `format`は必須。推定しない（Markdownとmrkdwnは`*bold*`の意味が逆で、推定すると誤る）
- `source`は注記の出し分けに使う（`claude-slack-mcp`なら「実際の変換はSlack側で行われる」を出す）
- `target`は将来`discord`／`teams`を足す口。v1は`slack`のみ

### 3.4 判断の往復（2段階で作る）

**v0: 静的HTML＋端末で判断**

1. 核がHTMLを一時ファイル（本人のみ読める権限）に書き、既定ブラウザで開く
2. フックは`permissionDecision: "ask"`と、警告一覧を`permissionDecisionReason`に入れて返す
3. ユーザーはClaude Codeの通常の許可ダイアログで承認／拒否する。修正は拒否理由として書き、エージェントが書き直す
4. ツール実行後（PostToolUse）に一時ファイルを消す

利点: サーバー不要で部品が少ない。Claude Codeのフックは「ブラウザを開けない」とドキュメントにあるが、これは「フック自身が対話UIを持てない」の意味で、`open <file>`で別プロセスにブラウザを開かせることは可能と見込む（**未検証。最初に実機で確かめる**）。

**v1: 一回限りのローカルサーバー＋ページ上で判断**

1. 核が`127.0.0.1:0`（OSが空き番号を割り当てる。本ツールは番号を選ばない）で待ち受け、ページを開く
2. ページに「承認」「修正して承認」「却下（理由）」。修正は本文のテキストエリア
3. 判断が来たらサーバーを閉じ、結果JSONを標準出力に書く。フックは承認→`allow`（修正があれば`updatedInput`で本文を差し替え）、却下→`deny`＋理由
4. 制限時間（既定10分。Claude Codeのフック既定タイムアウト600秒に合わせる）を過ぎたら`ask`で端末に戻す

MCPツール`preview_message`はv1の流儀で判断を返す（ツール呼び出しがユーザーの判断まで待つ）。

### 3.5 Claude Codeアダプタの配線

```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "mcp__claude_ai_Slack__slack_send_message|mcp__claude_ai_Slack__slack_schedule_message",
      "hooks": [{ "type": "command", "command": "chat-preview hook claude", "timeout": 600 }]
    }]
  }
}
```

アダプタの仕事は、フックがstdinに渡すJSONの`tool_input`（`channel_id`、`message`、`thread_ts`、`reply_broadcast`）を§3.3の形に詰め替えること、核の結果を`hookSpecificOutput`に変換することの2つだけ。他のクライアントはアダプタを足す。

### 3.6 名前解決をしない理由と緩和

Slack APIに問い合わせるとトークンの保管と送信先への通信が生じ、G9（内容を外に出さない）と常駐なしの方針を壊す。代わりに:

- 呼び出し側が`context.names`で対応表を渡せる（Claude Codeのアダプタは、同じ会話で`slack_search_users`等の結果があってもそれを拾えないので、v1では渡さない）
- 解決できないIDは`@U0123`の形で描き、警告欄に「未解決のメンション: U0123」と列挙する

### 3.7 安全・プライバシー

- 待ち受けはloopbackのみ。CSRF対策として1回限りのトークンをURLに含める（v1）
- 一時HTMLは本人のみ読める権限で作り、判断後に削除。ログに本文を残さない
- Block Kitの`text`など投稿本文に指示文が仕込まれていても、描画はエスケープしてデータとして扱う。フックがエージェントに返す文は定型（「ユーザーが却下した。理由: …」）だけ
- 外部リソース（画像URL）はページから読み込まない。`image`ブロックはURLを文字で示し、サムネイルは出さない（投稿前に相手サーバーへアクセスが飛ぶのを避ける）

### 3.8 リポジトリ構成（案）

```
cmd/chat-preview/
internal/{model,parse,emoji,render,store,cli,mcpserver}/   # store: 一時ファイル（0600、1時間で掃除）
internal/render/assets/{page.html,style.css}   # embed
internal/render/testdata/                       # 見本兼ゴールデンテストの入力
docs/{DESIGN,ROADMAP}.md  README.md  SECURITY.md  openspec/
```

---

## 4. テスト方針

- パーサ: 公式ドキュメントの例文を入力にしたゴールデンテスト（期待HTMLを`testdata/`に持つ。`go test`、`-update`で更新）
- 見た目: `gallery`コマンドの1枚HTMLを人が目視する。CIでは要素の有無だけ検査する
- 警告: 未解決メンション、未知絵文字、長さ超過、不正ブロックが必ず列挙されること
- 往復: フックのstdin JSON→出力JSONをプロセスを起動せず関数で検証（port-keeper-mcpと同じく実ポートにbindしない。サーバーは`net.Listener`を差し替える）
- 比較: slack-blocks-to-jsx（MIT）の出力と要素の順序を突き合わせる（任意。CIには入れない）

## 5. マイルストーン

| 段 | 内容 | 完了の目安 |
| --- | --- | --- |
| M0 | 核: mrkdwn＋Block Kit主要7種→AST→静的HTML、警告欄。`gallery`に書式要素ごとの見本 | CLIでJSONを渡すとブラウザに出る。`gallery`で全要素が見える |
| M1 | Claude Codeフック（v0: `ask`で端末判断）。標準Markdown入力 | 実際の`slack_send_message`に割り込める |
| M2 | v1: ローカルサーバーでページ上の承認／修正／却下。MCPツール`preview_message` | 修正が`updatedInput`で反映される |
| M3 | 配布（goreleaser、インストールスクリプト）、README非提携表記 | — |

## 6. 既知のリスク

- Claude Slack MCPのMarkdown→Slack変換は非公開。表（table）や見出しの扱いが実物とずれる可能性がある。注記で逃げるが、ずれが大きければ「Markdownのまま見せる」に落とす
- フックからブラウザを開けるかは未検証（§3.4）
- Block Kitの`rich_text`はネストが深く、v1で全部は追えない。未対応要素は「未対応: 〇〇」と枠で出す

## 7. ユーザーに判断を求める事項

| # | 論点 | 選択肢 | 推奨 |
| --- | --- | --- | --- |
| D1 | 見た目の方針 | (a) 書式忠実・見た目は中立（§2.3） / (b) Slack風の見た目を追う | **(a)に決定（2026-10-06）**。(b)はブランド規約に反する |
| D2 | 判断の経路 | (a) v0（端末の許可ダイアログ）から始める / (b) 最初からv1（ページ上で承認・修正） | **(a)→(b)の順に決定（2026-10-06）**。v0でフック経路の実機検証を先に済ませる |
| D3 | 実装言語 | (a) Go単体バイナリ＋自前描画 / (b) TypeScript（React＋Storybook） | **(a)に決定（2026-10-06）**。一度(b)を選んだが、利用側にNodeを要求する欠点が大きいとして同日(a)へ戻した。見本管理は`gallery`で代替（§3.1） |
| D4 | v1の入力範囲 | (a) Slackのみ3形式 / (b) 最初からDiscordも | **(a)で進める（2026-10-06、担当側の判断）**。`target`の口だけ残す |
| D5 | 名前 | (a) `chat-tool-ui-preview`のまま / (b) 短い別名（`msg-preview`等） | **(a)で進める（2026-10-06、担当側の判断）**。「slack」を含めないことだけ守る |
