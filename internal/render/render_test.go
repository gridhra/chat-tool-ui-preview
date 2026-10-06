package render

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridhra/chat-tool-ui-preview/internal/model"
	"github.com/gridhra/chat-tool-ui-preview/internal/parse"
	"github.com/gridhra/chat-tool-ui-preview/internal/render/testdata"
)

var update = flag.Bool("update", false, "ゴールデンファイルを書き直す")

// 見本入力（testdata/*.json）ごとに本文HTMLをゴールデンと比べる。更新は go test ./internal/render -update。
func TestGolden(t *testing.T) {
	for _, name := range testdata.Names() {
		t.Run(name, func(t *testing.T) {
			var in model.Input
			if err := json.Unmarshal(testdata.Read(name), &in); err != nil {
				t.Fatal(err)
			}
			doc, err := parse.Build(in)
			if err != nil {
				t.Fatal(err)
			}
			got := Blocks(doc.Blocks)
			golden := filepath.Join("testdata", name+".golden.html")
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("ゴールデンが無い（-update で作る）: %v", err)
			}
			if string(want) != got {
				t.Errorf("出力がゴールデンと違う。-update で更新する前に差分を確認する\n--- want\n%s\n--- got\n%s", want, got)
			}
			page, _ := Page(doc, Options{})
			for _, bad := range []string{"Slack", "slack", "Block Kit", "mrkdwn"} {
				if strings.Contains(string(page), bad) {
					t.Errorf("画面に特定サービスの語 %q が出ている", bad)
				}
			}
		})
	}
}

func TestPageEscapesAndMarksBanner(t *testing.T) {
	doc, _ := parse.Build(model.Input{Format: model.FormatMrkdwn, Text: "<script>alert(1)</script> *x*", Context: model.PostContext{ChannelID: "C1", ChannelName: "gen", ThreadTS: "1.2", As: "bot"}})
	page, err := Page(doc, Options{Raw: "<raw>"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	for _, want := range []string{"送信先サービスの画面ではなく", "&lt;script&gt;", "<strong>x</strong>", "#gen", "スレッドへの返信", "ボットとして送信", "&lt;raw&gt;", "Content-Security-Policy"} {
		if !strings.Contains(s, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if strings.Contains(s, "<script>") {
		t.Error("script tag leaked unescaped")
	}
	// 画面内に特定サービスの名前や用語を出さない（docs/DESIGN.md §2.2）
	for _, bad := range []string{"Slack", "slack", "Block Kit", "mrkdwn"} {
		if strings.Contains(s, bad) {
			t.Errorf("page mentions %q", bad)
		}
	}
}
