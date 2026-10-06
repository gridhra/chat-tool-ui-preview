// Package render は文書モデルを静的HTML（JS不要・外部リソースなし）にする。
package render

import (
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"strings"
	"time"

	"github.com/gridhra/chat-tool-ui-preview/internal/model"
)

//go:embed assets/style.css
var css string

//go:embed assets/page.html
var pageSrc string

var page = template.Must(template.New("page").Parse(pageSrc))

var formatLabel = map[model.Format]string{
	model.FormatMarkdown: "標準Markdown",
	model.FormatMrkdwn:   "送信先独自の記法",
	model.FormatBlocks:   "ブロック形式",
}

type pageData struct {
	Title          string
	CSS            template.CSS
	Gallery        bool
	Items          []GalleryItem
	FormatLabel    string
	Channel        string
	ChannelSub     string
	ThreadTS       string
	ReplyBroadcast bool
	IsBot          bool
	Sender         string
	SenderNamed    bool
	Body           template.HTML
	Warnings       []model.Warning
	Raw            string
}

// Options は描画の付帯情報。
type Options struct {
	Raw string // 元の入力。空なら折りたたみを出さない
}

// Page は1投稿のプレビューページ全体。
func Page(doc model.Document, opt Options) ([]byte, error) {
	ctx := doc.Context
	sender := ctx.SenderName
	isBot := ctx.As == "bot"
	if sender == "" && isBot {
		sender = "（名前未指定）"
	}
	d := pageData{
		Title:          "送信前の確認",
		CSS:            template.CSS(css),
		FormatLabel:    formatLabel[doc.Format],
		ThreadTS:       ctx.ThreadTS,
		ReplyBroadcast: ctx.ReplyBroadcast,
		IsBot:          isBot,
		Sender:         sender,
		SenderNamed:    ctx.SenderName != "",
		Body:           template.HTML(Blocks(doc.Blocks)),
		Warnings:       doc.Warnings,
		Raw:            opt.Raw,
	}
	switch {
	case ctx.ChannelName != "":
		d.Channel = "#" + ctx.ChannelName
		d.ChannelSub = ctx.ChannelID
	case strings.HasPrefix(ctx.ChannelID, "U") || strings.HasPrefix(ctx.ChannelID, "W") || strings.HasPrefix(ctx.ChannelID, "D"):
		// ユーザーIDやDM IDが宛先ならダイレクトメッセージ
		if name := ctx.Names[ctx.ChannelID]; name != "" {
			d.Channel = "@" + name
		} else {
			d.Channel = ctx.ChannelID
		}
		d.ChannelSub = "ダイレクトメッセージ"
	case ctx.ChannelID != "":
		d.Channel = ctx.ChannelID
	default:
		d.Channel = "（未指定）"
	}
	var buf bytes.Buffer
	if err := page.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GalleryItem は見本ページの1項目。
type GalleryItem struct {
	Name string
	Body template.HTML
}

// GalleryPage は書式要素ごとの見本を1枚にまとめる（Storybookの代替）。
func GalleryPage(items []GalleryItem) ([]byte, error) {
	var buf bytes.Buffer
	err := page.Execute(&buf, pageData{Title: "書式要素の見本", CSS: template.CSS(css), Gallery: true, Items: items})
	return buf.Bytes(), err
}

// Blocks はブロック列のHTML断片。
func Blocks(bs []model.Block) string {
	var sb strings.Builder
	for _, b := range bs {
		writeBlock(&sb, b)
	}
	return sb.String()
}

func esc(s string) string { return html.EscapeString(s) }

func writeBlock(sb *strings.Builder, b model.Block) {
	switch b.Kind {
	case model.BlockParagraph:
		sb.WriteString(`<p class="p">`)
		writeInlines(sb, b.Children)
		sb.WriteString(`</p>`)
	case model.BlockHeading:
		cls := "h"
		if b.Level >= 2 {
			cls += " h2"
		}
		fmt.Fprintf(sb, `<p class="%s">`, cls)
		writeInlines(sb, b.Children)
		sb.WriteString(`</p>`)
	case model.BlockQuote:
		sb.WriteString(`<blockquote class="quote">`)
		sb.WriteString(Blocks(b.Blocks))
		sb.WriteString(`</blockquote>`)
	case model.BlockCode:
		sb.WriteString(`<pre class="pre"`)
		if b.Lang != "" {
			fmt.Fprintf(sb, ` data-lang="%s"`, esc(b.Lang))
		}
		sb.WriteString(`>` + esc(b.Value) + `</pre>`)
	case model.BlockList:
		tag := "ul"
		if b.Ordered {
			tag = "ol"
		}
		fmt.Fprintf(sb, `<%s class="list">`, tag)
		for _, it := range b.Items {
			sb.WriteString(`<li>` + Blocks(it) + `</li>`)
		}
		fmt.Fprintf(sb, `</%s>`, tag)
	case model.BlockTable:
		sb.WriteString(`<table class="table"><thead><tr>`)
		for _, c := range b.Header {
			sb.WriteString(`<th>`)
			writeInlines(sb, c)
			sb.WriteString(`</th>`)
		}
		sb.WriteString(`</tr></thead><tbody>`)
		for _, r := range b.Rows {
			sb.WriteString(`<tr>`)
			for _, c := range r {
				sb.WriteString(`<td>`)
				writeInlines(sb, c)
				sb.WriteString(`</td>`)
			}
			sb.WriteString(`</tr>`)
		}
		sb.WriteString(`</tbody></table>`)
	case model.BlockDivider:
		sb.WriteString(`<hr class="hr">`)
	case model.BlockSection:
		sb.WriteString(`<div class="section">`)
		sb.WriteString(Blocks(b.Text))
		if len(b.Fields) > 0 {
			sb.WriteString(`<div class="fields">`)
			for _, f := range b.Fields {
				sb.WriteString(`<div>` + Blocks(f) + `</div>`)
			}
			if len(b.Fields)%2 == 1 {
				sb.WriteString(`<div></div>`)
			}
			sb.WriteString(`</div>`)
		}
		if b.Accessory != nil {
			sb.WriteString(`<div class="attach controls">`)
			writeElement(sb, *b.Accessory)
			sb.WriteString(`</div>`)
		}
		sb.WriteString(`</div>`)
	case model.BlockContext:
		sb.WriteString(`<div class="aside">`)
		for i, e := range b.Elements {
			if i > 0 {
				sb.WriteString(` · `)
			}
			if e.Element != nil {
				writeElement(sb, *e.Element)
			} else {
				sb.WriteString(`<span>` + Blocks(e.Text) + `</span>`)
			}
		}
		sb.WriteString(`</div>`)
	case model.BlockImage:
		// 画像は読み込まない（投稿前に相手サーバーへアクセスが飛ぶのを避ける）。URLと代替文だけ示す
		sb.WriteString(`<div class="media">`)
		if b.Title != "" {
			sb.WriteString(`<b>` + esc(b.Title) + `</b><br>`)
		}
		fmt.Fprintf(sb, `画像 <span class="link">%s</span>`, esc(b.URL))
		if b.Alt != "" {
			sb.WriteString(`<br>代替文: ` + esc(b.Alt))
		}
		sb.WriteString(`</div>`)
	case model.BlockActions:
		sb.WriteString(`<div class="controls">`)
		for _, e := range b.Actions {
			writeElement(sb, e)
		}
		sb.WriteString(`</div>`)
	case model.BlockUnsupported:
		sb.WriteString(`<div class="unsupported">未対応のブロック: ` + esc(b.What) + `（本物では何かが表示される。内容はこのプレビューでは確認できない）</div>`)
	}
}

func writeElement(sb *strings.Builder, e model.Element) {
	switch e.Kind {
	case model.ElementButton:
		// 色で種類を示さず、文字で添える（特定サービスのボタン配色を写さない）
		note := map[string]string{"primary": "強調", "danger": "警告"}[e.Style]
		fmt.Fprintf(sb, `<span class="ctl" title="%s">ボタン「%s」`, esc(e.URL), esc(e.Text))
		if note != "" {
			sb.WriteString(`<small>` + note + `</small>`)
		}
		sb.WriteString(`</span>`)
	case model.ElementImage:
		label := e.Alt
		if label == "" {
			label = e.URL
		}
		fmt.Fprintf(sb, `<span class="link" title="%s">画像 %s</span>`, esc(e.URL), esc(label))
	case model.ElementPlaceholder:
		label := e.Label
		if label == "" {
			label = e.What
		}
		fmt.Fprintf(sb, `<span class="ctl menu" title="%s">%s<small>%s</small></span>`, esc(e.What), esc(label), esc(e.What))
	}
}

var mentionLabel = map[model.MentionKind]string{model.MentionHere: "@here", model.MentionChannelAll: "@channel", model.MentionEveryone: "@everyone"}

func writeInlines(sb *strings.Builder, ins []model.Inline) {
	for _, in := range ins {
		switch in.Kind {
		case model.InlineText:
			sb.WriteString(esc(in.Value))
		case model.InlineBold:
			sb.WriteString(`<strong>`)
			writeInlines(sb, in.Children)
			sb.WriteString(`</strong>`)
		case model.InlineItalic:
			sb.WriteString(`<em>`)
			writeInlines(sb, in.Children)
			sb.WriteString(`</em>`)
		case model.InlineStrike:
			sb.WriteString(`<s>`)
			writeInlines(sb, in.Children)
			sb.WriteString(`</s>`)
		case model.InlineCode:
			sb.WriteString(`<code class="code">` + esc(in.Value) + `</code>`)
		case model.InlineLink:
			// プレビューからは遷移しない（投稿前に相手サーバーへアクセスが飛ぶのを避ける）。href を持たせず、URLは title に出す
			fmt.Fprintf(sb, `<span class="link" title="%s">`, esc(in.URL))
			writeInlines(sb, in.Children)
			sb.WriteString(`</span>`)
		case model.InlineMention:
			label, ok := mentionLabel[in.Mention]
			if !ok {
				if in.Mention == model.MentionChannel {
					label = "#" + in.Label
				} else {
					label = "@" + in.Label
				}
			}
			cls, title := "mention", in.ID
			if !in.Resolved {
				cls += " unresolved"
				title = "未解決: " + in.ID
			}
			fmt.Fprintf(sb, `<span class="%s" title="%s">%s</span>`, cls, esc(title), esc(label))
		case model.InlineEmoji:
			if in.Unicode != "" {
				fmt.Fprintf(sb, `<span title=":%s:">%s</span>`, esc(in.Name), esc(in.Unicode))
			} else {
				fmt.Fprintf(sb, `<span class="emoji-unknown" title="未知の絵文字">:%s:</span>`, esc(in.Name))
			}
		case model.InlineDate:
			txt := in.Fallback
			if txt == "" {
				txt = time.Unix(in.Timestamp, 0).Format("2006-01-02 15:04")
			}
			fmt.Fprintf(sb, `<span class="date" title="日付書式 %s / 受け手のタイムゾーンで表示される">%s</span>`, esc(in.Format), esc(txt))
		case model.InlineBreak:
			sb.WriteString(`<br>`)
		}
	}
}
