package main

import (
	"log"
	"net/http/cgi"
	"os"

	"github.com/MagicalTux/gophp/core"
	_ "github.com/MagicalTux/gophp/ext/ctype"
	_ "github.com/MagicalTux/gophp/ext/gmp"
	_ "github.com/MagicalTux/gophp/ext/hash"
	_ "github.com/MagicalTux/gophp/ext/pcre"
	_ "github.com/MagicalTux/gophp/ext/standard"
)

func main() {
	p := core.NewProcess("cgi")
	p.CommandLine(os.Args)
	err := cgi.Serve(p.Handler("."))
	if err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}
