package phpctx

import (
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/MagicalTux/goro/core/phpv"
)

type phpWebHandler struct {
	root string
	p    *Process
	cfg  phpv.IniConfig
}

func (p *phpWebHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	full := path.Join(p.root, path.Clean(req.URL.Path))
	if req.URL.Path[len(req.URL.Path)-1] == '/' {
		full += "/index.php"
	}

	// make a new global env
	g := NewGlobalReq(req, p.p, p.cfg)
	g.out = w

	// check if full exists
	fp, err := g.Open(phpv.ZString(full), "r", false)
	if err != nil {
		// likely not found. TODO check if dir. If dir, send redirect
		log.Printf("[php] Handling HTTP request for %s: Not Found", req.URL.Path)

		http.NotFound(w, req)
		return
	}
	defer fp.Close()

	// check if php
	if !strings.HasSuffix(full, ".php") {
		// normal file, just serve it
		log.Printf("[php] Handling HTTP request for %s: Static file", req.URL.Path)

		http.ServeContent(w, req, "", time.Time{}, fp)
		return
	}

	log.Printf("[php] Handling HTTP request for %s", req.URL.Path)

	// include file
	_, err = g.Include(g, phpv.ZString(full))
	g.Close()
	if err != nil {
		log.Printf("[php] Request failed: %s", err)
	}
}
