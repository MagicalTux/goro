package main

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/MagicalTux/goro/core/phpctx"
	_ "github.com/MagicalTux/goro/ext/ctype"
	_ "github.com/MagicalTux/goro/ext/date"
	_ "github.com/MagicalTux/goro/ext/gmp"
	_ "github.com/MagicalTux/goro/ext/hash"
	_ "github.com/MagicalTux/goro/ext/json"
	_ "github.com/MagicalTux/goro/ext/pcre"
	_ "github.com/MagicalTux/goro/ext/standard"
)

func main() {
	p := phpctx.NewProcess("httpd")
	p.CommandLine(os.Args)

	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to listen: %s", err)
	}

	err = http.Serve(l, p.Handler("."))
	if err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}
