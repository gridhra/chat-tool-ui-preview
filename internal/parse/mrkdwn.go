// Package parse は3つの入力形式を内部モデルに変換する。
package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/gridhra/chat-tool-ui-preview/internal/emoji"
	"github.com/gridhra/chat-tool-ui-preview/internal/model"
)

// Options はパース中に共有する状態（名前の対応表と警告の蓄積先）。
type Options struct {
	Names    map[string]string
	Warnings *[]model.Warning
}

func (o *Options) warn(code model.WarningCode, format string, args ...any) {
	*o.Warnings = append(*o.Warnings, model.Warning{Code: code, Message: fmt.Sprintf(format, args...)})
}

// MrkdwnBlocks はSlack記法（mrkdwn）の本文をブロック列にする。
// 公式の書式仕様（docs.slack.dev/messaging/formatting-message-text）に基づく。
func MrkdwnBlocks(src string, o *Options) []model.Block {
	var out []model.Block
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	i := 0
	for i < len(lines) {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "```"):
			rest := line[3:]
			if j := strings.Index(rest, "```"); j >= 0 {
				out = append(out, model.Block{Kind: model.BlockCode, Value: rest[:j]})
				i++
				continue
			}
			var buf []string
			if rest != "" {
				buf = append(buf, rest)
			}
			i++
			for i < len(lines) && !strings.Contains(lines[i], "```") {
				buf = append(buf, lines[i])
				i++
			}
			if i < len(lines) {
				if before := lines[i][:strings.Index(lines[i], "```")]; before != "" {
					buf = append(buf, before)
				}
				i++
			}
			out = append(out, model.Block{Kind: model.BlockCode, Value: strings.Join(buf, "\n")})
		case strings.HasPrefix(line, ">"):
			var buf []string
			for i < len(lines) && strings.HasPrefix(lines[i], ">") {
				buf = append(buf, strings.TrimPrefix(strings.TrimPrefix(lines[i], ">"), " "))
				i++
			}
			out = append(out, model.Block{Kind: model.BlockQuote, Blocks: MrkdwnBlocks(strings.Join(buf, "\n"), o)})
		default:
			var children []model.Inline
			first := true
			for i < len(lines) && !strings.HasPrefix(lines[i], "```") && !strings.HasPrefix(lines[i], ">") {
				l := lines[i]
				i++
				if l == "" {
					break
				}
				if !first {
					children = append(children, model.Inline{Kind: model.InlineBreak})
				}
				first = false
				children = append(children, MrkdwnInline(l, o)...)
			}
			if len(children) > 0 {
				out = append(out, model.Paragraph(children))
			}
		}
	}
	return out
}

var (
	emojiRe = regexp.MustCompile(`^:([a-zA-Z0-9_+\-]+(?:::skin-tone-\d)?):`)
	urlRe   = regexp.MustCompile(`^https?://[^\s<>]+`)
)

func isWordStart(src []rune, pos int) bool {
	return pos == 0 || strings.ContainsRune(" \t([{\"'>", src[pos-1])
}
func isWordEnd(src []rune, pos int) bool {
	return pos >= len(src) || strings.ContainsRune(" \t.,!?;:)]}\"'<", src[pos])
}

// MrkdwnInline は1行分の行内要素を読む。
func MrkdwnInline(s string, o *Options) []model.Inline {
	src := []rune(s)
	var out []model.Inline
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			out = append(out, model.Text(buf.String()))
			buf.Reset()
		}
	}
	n := len(src)
	for i := 0; i < n; {
		ch := src[i]
		if ch == '<' {
			if close := indexRune(src, '>', i+1); close > i {
				if tok, ok := angleToken(string(src[i+1:close]), o); ok {
					flush()
					out = append(out, tok)
					i = close + 1
					continue
				}
			}
		}
		if ch == '`' {
			if close := indexRune(src, '`', i+1); close > i+1 {
				flush()
				out = append(out, model.Inline{Kind: model.InlineCode, Value: string(src[i+1 : close])})
				i = close + 1
				continue
			}
		}
		if (ch == '*' || ch == '_' || ch == '~') && isWordStart(src, i) && i+1 < n && src[i+1] != ' ' {
			if close := findClosing(src, ch, i+1); close > i+1 {
				flush()
				kind := map[rune]model.InlineKind{'*': model.InlineBold, '_': model.InlineItalic, '~': model.InlineStrike}[ch]
				out = append(out, model.Inline{Kind: kind, Children: MrkdwnInline(string(src[i+1:close]), o)})
				i = close + 1
				continue
			}
		}
		if ch == ':' {
			if m := emojiRe.FindStringSubmatch(string(src[i:])); m != nil {
				flush()
				out = append(out, Emoji(m[1], o))
				i += len([]rune(m[0]))
				continue
			}
		}
		if ch == 'h' && isWordStart(src, i) {
			if m := urlRe.FindString(string(src[i:])); m != "" {
				flush()
				url := strings.TrimRight(m, ".,;:!?)")
				out = append(out, model.Inline{Kind: model.InlineLink, URL: url, Children: []model.Inline{model.Text(url)}})
				i += len([]rune(url))
				continue
			}
		}
		buf.WriteRune(ch)
		i++
	}
	flush()
	return out
}

func indexRune(src []rune, r rune, from int) int {
	for j := from; j < len(src); j++ {
		if src[j] == r {
			return j
		}
	}
	return -1
}

func findClosing(src []rune, ch rune, from int) int {
	for j := from; j < len(src); j++ {
		if src[j] == ch && src[j-1] != ' ' && isWordEnd(src, j+1) {
			return j
		}
	}
	return -1
}

// Emoji は短縮名から絵文字要素を作り、未知なら警告する。
func Emoji(name string, o *Options) model.Inline {
	u := emoji.Lookup(name)
	if u == "" {
		o.warn(model.WarnUnknownEmoji, "未知の絵文字: :%s:（ワークスペース独自の絵文字か、名前の誤り）", name)
	}
	return model.Inline{Kind: model.InlineEmoji, Name: name, Unicode: u}
}

var (
	userRe    = regexp.MustCompile(`^@([UW][A-Z0-9]+)(?:\|(.*))?$`)
	channelRe = regexp.MustCompile(`^#([CG][A-Z0-9]+)(?:\|(.*))?$`)
	subteamRe = regexp.MustCompile(`^!subteam\^([A-Z0-9]+)(?:\|(.*))?$`)
	specialRe = regexp.MustCompile(`^!(here|channel|everyone)(?:\|.*)?$`)
	dateRe    = regexp.MustCompile(`^!date\^(\d+)\^([^|^]+)(?:\^[^|]*)?(?:\|(.*))?$`)
	linkRe    = regexp.MustCompile(`^(https?://[^|]+|mailto:[^|]+)(?:\|(.*))?$`)
)

func (o *Options) mention(kind model.MentionKind, what, id, label string) model.Inline {
	if label == "" {
		label = o.Names[id]
	}
	if label == "" {
		o.warn(model.WarnUnresolvedMention, "未解決の%s: %s（表示名は送信先サービスに問い合わせないと分からない）", what, id)
		return model.Inline{Kind: model.InlineMention, Mention: kind, ID: id, Label: id}
	}
	return model.Inline{Kind: model.InlineMention, Mention: kind, ID: id, Label: label, Resolved: true}
}

// broadcast は @here / @channel / @everyone の警告。送信前に最も見落としやすい事故なので必ず出す。
func (o *Options) broadcast(name string) {
	scope := map[string]string{"here": "チャンネルにいるオンラインの全員", "channel": "チャンネルの全員（オフラインも含む）", "everyone": "ワークスペースの全員"}[name]
	o.warn(model.WarnBroadcast, "@%s が含まれる。%sに通知が飛ぶ", name, scope)
}

func angleToken(inner string, o *Options) (model.Inline, bool) {
	if m := userRe.FindStringSubmatch(inner); m != nil {
		return o.mention(model.MentionUser, "ユーザーメンション", m[1], m[2]), true
	}
	if m := channelRe.FindStringSubmatch(inner); m != nil {
		return o.mention(model.MentionChannel, "チャンネル参照", m[1], m[2]), true
	}
	if m := subteamRe.FindStringSubmatch(inner); m != nil {
		return o.mention(model.MentionSubteam, "ユーザーグループ", m[1], m[2]), true
	}
	if m := specialRe.FindStringSubmatch(inner); m != nil {
		kind := map[string]model.MentionKind{"here": model.MentionHere, "channel": model.MentionChannelAll, "everyone": model.MentionEveryone}[m[1]]
		o.broadcast(m[1])
		return model.Inline{Kind: model.InlineMention, Mention: kind, Label: m[1], Resolved: true}, true
	}
	if m := dateRe.FindStringSubmatch(inner); m != nil {
		ts, _ := strconv.ParseInt(m[1], 10, 64)
		return model.Inline{Kind: model.InlineDate, Timestamp: ts, Format: m[2], Fallback: m[3]}, true
	}
	if m := linkRe.FindStringSubmatch(inner); m != nil {
		children := []model.Inline{model.Text(m[1])}
		if m[2] != "" {
			children = MrkdwnInline(m[2], o)
		}
		return model.Inline{Kind: model.InlineLink, URL: m[1], Children: children}, true
	}
	return model.Inline{}, false
}
