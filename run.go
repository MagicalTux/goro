package main

import (
	"context"
	"log"
	"os"

	"github.com/MagicalTux/gophp/core"
	_ "github.com/MagicalTux/gophp/ext/ctype"
	_ "github.com/MagicalTux/gophp/ext/pcre"
	_ "github.com/MagicalTux/gophp/ext/standard"
)

func main() {
	// by default, run script test.php
	p := core.NewProcess()
	p.SetConstant("PHP_SAPI", "cli")
	ctx := core.NewGlobal(context.Background(), p)
	if err := ctx.RunFile("test.php"); err != nil {
		log.Printf("failed to run test file: %s", err)
		os.Exit(1)
	}
}
