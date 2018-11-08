package main

import (
	"log"
	"net/http/cgi"

	"git.atonline.com/tristantech/gophp/core"
	_ "git.atonline.com/tristantech/gophp/ext/standard"
)

func main() {
	p := core.NewProcess()
	err := cgi.Serve(p.Handler("."))
	if err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}
