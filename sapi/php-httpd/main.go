package main

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/MagicalTux/gophp/core"
	_ "github.com/MagicalTux/gophp/ext/ctype"
	_ "github.com/MagicalTux/gophp/ext/gmp"
	_ "github.com/MagicalTux/gophp/ext/hash"
	_ "github.com/MagicalTux/gophp/ext/pcre"
	_ "github.com/MagicalTux/gophp/ext/standard"
)

func main() {
	p := core.NewProcess("httpd")
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
