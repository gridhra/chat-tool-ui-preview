package parse

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"github.com/gridhra/chat-tool-ui-preview/internal/model"
)

const limitMarkdown = 5000

// Markdown は標準Markdown（Claude CodeのSlack連携 `slack_send_message` の `message`）を内部モデルにする。
// goldmark（MIT）でASTにし、テキスト節の中の <@U..> や :emoji: はSlack記法として読む。
// 実際の変換はSlack側で行われ非公開なので、表・見出しの見え方は実物とずれうる。
func Markdown(src string, o *Options) []model.Block {
	if n := len([]rune(src)); n > limitMarkdown {
		o.warn(model.WarnTooLong, "本文 %d文字（送信先の上限は1要素%d）", n, limitMarkdown)
	}
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	source := []byte(src)
	doc := md.Parser().Parse(text.NewReader(source))
	var out []model.Block
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		out = append(out, mdBlock(c, source, o)...)
	}
	return out
}

func mdBlock(n ast.Node, src []byte, o *Options) []model.Block {
	switch v := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		return []model.Block{model.Paragraph(mdInlines(n, src, o))}
	case *ast.Heading:
		return []model.Block{{Kind: model.BlockHeading, Level: v.Level, Children: mdInlines(n, src, o)}}
	case *ast.Blockquote:
		return []model.Block{{Kind: model.BlockQuote, Blocks: mdChildren(n, src, o)}}
	case *ast.FencedCodeBlock:
		return []model.Block{{Kind: model.BlockCode, Value: linesText(v.Lines(), src), Lang: string(v.Language(src))}}
	case *ast.CodeBlock:
		return []model.Block{{Kind: model.BlockCode, Value: linesText(v.Lines(), src)}}
	case *ast.List:
		blk := model.Block{Kind: model.BlockList, Ordered: v.IsOrdered()}
		for li := n.FirstChild(); li != nil; li = li.NextSibling() {
			blk.Items = append(blk.Items, mdChildren(li, src, o))
		}
		return []model.Block{blk}
	case *ast.ThematicBreak:
		return []model.Block{{Kind: model.BlockDivider}}
	case *east.Table:
		blk := model.Block{Kind: model.BlockTable}
		for row := n.FirstChild(); row != nil; row = row.NextSibling() {
			var cells [][]model.Inline
			for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
				cells = append(cells, mdInlines(cell, src, o))
			}
			if _, isHead := row.(*east.TableHeader); isHead {
				blk.Header = cells
			} else {
				blk.Rows = append(blk.Rows, cells)
			}
		}
		return []model.Block{blk}
	case *ast.HTMLBlock:
		return []model.Block{model.Paragraph([]model.Inline{model.Text(linesText(v.Lines(), src))})}
	default:
		o.warn(model.WarnUnsupported, "Markdownの %q は未対応", n.Kind().String())
		return nil
	}
}

func mdChildren(n ast.Node, src []byte, o *Options) []model.Block {
	var out []model.Block
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		out = append(out, mdBlock(c, src, o)...)
	}
	return out
}

func linesText(lines *text.Segments, src []byte) string {
	var b []byte
	for i := 0; i < lines.Len(); i++ {
		s := lines.At(i)
		b = append(b, s.Value(src)...)
	}
	// 末尾の改行は落とす
	for len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return string(b)
}

func mdInlines(n ast.Node, src []byte, o *Options) []model.Inline {
	var out []model.Inline
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			for _, in := range MrkdwnInline(string(v.Segment.Value(src)), o) {
				out = append(out, stripEmphasis(in))
			}
			if v.SoftLineBreak() {
				out = append(out, model.Text("\n"))
			} else if v.HardLineBreak() {
				out = append(out, model.Inline{Kind: model.InlineBreak})
			}
		case *ast.String:
			out = append(out, model.Text(string(v.Value)))
		case *ast.Emphasis:
			kind := model.InlineItalic
			if v.Level >= 2 {
				kind = model.InlineBold
			}
			out = append(out, model.Inline{Kind: kind, Children: mdInlines(c, src, o)})
		case *east.Strikethrough:
			out = append(out, model.Inline{Kind: model.InlineStrike, Children: mdInlines(c, src, o)})
		case *ast.CodeSpan:
			var b []byte
			for t := c.FirstChild(); t != nil; t = t.NextSibling() {
				if tt, ok := t.(*ast.Text); ok {
					b = append(b, tt.Segment.Value(src)...)
				}
			}
			out = append(out, model.Inline{Kind: model.InlineCode, Value: string(b)})
		case *ast.Link:
			out = append(out, model.Inline{Kind: model.InlineLink, URL: string(v.Destination), Children: mdInlines(c, src, o)})
		case *ast.AutoLink:
			u := string(v.URL(src))
			out = append(out, model.Inline{Kind: model.InlineLink, URL: u, Children: []model.Inline{model.Text(string(v.Label(src)))}})
		case *ast.Image:
			out = append(out, model.Inline{Kind: model.InlineLink, URL: string(v.Destination), Children: mdInlines(c, src, o)})
		case *ast.RawHTML:
			seg := v.Segments.At(0)
			out = append(out, model.Text(string(seg.Value(src))))
		default:
			o.warn(model.WarnUnsupported, "Markdownのインライン %q は未対応", c.Kind().String())
		}
	}
	return out
}

// Markdownのテキスト節では *x* は goldmark が既に処理している。mrkdwnパーサの強調は文字に戻し、
// トークン（メンション・絵文字・リンク）だけを活かす。
func stripEmphasis(in model.Inline) model.Inline {
	switch in.Kind {
	case model.InlineBold, model.InlineItalic, model.InlineStrike:
		mark := map[model.InlineKind]string{model.InlineBold: "*", model.InlineItalic: "_", model.InlineStrike: "~"}[in.Kind]
		return model.Text(mark + Flatten(in.Children) + mark)
	}
	return in
}

// Flatten は行内要素を素の文字列に戻す（長さ検査や代替表示に使う）。
func Flatten(ins []model.Inline) string {
	var s string
	for _, in := range ins {
		switch in.Kind {
		case model.InlineText, model.InlineCode:
			s += in.Value
		case model.InlineBold, model.InlineItalic, model.InlineStrike, model.InlineLink:
			s += Flatten(in.Children)
		case model.InlineMention:
			s += "@" + in.Label
		case model.InlineEmoji:
			if in.Unicode != "" {
				s += in.Unicode
			} else {
				s += ":" + in.Name + ":"
			}
		case model.InlineDate:
			if in.Fallback != "" {
				s += in.Fallback
			} else {
				s += in.Format
			}
		case model.InlineBreak:
			s += "\n"
		}
	}
	return s
}
