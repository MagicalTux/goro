package main

import (
	"log"
	"net"
	"net/http/fcgi"
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
	p := phpctx.NewProcess("fpm")
	p.CommandLine(os.Args)

	l, err := net.Listen("unix", "/tmp/php-fpm.sock")
	if err != nil {
		log.Fatalf("failed to listne: %s", err)
	}

	err = fcgi.Serve(l, p.Handler(".", ini.New()))
	if err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}
