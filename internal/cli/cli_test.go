package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testApp(t *testing.T, stdin string) (*App, *bytes.Buffer, *[]string) {
	t.Helper()
	var out bytes.Buffer
	var opened []string
	app := &App{
		Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: os.Stderr,
		Open:    func(p string) error { opened = append(opened, p); return nil },
		TempDir: t.TempDir(),
		Now:     func() time.Time { return time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC) },
	}
	return app, &out, &opened
}

func TestPreviewWritesTempFileAndOpens(t *testing.T) {
	app, out, opened := testApp(t, `{"format":"mrkdwn","text":"*hi* <@U1>","context":{"channel_id":"C1"}}`)
	if code := app.Run([]string{"preview"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	var r Result
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if len(*opened) != 1 || (*opened)[0] != r.Path {
		t.Fatalf("opened %v, path %s", *opened, r.Path)
	}
	st, err := os.Stat(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("perm %o", st.Mode().Perm())
	}
	if len(r.Warnings) != 1 || r.Warnings[0].Code != "unresolved_mention" {
		t.Fatalf("warnings %+v", r.Warnings)
	}
}

func TestPreviewRejectsBadFormat(t *testing.T) {
	app, _, _ := testApp(t, `{"format":"html","text":"x"}`)
	if code := app.Run([]string{"preview", "--no-open"}); code != 1 {
		t.Fatalf("exit %d", code)
	}
}

func TestHookClaudeAsksWithPreview(t *testing.T) {
	stdin := `{"tool_name":"mcp__claude_ai_Slack__slack_send_message","tool_input":{"channel_id":"C1","message":"**done** <@U1>","thread_ts":"1.2"}}`
	app, out, opened := testApp(t, stdin)
	if code := app.Run([]string{"hook", "claude"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	var h hookOutput
	if err := json.Unmarshal(out.Bytes(), &h); err != nil {
		t.Fatal(err)
	}
	if h.HookSpecificOutput.PermissionDecision != "ask" || h.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Fatalf("output %+v", h)
	}
	reason := h.HookSpecificOutput.PermissionDecisionReason
	if !strings.Contains(reason, "unresolved_mention") || !strings.Contains(reason, "conversion_note") || !strings.Contains(reason, (*opened)[0]) {
		t.Fatalf("reason %q", reason)
	}
	page, _ := os.ReadFile((*opened)[0])
	if !strings.Contains(string(page), "スレッドへの返信") || !strings.Contains(string(page), "<strong>done</strong>") {
		t.Fatalf("page lacks thread flag or bold")
	}
}

func TestHookClaudeIgnoresOtherTools(t *testing.T) {
	app, out, opened := testApp(t, `{"tool_name":"Bash","tool_input":{"command":"ls"}}`)
	if code := app.Run([]string{"hook", "claude"}); code != 0 || out.Len() != 0 || len(*opened) != 0 {
		t.Fatalf("code %d out %q opened %v", code, out.String(), *opened)
	}
}

func TestSweepRemovesOldPreviews(t *testing.T) {
	app, _, _ := testApp(t, `{"format":"mrkdwn","text":"x","context":{"channel_id":"C1"}}`)
	old := filepath.Join(app.TempDir, "chat-preview-old.html")
	os.WriteFile(old, []byte("x"), 0o600)
	os.Chtimes(old, time.Time{}, app.Now().Add(-2*time.Hour))
	recent := filepath.Join(app.TempDir, "chat-preview-new.html")
	os.WriteFile(recent, []byte("x"), 0o600)
	os.Chtimes(recent, time.Time{}, app.Now().Add(-10*time.Minute))
	app.Run([]string{"preview", "--no-open"})
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("old file should be removed")
	}
	if _, err := os.Stat(recent); err != nil {
		t.Error("recent file should be kept")
	}
}
