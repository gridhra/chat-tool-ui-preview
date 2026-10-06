// Package mcpserver はプレビューをエージェントにMCP（stdio）で提供する。ツールは preview_message の1本。
// v0: プレビューを開いて警告を返すまで。承認の往復（ページ上で承認・修正・却下）はローカルサーバー版と一緒に後で足す。
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gridhra/chat-tool-ui-preview/internal/model"
	"github.com/gridhra/chat-tool-ui-preview/internal/parse"
	"github.com/gridhra/chat-tool-ui-preview/internal/render"
	"github.com/gridhra/chat-tool-ui-preview/internal/store"
)

// Deps は差し替え可能な外部作用（テストではブラウザを開かない）。
type Deps struct {
	Open    func(path string) error
	TempDir string
	Now     func() time.Time
}

const instructions = "chat-tool-ui-preview shows a chat post to the user in their browser before it is sent, " +
	"so the user can check wording, mentions, destination and thread placement. " +
	"Call preview_message BEFORE sending any message to Slack on the user's behalf (any tool that posts, replies, schedules or drafts), " +
	"then ask the user in chat whether to send, and do not send until they say yes. " +
	"Pass the exact text and destination you are about to send; pass format=markdown for Claude's Slack tools, mrkdwn or blocks for the Slack Web API. " +
	"Report every warning the tool returns to the user. The tool never sends anything and never contacts Slack."

func boolp(b bool) *bool { return &b }

// New はMCPサーバーを組み立てる。
func New(d Deps, version string) *mcp.Server {
	s := &server{d: d}
	srv := mcp.NewServer(&mcp.Implementation{Name: "chat-tool-ui-preview", Version: version}, &mcp.ServerOptions{Instructions: instructions})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "preview_message",
		Description: "Render a chat post as a preview page, open it in the user's browser, and return the file path and the warnings found (unresolved mentions, @here/@channel broadcasts, unknown emoji, length limits, invalid blocks). Does not send anything. Call it before sending a message on the user's behalf, then ask the user to confirm in chat.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: boolp(false), IdempotentHint: true, OpenWorldHint: boolp(false)},
	}, s.previewMessage)
	return srv
}

// RunStdio は標準入出力でサーバーを動かす。
func RunStdio(ctx context.Context, d Deps, version string) error {
	return New(d, version).Run(ctx, &mcp.StdioTransport{})
}

type server struct{ d Deps }

// PreviewIn は preview_message の入力。CLIの入力JSONと同じ形。
type PreviewIn struct {
	Target         string            `json:"target,omitempty" jsonschema:"destination service; only slack for now (default)"`
	Format         string            `json:"format" jsonschema:"markdown (Claude's Slack tools), mrkdwn (Slack Web API text) or blocks (Block Kit JSON). Required; never guessed"`
	Text           string            `json:"text,omitempty" jsonschema:"the message body for markdown/mrkdwn; for blocks, the fallback text used in notifications"`
	Blocks         []map[string]any  `json:"blocks,omitempty" jsonschema:"Block Kit blocks array when format=blocks"`
	ChannelID      string            `json:"channel_id,omitempty" jsonschema:"destination channel or user ID exactly as it will be sent"`
	ChannelName    string            `json:"channel_name,omitempty" jsonschema:"human-readable channel name if known"`
	ThreadTS       string            `json:"thread_ts,omitempty" jsonschema:"parent message timestamp when replying in a thread"`
	ReplyBroadcast bool              `json:"reply_broadcast,omitempty" jsonschema:"true when the thread reply will also be shown in the channel"`
	As             string            `json:"as,omitempty" jsonschema:"user (default) when sent as the user, bot when sent as an app"`
	SenderName     string            `json:"sender_name,omitempty" jsonschema:"display name of the sender when known"`
	Names          map[string]string `json:"names,omitempty" jsonschema:"ID to display name map for users, channels and user groups mentioned, when known"`
	Source         string            `json:"source,omitempty" jsonschema:"claude-slack-mcp, slack-web-api or manual; controls which notes are shown"`
}

// PreviewOut は preview_message の出力。
type PreviewOut struct {
	Path     string          `json:"path" jsonschema:"path of the preview HTML file"`
	Opened   bool            `json:"opened" jsonschema:"true when the browser was opened; otherwise ask the user to open path by hand"`
	Warnings []model.Warning `json:"warnings" jsonschema:"points the user must check; empty when none"`
	Next     string          `json:"next" jsonschema:"what to do next"`
}

func (s *server) previewMessage(ctx context.Context, _ *mcp.CallToolRequest, in PreviewIn) (*mcp.CallToolResult, PreviewOut, error) {
	out := PreviewOut{Warnings: []model.Warning{}}
	blocks := make([]json.RawMessage, 0, len(in.Blocks))
	for _, b := range in.Blocks {
		raw, err := json.Marshal(b)
		if err != nil {
			return nil, out, err
		}
		blocks = append(blocks, raw)
	}
	input := model.Input{
		Target: in.Target, Format: model.Format(in.Format), Text: in.Text, Blocks: blocks, Source: model.Source(in.Source),
		Context: model.PostContext{
			ChannelID: in.ChannelID, ChannelName: in.ChannelName, ThreadTS: in.ThreadTS, ReplyBroadcast: in.ReplyBroadcast,
			As: in.As, SenderName: in.SenderName, Names: in.Names,
		},
	}
	doc, err := parse.Build(input)
	if err != nil {
		return nil, out, err
	}
	rawIn, _ := json.MarshalIndent(input, "", "  ")
	page, err := render.Page(doc, render.Options{Raw: string(rawIn)})
	if err != nil {
		return nil, out, err
	}
	path, err := store.Write(s.d.TempDir, store.Pattern, page, s.d.Now())
	if err != nil {
		return nil, out, err
	}
	out.Path = path
	out.Opened = s.d.Open(path) == nil
	if doc.Warnings != nil {
		out.Warnings = doc.Warnings
	}
	if out.Opened {
		out.Next = "The preview is open in the user's browser. Tell the user what it shows and list the warnings, then ask whether to send. Do not send until the user says yes."
	} else {
		out.Next = "The browser could not be opened. Ask the user to open the path, list the warnings, then ask whether to send. Do not send until the user says yes."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "preview written to %s (opened=%t), %d warning(s)", path, out.Opened, len(out.Warnings))
	for _, w := range out.Warnings {
		fmt.Fprintf(&sb, "\n- [%s] %s", w.Code, w.Message)
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}}}, out, nil
}
