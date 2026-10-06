package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/gridhra/chat-tool-ui-preview/internal/model"
	"github.com/gridhra/chat-tool-ui-preview/internal/parse"
	"github.com/gridhra/chat-tool-ui-preview/internal/render"
)

// Claude Codeアダプタ。PreToolUseフックのstdin JSONを核の入力に詰め替え、結果を hookSpecificOutput に変換する。
// v0: プレビューを開き、判断は permissionDecision "ask" で端末の許可ダイアログに委ねる。

// hookInput はClaude CodeがPreToolUseフックのstdinに渡すJSONのうち使う部分。
type hookInput struct {
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// slackSendInput は mcp__claude_ai_Slack__slack_send_message / slack_schedule_message / slack_send_message_draft の引数。
type slackSendInput struct {
	ChannelID      string `json:"channel_id"`
	Message        string `json:"message"`
	ThreadTS       string `json:"thread_ts"`
	ReplyBroadcast bool   `json:"reply_broadcast"`
}

type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// ClaudeSlackTools はこのアダプタが割り込むツール名（末尾一致）。
var ClaudeSlackTools = []string{"slack_send_message", "slack_schedule_message", "slack_send_message_draft"}

func matchesSlackTool(name string) bool {
	if !strings.HasPrefix(name, "mcp__") {
		return false
	}
	for _, t := range ClaudeSlackTools {
		if strings.HasSuffix(name, "__"+t) {
			return true
		}
	}
	return false
}

func (a *App) hookClaude() error {
	raw, err := io.ReadAll(a.Stdin)
	if err != nil {
		return err
	}
	var h hookInput
	if err := json.Unmarshal(raw, &h); err != nil {
		return fmt.Errorf("フックの入力JSONを読めない: %w", err)
	}
	if !matchesSlackTool(h.ToolName) {
		return nil // 対象外。何も出さず通常の許可の流れに任せる
	}
	var ti slackSendInput
	if err := json.Unmarshal(h.ToolInput, &ti); err != nil {
		return fmt.Errorf("tool_input を読めない: %w", err)
	}
	in := model.Input{
		Target: "slack",
		Format: model.FormatMarkdown,
		Text:   ti.Message,
		Source: model.SourceClaudeSlackMCP,
		Context: model.PostContext{
			ChannelID: ti.ChannelID, ThreadTS: ti.ThreadTS, ReplyBroadcast: ti.ReplyBroadcast, As: "user",
		},
	}
	doc, err := parse.Build(in)
	if err != nil {
		return err
	}
	page, err := render.Page(doc, render.Options{Raw: ti.Message})
	if err != nil {
		return err
	}
	path, err := a.write("", "chat-preview-*.html", page)
	if err != nil {
		return err
	}
	opened := a.Open(path) == nil

	var out hookOutput
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = "ask"
	out.HookSpecificOutput.PermissionDecisionReason = reason(h.ToolName, path, opened, doc.Warnings)
	return json.NewEncoder(a.Stdout).Encode(out)
}

func reason(tool, path string, opened bool, ws []model.Warning) string {
	var sb strings.Builder
	short := tool[strings.LastIndex(tool, "__")+2:]
	if opened {
		fmt.Fprintf(&sb, "[chat-preview] %s の送信前プレビューをブラウザで開いた（%s）。内容を見てから許可／拒否を選ぶ。", short, path)
	} else {
		fmt.Fprintf(&sb, "[chat-preview] %s の送信前プレビューを書き出した（%s）。ブラウザを自動で開けなかったので手で開く。", short, path)
	}
	if len(ws) == 0 {
		sb.WriteString(" 自動検査で引っかかった点はない。")
		return sb.String()
	}
	fmt.Fprintf(&sb, " 確認が必要な点 %d件:", len(ws))
	for _, w := range ws {
		fmt.Fprintf(&sb, "\n- [%s] %s", w.Code, w.Message)
	}
	return sb.String()
}
