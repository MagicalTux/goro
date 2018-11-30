package phpctx

import (
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/MagicalTux/goro/core/phpv"
)

type phpWebHandler struct {
	root string
	p    *Process
}

func (p *phpWebHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	full := path.Join(p.root, path.Clean(req.URL.Path))
	if full[len(full)-1] == '/' {
		full += "index.php"
	}

	// make a new global env
	g := NewGlobalReq(req, p.p)
	g.out = w

	// check if full exists
	fp, err := g.Open(phpv.ZString(full), false)
	if err != nil {
		// likely not found. TODO check if dir. If dir, send redirect
		http.NotFound(w, req)
		return
	}
	defer fp.Close()

	// check if php
	if !strings.HasSuffix(full, ".php") {
		// normal file, just serve it
		http.ServeContent(w, req, "", time.Time{}, fp)
		return
	}

	// include file
	g.Include(g, phpv.ZString(full))
	g.Close()
}
