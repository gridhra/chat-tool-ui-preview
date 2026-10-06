// Package cli はコマンドの入口。核（parse / render）に依存し、クライアント固有の作法はアダプタ（hook_*.go）に閉じ込める。
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/gridhra/chat-tool-ui-preview/internal/mcpserver"
	"github.com/gridhra/chat-tool-ui-preview/internal/model"
	"github.com/gridhra/chat-tool-ui-preview/internal/parse"
	"github.com/gridhra/chat-tool-ui-preview/internal/render"
	"github.com/gridhra/chat-tool-ui-preview/internal/store"
)

// App は差し替え可能な外部作用を束ねる（テストではブラウザを開かず、一時ディレクトリを変える）。
type App struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Open    func(path string) error // 既定ブラウザで開く
	TempDir string
	Now     func() time.Time
}

// Default は実環境のApp。
func Default() *App {
	return &App{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Open: openBrowser, TempDir: os.TempDir(), Now: time.Now}
}

const usage = `chat-preview — チャット投稿の送信前プレビュー（Slackの画面ではなく、書式と宛先を確認する中立な表示）

使い方:
  chat-preview preview [file.json] [--out path] [--no-open]   入力JSONをプレビューHTMLにしてブラウザで開く（省略時はstdin）
  chat-preview gallery [--out path] [--no-open]               書式要素の見本を1枚のHTMLで開く
  chat-preview hook claude                                    Claude CodeのPreToolUseフックとして動く（stdinにフックJSON）
  chat-preview mcp                                            MCPサーバー（stdio）。ツールは preview_message
  chat-preview --version

入力JSON: {"target":"slack","format":"markdown|mrkdwn|blocks","text":"...","blocks":[...],
           "context":{"channel_id":"C..","channel_name":"..","thread_ts":"..","reply_broadcast":false,"as":"user|bot","sender_name":"..","names":{"U..":"name"}},
           "source":"claude-slack-mcp|slack-web-api|manual"}
`

// Version はビルド時に差し替える。
var Version = "dev"

// Run はコマンドを実行し、終了コードを返す。
func (a *App) Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.Stderr, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "preview":
		err = a.preview(args[1:])
	case "gallery":
		err = a.gallery(args[1:])
	case "hook":
		err = a.hook(args[1:])
	case "mcp":
		err = mcpserver.RunStdio(context.Background(), mcpserver.Deps{Open: a.Open, TempDir: a.TempDir, Now: a.Now}, Version)
	case "version", "--version", "-V":
		fmt.Fprintf(a.Stdout, "chat-preview %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
	case "-h", "--help", "help":
		fmt.Fprint(a.Stdout, usage)
	default:
		fmt.Fprintf(a.Stderr, "不明なコマンド: %s\n\n%s", args[0], usage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(a.Stderr, "error:", err)
		return 1
	}
	return 0
}

// Result は preview の結果（標準出力にJSONで出す）。
type Result struct {
	Path     string          `json:"path"`
	Warnings []model.Warning `json:"warnings"`
}

func (a *App) preview(args []string) error {
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	out := fs.String("out", "", "HTMLの出力先（省略時は一時ファイル）")
	noOpen := fs.Bool("no-open", false, "ブラウザを開かない")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var raw []byte
	var err error
	if fs.NArg() > 0 {
		raw, err = os.ReadFile(fs.Arg(0))
	} else {
		raw, err = io.ReadAll(a.Stdin)
	}
	if err != nil {
		return err
	}
	var in model.Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return fmt.Errorf("入力JSONを読めない: %w", err)
	}
	doc, err := parse.Build(in)
	if err != nil {
		return err
	}
	page, err := render.Page(doc, render.Options{Raw: string(raw)})
	if err != nil {
		return err
	}
	path, err := a.write(*out, "chat-preview-*.html", page)
	if err != nil {
		return err
	}
	if !*noOpen {
		if err := a.Open(path); err != nil {
			fmt.Fprintln(a.Stderr, "ブラウザを開けなかった:", err)
		}
	}
	return json.NewEncoder(a.Stdout).Encode(Result{Path: path, Warnings: doc.Warnings})
}

// write は出力先が空なら一時ファイル（本人のみ読める権限）に書く。古い一時ファイルはついでに消す。
func (a *App) write(out, pattern string, data []byte) (string, error) {
	if out != "" {
		return out, os.WriteFile(out, data, 0o600)
	}
	return store.Write(a.TempDir, pattern, data, a.Now())
}

func openBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

var errNoHookClient = errors.New("hook の後にクライアント名が要る（対応: claude）")

func (a *App) hook(args []string) error {
	if len(args) == 0 {
		return errNoHookClient
	}
	switch args[0] {
	case "claude":
		return a.hookClaude()
	default:
		return fmt.Errorf("未対応のクライアント: %s（対応: claude）", args[0])
	}
}
