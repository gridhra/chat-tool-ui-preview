package parse

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridhra/chat-tool-ui-preview/internal/model"
)

func kinds(bs []model.Block) string {
	var ks []string
	for _, b := range bs {
		ks = append(ks, string(b.Kind))
	}
	return strings.Join(ks, ",")
}

func codes(ws []model.Warning) string {
	var cs []string
	for _, w := range ws {
		cs = append(cs, string(w.Code))
	}
	return strings.Join(cs, ",")
}

func TestBuildMarkdown(t *testing.T) {
	doc, err := Build(model.Input{
		Format: model.FormatMarkdown, Source: model.SourceClaudeSlackMCP,
		Text:    "# Title\n\n**bold** and <@U1> :tada:\n\n- a\n- b\n\n| h | i |\n|---|---|\n| 1 | 2 |",
		Context: model.PostContext{ChannelID: "C1", Names: map[string]string{"U1": "aquila"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := kinds(doc.Blocks); got != "heading,paragraph,list,table" {
		t.Fatalf("kinds: %s", got)
	}
	p := doc.Blocks[1].Children
	if p[0].Kind != model.InlineBold || p[2].Kind != model.InlineMention || p[2].Label != "aquila" || p[4].Unicode != "🎉" {
		t.Fatalf("paragraph: %+v", p)
	}
	if got := codes(doc.Warnings); got != "conversion_note" {
		t.Fatalf("warnings: %s", got)
	}
	if tb := doc.Blocks[3]; len(tb.Header) != 2 || len(tb.Rows) != 1 {
		t.Fatalf("table: %+v", tb)
	}
}

func TestBuildBlocks(t *testing.T) {
	raw := `[
	 {"type":"header","text":{"type":"plain_text","text":"Deploy :rocket:"}},
	 {"type":"section","text":{"type":"mrkdwn","text":"*v1.2* by <@U9>"},"fields":[{"type":"mrkdwn","text":"*env*\nprod"}]},
	 {"type":"divider"},
	 {"type":"context","elements":[{"type":"mrkdwn","text":"_note_"}]},
	 {"type":"actions","elements":[{"type":"button","text":{"type":"plain_text","text":"Approve"},"style":"primary"},{"type":"static_select","placeholder":{"type":"plain_text","text":"pick"}}]},
	 {"type":"video"}]`
	var blocks []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &blocks); err != nil {
		t.Fatal(err)
	}
	doc, err := Build(model.Input{Format: model.FormatBlocks, Text: "fallback", Blocks: blocks, Context: model.PostContext{ChannelID: "C1"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := kinds(doc.Blocks); got != "heading,section,divider,context,actions,unsupported" {
		t.Fatalf("kinds: %s", got)
	}
	if got := codes(doc.Warnings); got != "unresolved_mention,unsupported" {
		t.Fatalf("warnings: %s", got)
	}
	if h := doc.Blocks[0].Children; len(h) != 2 || h[1].Unicode != "🚀" {
		t.Fatalf("header: %+v", h)
	}
	if a := doc.Blocks[4].Actions; a[0].Style != "primary" || a[1].Kind != model.ElementPlaceholder || a[1].Label != "pick" {
		t.Fatalf("actions: %+v", a)
	}
}

func TestBuildRejectsUnknownFormat(t *testing.T) {
	if _, err := Build(model.Input{Format: "html"}); err == nil || !strings.Contains(err.Error(), "format") {
		t.Fatalf("want format error, got %v", err)
	}
}

func TestBuildWarnsMissingChannel(t *testing.T) {
	doc, _ := Build(model.Input{Format: model.FormatMrkdwn, Text: "x"})
	if codes(doc.Warnings) != "missing_context" {
		t.Fatalf("warnings: %+v", doc.Warnings)
	}
}
