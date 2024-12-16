package main

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/MagicalTux/goro/core/ini"
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

	log.Printf("[php-httpd] Listening on %s", l.Addr())

	path := "."

	if len(os.Args) == 2 {
		path = os.Args[1]
	}

	err = http.Serve(l, p.Handler(path, ini.New()))
	if err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}
