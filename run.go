package main

import (
	"context"
	"log"
	"os"

	"github.com/KarpelesLab/goro/core/ini"
	"github.com/KarpelesLab/goro/core/phpctx"
	_ "github.com/KarpelesLab/goro/ext/bz2"
	_ "github.com/KarpelesLab/goro/ext/curl"
	_ "github.com/KarpelesLab/goro/ext/ctype"
	_ "github.com/KarpelesLab/goro/ext/date"
	_ "github.com/KarpelesLab/goro/ext/gmp"
	_ "github.com/KarpelesLab/goro/ext/hash"
	_ "github.com/KarpelesLab/goro/ext/json"
	_ "github.com/KarpelesLab/goro/ext/openssl"
	_ "github.com/KarpelesLab/goro/ext/mbstring"
	_ "github.com/KarpelesLab/goro/ext/mysqli"
	_ "github.com/KarpelesLab/goro/ext/sqlite3"
	_ "github.com/KarpelesLab/goro/ext/pcre"
	_ "github.com/KarpelesLab/goro/ext/reflection"
	_ "github.com/KarpelesLab/goro/ext/session"
	_ "github.com/KarpelesLab/goro/ext/sockets"
	_ "github.com/KarpelesLab/goro/ext/spl"
	_ "github.com/KarpelesLab/goro/ext/standard"
	_ "github.com/KarpelesLab/goro/ext/xml"
	_ "github.com/KarpelesLab/goro/ext/zlib"
)

func main() {
	// by default, run script test.php
	p := phpctx.NewProcess("cli")
	p.CommandLine(os.Args)
	ctx := phpctx.NewGlobal(context.Background(), p, ini.New())
	file := "test.php"
	if len(os.Args) > 1 {
		file = os.Args[1]
	}
	if err := ctx.RunFile(file); err != nil {
		log.Printf("failed to run test file: %s", err)
		os.Exit(1)
	}
}
