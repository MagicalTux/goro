package main

import (
	"log"
	"net/http/cgi"
	"os"

	"github.com/KarpelesLab/goro/core/ini"
	"github.com/KarpelesLab/goro/core/phpctx"
	_ "github.com/KarpelesLab/goro/ext/bz2"
	_ "github.com/KarpelesLab/goro/ext/curl"
	_ "github.com/KarpelesLab/goro/ext/ctype"
	_ "github.com/KarpelesLab/goro/ext/date"
	_ "github.com/KarpelesLab/goro/ext/gd"
	_ "github.com/KarpelesLab/goro/ext/gmp"
	_ "github.com/KarpelesLab/goro/ext/hash"
	_ "github.com/KarpelesLab/goro/ext/json"
	_ "github.com/KarpelesLab/goro/ext/mysqli"
	_ "github.com/KarpelesLab/goro/ext/sqlite3"
	_ "github.com/KarpelesLab/goro/ext/openssl"
	_ "github.com/KarpelesLab/goro/ext/pcre"
	_ "github.com/KarpelesLab/goro/ext/reflection"
	_ "github.com/KarpelesLab/goro/ext/session"
	_ "github.com/KarpelesLab/goro/ext/sockets"
	_ "github.com/KarpelesLab/goro/ext/standard"
	_ "github.com/KarpelesLab/goro/ext/xml"
	_ "github.com/KarpelesLab/goro/ext/zlib"
)

func main() {
	p := phpctx.NewProcess("cgi")
	p.CommandLine(os.Args)
	err := cgi.Serve(p.Handler(".", ini.New()))
	if err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}
