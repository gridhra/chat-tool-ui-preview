package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"

	"github.com/gridhra/chat-tool-ui-preview/internal/model"
	"github.com/gridhra/chat-tool-ui-preview/internal/parse"
	"github.com/gridhra/chat-tool-ui-preview/internal/render"
	"github.com/gridhra/chat-tool-ui-preview/internal/render/testdata"
)

// gallery は書式要素ごとの見本を1枚のHTMLで開く（Storybookの代替。docs/DESIGN.md §3.1）。
func (a *App) gallery(args []string) error {
	fs := flag.NewFlagSet("gallery", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	out := fs.String("out", "", "HTMLの出力先（省略時は一時ファイル）")
	noOpen := fs.Bool("no-open", false, "ブラウザを開かない")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var items []render.GalleryItem
	for _, name := range testdata.Names() {
		var in model.Input
		if err := json.Unmarshal(testdata.Read(name), &in); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		doc, err := parse.Build(in)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		body := render.Blocks(doc.Blocks)
		if len(doc.Warnings) > 0 {
			body += `<div class="warnings"><ul>`
			for _, w := range doc.Warnings {
				body += "<li><code>" + template.HTMLEscapeString(string(w.Code)) + "</code> " + template.HTMLEscapeString(w.Message) + "</li>"
			}
			body += `</ul></div>`
		}
		items = append(items, render.GalleryItem{Name: name, Body: template.HTML(body)})
	}
	page, err := render.GalleryPage(items)
	if err != nil {
		return err
	}
	path, err := a.write(*out, "chat-preview-gallery-*.html", page)
	if err != nil {
		return err
	}
	if !*noOpen {
		if err := a.Open(path); err != nil {
			fmt.Fprintln(a.Stderr, "ブラウザを開けなかった:", err)
		}
	}
	fmt.Fprintln(a.Stdout, path)
	return nil
}
