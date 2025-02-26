package main

import (
	"context"
	"fmt"
	"os"

	"github.com/MagicalTux/goro/core/ini"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phperr"
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

	err := p.CommandLine(os.Args)
	if err != nil {
		println("error:", err.Error())
		os.Exit(1)
	}

	options := p.Options

	cfg := ini.New()
	ctx := phpctx.NewGlobal(context.Background(), p, cfg)

	if options.RunCode != "" {
		fmt.Printf("options: %+v\n", options)
		_, err = ctx.DoString(ctx, phpv.ZString(options.RunCode))
		if err != nil {
			println("error:", err.Error())
			os.Exit(1)
		}
	}

	if p.ScriptFilename != "" {
		if err := ctx.RunFile(p.ScriptFilename); err != nil {
			displayErrors := ctx.GetConfig("display_errors", phpv.ZFalse.ZVal()).AsBool(ctx)
			if !displayErrors {
				if os.Getenv("DEBUG") == "" {
					os.Exit(1)
				}
				println("**NOTE: still showing errors even with display_errors=0 since DEBUG=1")
			}

			if ex, ok := err.(*phperr.PhpThrow); ok && bool(displayErrors) {
				ctx.Write([]byte("\nFatal error: "))
				ctx.Write([]byte(fmt.Sprintf(ex.ErrorTrace(ctx))))
				s := fmt.Sprintf("\n  thrown in %s on line %d", ex.Loc.Filename, ex.Loc.Line)
				ctx.Write([]byte(s))
			} else {
				if phpErr, ok := err.(*phpv.PhpError); ok {
					errorLevel := ctx.GetConfig("error_reporting", phpv.ZInt(0).ZVal()).AsInt(ctx)
					logError := int(errorLevel)&int(phpErr.Code) > 0
					if logError {
						if phpErr.Code == phpv.E_PARSE {
							ctx.Write([]byte(err.Error()))
						} else {
							ctx.Write([]byte("\nFatal error: "))
							ctx.Write([]byte(fmt.Sprintf("Uncaught Error: %s", err.Error())))
						}

					}
				} else {
					ctx.Write([]byte("\nFatal error: "))
					ctx.Write([]byte(fmt.Sprintf("Uncaught Error: %s", err.Error())))
				}

			}
			os.Exit(1)
		}
	}
}
