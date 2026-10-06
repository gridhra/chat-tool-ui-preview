// Package emoji は `:name:` の短縮名をUnicode絵文字に引く。描画はOSの絵文字フォントに任せ、画像は同梱しない（docs/DESIGN.md §2.2 R5）。
package emoji

import (
	"regexp"
	"strings"

	kemoji "github.com/kyokomi/emoji/v2"
)

var skinTone = regexp.MustCompile(`::skin-tone-\d$`)

// Lookup は短縮名（コロン無し）をUnicodeに引く。見つからなければ空文字。
func Lookup(name string) string {
	base := skinTone.ReplaceAllString(name, "")
	if u, ok := kemoji.CodeMap()[":"+base+":"]; ok {
		return strings.TrimSpace(u)
	}
	return ""
}
