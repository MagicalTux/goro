package main

import (
	"context"
	"log"
	"os"

	"github.com/MagicalTux/goro/core/phpctx"
	_ "github.com/MagicalTux/goro/ext/bz2"
	_ "github.com/MagicalTux/goro/ext/ctype"
	_ "github.com/MagicalTux/goro/ext/date"
	_ "github.com/MagicalTux/goro/ext/gmp"
	_ "github.com/MagicalTux/goro/ext/hash"
	_ "github.com/MagicalTux/goro/ext/json"
	_ "github.com/MagicalTux/goro/ext/pcre"
	_ "github.com/MagicalTux/goro/ext/standard"
)

func main() {
	// by default, run script test.php
	p := phpctx.NewProcess("cli")
	p.CommandLine(os.Args)
	ctx := phpctx.NewGlobal(context.Background(), p)
	if err := ctx.RunFile("test.php"); err != nil {
		log.Printf("failed to run test file: %s", err)
		os.Exit(1)
	}
}
