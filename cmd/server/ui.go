//go:build !dev

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:web/build
var embeddedUI embed.FS

func init() {
	var err error
	uiFS, err = fs.Sub(embeddedUI, "web/build")
	if err != nil {
		panic(err)
	}
}
