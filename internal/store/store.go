// Package store はプレビューHTMLの一時ファイルを扱う。本人のみ読める権限で作り、古いものは掃除する（投稿内容を残さない）。
package store

import (
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Pattern は一時ファイル名の形。
const Pattern = "chat-preview-*.html"

// MaxAge はこれより古い一時ファイルを掃除する。
const MaxAge = time.Hour

// Write は dir に本人のみ読める一時ファイルを作って data を書き、パスを返す。ついでに古いファイルを消す。
func Write(dir, pattern string, data []byte, now time.Time) (string, error) {
	Sweep(dir, pattern, now)
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := f.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// Sweep は dir にある pattern の一時ファイルのうち MaxAge より古いものを消す。
func Sweep(dir, pattern string, now time.Time) {
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	for _, m := range matches {
		if st, err := os.Stat(m); err == nil && now.Sub(st.ModTime()) > MaxAge {
			_ = os.Remove(m)
		}
	}
}
