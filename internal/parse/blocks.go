package parse

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gridhra/chat-tool-ui-preview/internal/model"
)

// Block Kitの長さ上限（公式リファレンスの値）。
const (
	limitSectionText  = 3000
	limitSectionField = 2000
	limitHeader       = 150
	limitContextText  = 2000
	limitBlocks       = 50
)

type anyMap = map[string]any

// BlockKit はBlock Kit JSON（blocks配列）を内部モデルにする。
// 対応: section / header / divider / context / image / actions / rich_text（主要な部分）。未対応は枠だけ示し警告する。
func BlockKit(raw []json.RawMessage, o *Options) []model.Block {
	if len(raw) > limitBlocks {
		o.warn(model.WarnTooLong, "ブロック数 %d（送信先の上限は1メッセージ%d）", len(raw), limitBlocks)
	}
	out := make([]model.Block, 0, len(raw))
	for i, r := range raw {
		var b anyMap
		if err := json.Unmarshal(r, &b); err != nil || b == nil {
			o.warn(model.WarnInvalidBlock, "blocks[%d]: オブジェクトではない", i)
			out = append(out, model.Block{Kind: model.BlockUnsupported, What: "不正なブロック"})
			continue
		}
		out = append(out, parseOne(b, fmt.Sprintf("blocks[%d]", i), o))
	}
	return out
}

func str(m anyMap, k string) string {
	if s, ok := m[k].(string); ok {
		return s
	}
	return ""
}
func obj(m anyMap, k string) anyMap {
	if v, ok := m[k].(anyMap); ok {
		return v
	}
	return nil
}
func arr(m anyMap, k string) []any {
	if v, ok := m[k].([]any); ok {
		return v
	}
	return nil
}

func textObject(x any, where string, limit int, o *Options) []model.Block {
	m, ok := x.(anyMap)
	if !ok || m["text"] == nil {
		o.warn(model.WarnInvalidBlock, "%s: text オブジェクトが不正", where)
		return nil
	}
	t := str(m, "text")
	if limit > 0 && len([]rune(t)) > limit {
		o.warn(model.WarnTooLong, "%s: %d文字（上限 %d）", where, len([]rune(t)), limit)
	}
	if str(m, "type") == "plain_text" {
		withEmoji := true
		if e, ok := m["emoji"].(bool); ok {
			withEmoji = e
		}
		return []model.Block{model.Paragraph(PlainText(t, withEmoji, o))}
	}
	return MrkdwnBlocks(t, o)
}

// PlainText は plain_text（書式なし。絵文字だけ展開）を読む。
func PlainText(s string, withEmoji bool, o *Options) []model.Inline {
	if !withEmoji {
		return []model.Inline{model.Text(s)}
	}
	var out []model.Inline
	var buf strings.Builder
	src := []rune(s)
	for i := 0; i < len(src); {
		if src[i] == ':' {
			if m := emojiRe.FindStringSubmatch(string(src[i:])); m != nil {
				if buf.Len() > 0 {
					out = append(out, model.Text(buf.String()))
					buf.Reset()
				}
				out = append(out, Emoji(m[1], o))
				i += len([]rune(m[0]))
				continue
			}
		}
		buf.WriteRune(src[i])
		i++
	}
	if buf.Len() > 0 {
		out = append(out, model.Text(buf.String()))
	}
	return out
}

func element(x any, where string, o *Options) model.Element {
	m, ok := x.(anyMap)
	if !ok || str(m, "type") == "" {
		o.warn(model.WarnInvalidBlock, "%s: 要素が不正", where)
		return model.Element{Kind: model.ElementPlaceholder, What: "不正な要素"}
	}
	switch t := str(m, "type"); t {
	case "button":
		label := "(button)"
		if tx := obj(m, "text"); tx != nil && str(tx, "text") != "" {
			label = str(tx, "text")
		}
		style := str(m, "style")
		if style != "primary" && style != "danger" {
			style = ""
		}
		return model.Element{Kind: model.ElementButton, Text: label, Style: style, URL: str(m, "url")}
	case "image":
		return model.Element{Kind: model.ElementImage, URL: str(m, "image_url"), Alt: str(m, "alt_text")}
	default:
		ph := ""
		if p := obj(m, "placeholder"); p != nil {
			ph = str(p, "text")
		}
		return model.Element{Kind: model.ElementPlaceholder, What: t, Label: ph}
	}
}

func parseOne(b anyMap, where string, o *Options) model.Block {
	switch t := str(b, "type"); t {
	case "divider":
		return model.Block{Kind: model.BlockDivider}
	case "header":
		var children []model.Inline
		for _, blk := range textObject(b["text"], where, limitHeader, o) {
			if blk.Kind == model.BlockParagraph {
				children = append(children, blk.Children...)
			}
		}
		return model.Block{Kind: model.BlockHeading, Level: 1, Children: children}
	case "section":
		out := model.Block{Kind: model.BlockSection}
		if b["text"] != nil {
			out.Text = textObject(b["text"], where+".text", limitSectionText, o)
		}
		if fs := arr(b, "fields"); fs != nil {
			if len(fs) > 10 {
				o.warn(model.WarnTooLong, "%s.fields: %d個（上限10）", where, len(fs))
			}
			for i, f := range fs {
				out.Fields = append(out.Fields, textObject(f, fmt.Sprintf("%s.fields[%d]", where, i), limitSectionField, o))
			}
		}
		if b["accessory"] != nil {
			e := element(b["accessory"], where+".accessory", o)
			out.Accessory = &e
		}
		if out.Text == nil && out.Fields == nil {
			o.warn(model.WarnInvalidBlock, "%s: section に text も fields も無い", where)
		}
		return out
	case "context":
		els := arr(b, "elements")
		if len(els) > 10 {
			o.warn(model.WarnTooLong, "%s.elements: %d個（上限10）", where, len(els))
		}
		out := model.Block{Kind: model.BlockContext}
		for i, e := range els {
			w := fmt.Sprintf("%s.elements[%d]", where, i)
			if m, ok := e.(anyMap); ok && (str(m, "type") == "mrkdwn" || str(m, "type") == "plain_text") {
				out.Elements = append(out.Elements, model.ContextElement{Text: textObject(e, w, limitContextText, o)})
				continue
			}
			el := element(e, w, o)
			out.Elements = append(out.Elements, model.ContextElement{Element: &el})
		}
		return out
	case "image":
		title := ""
		if ti := obj(b, "title"); ti != nil {
			title = str(ti, "text")
		}
		return model.Block{Kind: model.BlockImage, URL: str(b, "image_url"), Alt: str(b, "alt_text"), Title: title}
	case "actions":
		els := arr(b, "elements")
		if len(els) > 25 {
			o.warn(model.WarnTooLong, "%s.elements: %d個（上限25）", where, len(els))
		}
		out := model.Block{Kind: model.BlockActions}
		for i, e := range els {
			out.Actions = append(out.Actions, element(e, fmt.Sprintf("%s.elements[%d]", where, i), o))
		}
		return out
	case "rich_text":
		return model.Block{Kind: model.BlockSection, Text: richText(arr(b, "elements"), where, o)}
	case "":
		o.warn(model.WarnInvalidBlock, "%s: type が無い", where)
		return model.Block{Kind: model.BlockUnsupported, What: "不正なブロック"}
	default:
		o.warn(model.WarnUnsupported, "%s: 未対応のブロック種 %q（枠だけ表示）", where, t)
		return model.Block{Kind: model.BlockUnsupported, What: t}
	}
}

// richText は rich_text ブロック。section / list / quote / preformatted と、その中の
// text / link / emoji / user / channel / usergroup / broadcast に対応。
func richText(elements []any, where string, o *Options) []model.Block {
	var out []model.Block
	for i, e := range elements {
		el, ok := e.(anyMap)
		if !ok {
			continue
		}
		w := fmt.Sprintf("%s.elements[%d]", where, i)
		switch t := str(el, "type"); t {
		case "rich_text_section":
			out = append(out, model.Paragraph(richInline(arr(el, "elements"), w, o)))
		case "rich_text_quote":
			out = append(out, model.Block{Kind: model.BlockQuote, Blocks: []model.Block{model.Paragraph(richInline(arr(el, "elements"), w, o))}})
		case "rich_text_preformatted":
			var sb strings.Builder
			for _, x := range arr(el, "elements") {
				if m, ok := x.(anyMap); ok {
					sb.WriteString(str(m, "text"))
				}
			}
			out = append(out, model.Block{Kind: model.BlockCode, Value: sb.String()})
		case "rich_text_list":
			blk := model.Block{Kind: model.BlockList, Ordered: str(el, "style") == "ordered"}
			for j, it := range arr(el, "elements") {
				m, _ := it.(anyMap)
				blk.Items = append(blk.Items, []model.Block{model.Paragraph(richInline(arr(m, "elements"), fmt.Sprintf("%s[%d]", w, j), o))})
			}
			out = append(out, blk)
		default:
			o.warn(model.WarnUnsupported, "%s: 未対応の rich_text 要素 %q", w, t)
		}
	}
	return out
}

func richInline(elements []any, where string, o *Options) []model.Inline {
	var out []model.Inline
	for _, e := range elements {
		m, ok := e.(anyMap)
		if !ok {
			continue
		}
		var node model.Inline
		switch t := str(m, "type"); t {
		case "text":
			node = model.Text(str(m, "text"))
		case "link":
			label := str(m, "text")
			if label == "" {
				label = str(m, "url")
			}
			node = model.Inline{Kind: model.InlineLink, URL: str(m, "url"), Children: []model.Inline{model.Text(label)}}
		case "emoji":
			if u := str(m, "unicode"); u != "" {
				var sb strings.Builder
				for _, h := range strings.Split(u, "-") {
					if cp, err := strconv.ParseInt(h, 16, 32); err == nil {
						sb.WriteRune(rune(cp))
					}
				}
				node = model.Inline{Kind: model.InlineEmoji, Name: str(m, "name"), Unicode: sb.String()}
			} else {
				node = Emoji(str(m, "name"), o)
			}
		case "user":
			node = o.mention(model.MentionUser, "ユーザーメンション", str(m, "user_id"), "")
		case "channel":
			node = o.mention(model.MentionChannel, "チャンネル参照", str(m, "channel_id"), "")
		case "usergroup":
			node = o.mention(model.MentionSubteam, "ユーザーグループ", str(m, "usergroup_id"), "")
		case "broadcast":
			r := str(m, "range")
			o.broadcast(r)
			kind := model.MentionHere
			if r == "channel" {
				kind = model.MentionChannelAll
			} else if r == "everyone" {
				kind = model.MentionEveryone
			}
			node = model.Inline{Kind: model.InlineMention, Mention: kind, Label: r, Resolved: true}
		default:
			o.warn(model.WarnUnsupported, "%s: 未対応の rich_text 内要素 %q", where, t)
			continue
		}
		if st := obj(m, "style"); st != nil {
			if b, _ := st["code"].(bool); b && node.Kind == model.InlineText {
				node = model.Inline{Kind: model.InlineCode, Value: node.Value}
			}
			for _, k := range []struct {
				key  string
				kind model.InlineKind
			}{{"bold", model.InlineBold}, {"italic", model.InlineItalic}, {"strike", model.InlineStrike}} {
				if b, _ := st[k.key].(bool); b {
					node = model.Inline{Kind: k.kind, Children: []model.Inline{node}}
				}
			}
		}
		out = append(out, node)
	}
	return out
}
