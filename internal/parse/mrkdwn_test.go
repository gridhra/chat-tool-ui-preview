package parse

import (
	"reflect"
	"testing"

	"github.com/gridhra/chat-tool-ui-preview/internal/model"
)

func opts(names map[string]string) *Options {
	var ws []model.Warning
	return &Options{Names: names, Warnings: &ws}
}

func TestMrkdwnInlineEmphasis(t *testing.T) {
	got := MrkdwnInline("*b* _i_ ~s~ `c`", opts(nil))
	want := []model.Inline{
		{Kind: model.InlineBold, Children: []model.Inline{model.Text("b")}}, model.Text(" "),
		{Kind: model.InlineItalic, Children: []model.Inline{model.Text("i")}}, model.Text(" "),
		{Kind: model.InlineStrike, Children: []model.Inline{model.Text("s")}}, model.Text(" "),
		{Kind: model.InlineCode, Value: "c"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if got := MrkdwnInline("a*b*c", opts(nil)); !reflect.DeepEqual(got, []model.Inline{model.Text("a*b*c")}) {
		t.Fatalf("mid-word asterisk should stay text, got %+v", got)
	}
}

func TestMrkdwnInlineLinks(t *testing.T) {
	got := MrkdwnInline("<https://x.io|X> https://y.io.", opts(nil))
	if len(got) != 4 || got[0].Kind != model.InlineLink || got[0].URL != "https://x.io" || Flatten(got[0].Children) != "X" {
		t.Fatalf("labelled link: %+v", got)
	}
	if got[2].Kind != model.InlineLink || got[2].URL != "https://y.io" || got[3].Value != "." {
		t.Fatalf("autolink: %+v", got)
	}
}

func TestMrkdwnMentions(t *testing.T) {
	o := opts(map[string]string{"U1": "aquila"})
	got := MrkdwnInline("<@U1> <@U2> <#C1|general> <!here> <!subteam^S1>", o)
	check := func(i int, kind model.MentionKind, label string, resolved bool) {
		t.Helper()
		in := got[i]
		if in.Kind != model.InlineMention || in.Mention != kind || in.Label != label || in.Resolved != resolved {
			t.Fatalf("got[%d] = %+v", i, in)
		}
	}
	check(0, model.MentionUser, "aquila", true)
	check(2, model.MentionUser, "U2", false)
	check(4, model.MentionChannel, "general", true)
	check(6, model.MentionHere, "here", true)
	check(8, model.MentionSubteam, "S1", false)
	var codes []model.WarningCode
	for _, w := range *o.Warnings {
		codes = append(codes, w.Code)
	}
	if !reflect.DeepEqual(codes, []model.WarningCode{model.WarnUnresolvedMention, model.WarnBroadcast, model.WarnUnresolvedMention}) {
		t.Fatalf("warnings: %+v", *o.Warnings)
	}
}

func TestMrkdwnEmojiAndDate(t *testing.T) {
	o := opts(nil)
	got := MrkdwnInline(":tada: :no_such_emoji_xyz: <!date^1392734382^{date_short} at {time}|Feb 18>", o)
	if got[0].Unicode != "🎉" || got[2].Unicode != "" || got[2].Name != "no_such_emoji_xyz" {
		t.Fatalf("emoji: %+v", got[:3])
	}
	if d := got[4]; d.Kind != model.InlineDate || d.Timestamp != 1392734382 || d.Format != "{date_short} at {time}" || d.Fallback != "Feb 18" {
		t.Fatalf("date: %+v", d)
	}
	if len(*o.Warnings) != 1 || (*o.Warnings)[0].Code != model.WarnUnknownEmoji {
		t.Fatalf("warnings: %+v", *o.Warnings)
	}
}

func TestMrkdwnBlocks(t *testing.T) {
	got := MrkdwnBlocks("hi\nthere\n\n> q1\n> q2\n```\ncode\n```\nend", opts(nil))
	kinds := []model.BlockKind{}
	for _, b := range got {
		kinds = append(kinds, b.Kind)
	}
	want := []model.BlockKind{model.BlockParagraph, model.BlockQuote, model.BlockCode, model.BlockParagraph}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("kinds %v", kinds)
	}
	if !reflect.DeepEqual(got[0].Children, []model.Inline{model.Text("hi"), {Kind: model.InlineBreak}, model.Text("there")}) {
		t.Fatalf("paragraph: %+v", got[0].Children)
	}
	if got[2].Value != "code" {
		t.Fatalf("code: %q", got[2].Value)
	}
}
