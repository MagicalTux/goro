package main

import (
	"log"
	"net"
	"net/http"
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
	_ "github.com/KarpelesLab/goro/ext/spl"
	_ "github.com/KarpelesLab/goro/ext/standard"
	_ "github.com/KarpelesLab/goro/ext/xml"
	_ "github.com/KarpelesLab/goro/ext/zlib"
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
