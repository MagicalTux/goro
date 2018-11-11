package main

import (
	"log"
	"net"
	"net/http"

	"git.atonline.com/tristantech/gophp/core"
	_ "git.atonline.com/tristantech/gophp/ext/standard"
)

func main() {
	p := core.NewProcess()

	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to listen: %s", err)
	}

	err = http.Serve(l, p.Handler("."))
	if err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}
