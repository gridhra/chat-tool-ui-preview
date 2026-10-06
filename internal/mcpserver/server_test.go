package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func session(t *testing.T) (*mcp.ClientSession, *[]string) {
	t.Helper()
	var opened []string
	d := Deps{Open: func(p string) error { opened = append(opened, p); return nil }, TempDir: t.TempDir(), Now: func() time.Time { return time.Now() }}
	srv := New(d, "test")
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs, &opened
}

func TestPreviewMessage(t *testing.T) {
	cs, opened := session(t)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "preview_message", Arguments: map[string]any{
		"format": "markdown", "text": "<!here> **done** <@U1>", "channel_id": "C1", "thread_ts": "1.2", "source": "claude-slack-mcp",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	b, _ := json.Marshal(res.StructuredContent)
	var out PreviewOut
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Opened || len(*opened) != 1 || (*opened)[0] != out.Path {
		t.Fatalf("opened %v, out %+v", *opened, out)
	}
	var codes []string
	for _, w := range out.Warnings {
		codes = append(codes, string(w.Code))
	}
	if strings.Join(codes, ",") != "broadcast,unresolved_mention,conversion_note" {
		t.Fatalf("warnings: %v", codes)
	}
	page, err := os.ReadFile(out.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "<strong>done</strong>") || !strings.Contains(string(page), "スレッドへの返信") {
		t.Fatal("page lacks body or thread flag")
	}
	if !strings.Contains(out.Next, "Do not send") {
		t.Fatalf("next: %s", out.Next)
	}
}

func TestPreviewMessageBlocks(t *testing.T) {
	cs, _ := session(t)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "preview_message", Arguments: map[string]any{
		"format": "blocks", "text": "alt", "channel_id": "C1",
		"blocks": []any{map[string]any{"type": "section", "text": map[string]any{"type": "mrkdwn", "text": "*hi*"}}},
	}})
	if err != nil || res.IsError {
		t.Fatalf("err %v res %+v", err, res)
	}
	b, _ := json.Marshal(res.StructuredContent)
	var out PreviewOut
	json.Unmarshal(b, &out)
	if len(out.Warnings) != 0 {
		t.Fatalf("warnings: %+v", out.Warnings)
	}
}

func TestPreviewMessageRejectsBadFormat(t *testing.T) {
	cs, opened := session(t)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "preview_message", Arguments: map[string]any{"format": "html", "text": "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || len(*opened) != 0 {
		t.Fatalf("want tool error without opening, got %+v opened %v", res, *opened)
	}
}
