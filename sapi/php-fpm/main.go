package main

import (
	"log"
	"net"
	"net/http/fcgi"
	"os"

	"github.com/MagicalTux/gophp/core"
	_ "github.com/MagicalTux/gophp/ext/ctype"
	_ "github.com/MagicalTux/gophp/ext/gmp"
	_ "github.com/MagicalTux/gophp/ext/pcre"
	_ "github.com/MagicalTux/gophp/ext/standard"
)

func main() {
	p := core.NewProcess("fpm")
	p.CommandLine(os.Args)

	l, err := net.Listen("unix", "/tmp/php-fpm.sock")
	if err != nil {
		log.Fatalf("failed to listne: %s", err)
	}

	err = fcgi.Serve(l, p.Handler("."))
	if err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}
