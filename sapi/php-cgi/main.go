package main

import (
	"log"
	"net/http/cgi"
	"os"

	"github.com/MagicalTux/goro/core/ini"
	"github.com/MagicalTux/goro/core/phpctx"
	_ "github.com/MagicalTux/goro/ext/bz2"
	_ "github.com/MagicalTux/goro/ext/curl"
	_ "github.com/MagicalTux/goro/ext/ctype"
	_ "github.com/MagicalTux/goro/ext/date"
	_ "github.com/MagicalTux/goro/ext/gd"
	_ "github.com/MagicalTux/goro/ext/gmp"
	_ "github.com/MagicalTux/goro/ext/hash"
	_ "github.com/MagicalTux/goro/ext/json"
	_ "github.com/MagicalTux/goro/ext/mysqli"
	_ "github.com/MagicalTux/goro/ext/sqlite3"
	_ "github.com/MagicalTux/goro/ext/openssl"
	_ "github.com/MagicalTux/goro/ext/pcre"
	_ "github.com/MagicalTux/goro/ext/reflection"
	_ "github.com/MagicalTux/goro/ext/session"
	_ "github.com/MagicalTux/goro/ext/sockets"
	_ "github.com/MagicalTux/goro/ext/standard"
	_ "github.com/MagicalTux/goro/ext/xml"
	_ "github.com/MagicalTux/goro/ext/zlib"
)

func main() {
	p := phpctx.NewProcess("cgi")
	p.CommandLine(os.Args)
	err := cgi.Serve(p.Handler(".", ini.New()))
	if err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}
