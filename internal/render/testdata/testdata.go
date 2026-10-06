// Package testdata は見本兼ゴールデンテストの入力（*.json）を同梱する。
package testdata

import (
	"embed"
	"sort"
	"strings"
)

//go:embed *.json
var fs embed.FS

// Names は入力ファイル名（拡張子なし）を名前順で返す。
func Names() []string {
	entries, _ := fs.ReadDir(".")
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	sort.Strings(names)
	return names
}

// Read は入力ファイルの中身を返す。無ければnil。
func Read(name string) []byte {
	b, _ := fs.ReadFile(name + ".json")
	return b
}
