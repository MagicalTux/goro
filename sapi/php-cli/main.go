package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
	_ "github.com/MagicalTux/goro/ext/ctype"
	_ "github.com/MagicalTux/goro/ext/date"
	_ "github.com/MagicalTux/goro/ext/gmp"
	_ "github.com/MagicalTux/goro/ext/hash"
	_ "github.com/MagicalTux/goro/ext/json"
	_ "github.com/MagicalTux/goro/ext/pcre"
	_ "github.com/MagicalTux/goro/ext/standard"
)

func main() {
	p := phpctx.NewProcess("cli")

	args, options, err := p.CommandLine(os.Args)
	if err != nil {
		println(err.Error())
	}
	ctx := phpctx.NewGlobal(context.Background(), p, options)

	if options.RunCode != ""{
		fmt.Printf("options: %+v\n", options)
		_, err = ctx.DoString(ctx, phpv.ZString(options.RunCode))
		if err != nil {
			println("error:", err.Error())
			os.Exit(-1)
		}
	}


	if len(args) >= 2 {
		if err := ctx.RunFile(args[1]); err != nil {
			log.Printf("failed to run file: %s", err)
			os.Exit(1)
		}
	}
}
