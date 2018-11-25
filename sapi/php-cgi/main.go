package main

import (
	"log"
	"net/http/cgi"
	"os"

	"github.com/MagicalTux/goro/core"
	_ "github.com/MagicalTux/goro/ext/ctype"
	_ "github.com/MagicalTux/goro/ext/gmp"
	_ "github.com/MagicalTux/goro/ext/hash"
	_ "github.com/MagicalTux/goro/ext/pcre"
	_ "github.com/MagicalTux/goro/ext/standard"
)

func main() {
	p := core.NewProcess("cgi")
	p.CommandLine(os.Args)
	err := cgi.Serve(p.Handler("."))
	if err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}
