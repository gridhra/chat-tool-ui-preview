// chat-preview: チャット投稿の送信前プレビュー。
package main

import (
	"os"

	"github.com/gridhra/chat-tool-ui-preview/internal/cli"
)

func main() {
	os.Exit(cli.Default().Run(os.Args[1:]))
}
