// Package web は cc-pages が同梱する静的ファイルとテンプレートを保持する。
//
// go:embed は親ディレクトリを参照できないため、ファイルはこのパッケージの中に置く。
package web

import (
	"embed"
	"io/fs"
)

//go:embed *.html static
var files embed.FS

// Templates は html/template の ParseFS に渡す読み出し口。
var Templates fs.FS = files

// Static は /_/ で配信する静的ファイル。
var Static = mustSub("static")

// mustSub は埋め込みのサブディレクトリを取り出す。
func mustSub(dir string) fs.FS {
	sub, err := fs.Sub(files, dir)
	if err != nil {
		panic(err) // 埋め込みはビルド時に確定するので、ここはビルド時のバグでしか起きない
	}
	return sub
}
