package core

import (
	"net/http"
	"net/url"
	"os"

	"github.com/MagicalTux/gophp/core/stream"
)

type Process struct {
	fHandler         map[string]stream.Handler
	defaultConstants map[ZString]*ZVal
	environ          []string
}

// NewProcess instanciates a new instance of Process, which represents a
// running PHP process.
func NewProcess() *Process {
	res := &Process{
		fHandler:         make(map[string]stream.Handler),
		defaultConstants: make(map[ZString]*ZVal),
		environ:          os.Environ(),
	}
	res.fHandler["file"], _ = stream.NewFileHandler("/")
	res.fHandler["php"] = stream.PhpHandler()
	res.populateConstants()
	return res
}

// Open opens a file using PHP stream wrappers and returns a handler to said
// file.
func (p *Process) Open(u *url.URL) (*stream.Stream, error) {
	s := u.Scheme
	if s == "" {
		s = "file"
	}

	h, ok := p.fHandler[s]
	if !ok {
		return nil, os.ErrInvalid
	}

	return h.Open(u)
}

// Hander returns a http.Handler object suitable for use with golang standard
// http servers and similar.
func (p *Process) Handler(docroot string) http.Handler {
	return &phpWebHandler{root: docroot, p: p}
}

func (p *Process) populateConstants() {
	for _, e := range globalExtMap {
		for k, v := range e.Constants {
			p.defaultConstants[k] = v
		}
	}

}

// SetConstant sets a global constant, typically used to set PHP_SAPI.
func (p *Process) SetConstant(name, value ZString) {
	p.defaultConstants[name] = value.ZVal()
}
