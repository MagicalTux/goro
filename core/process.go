package core

import (
	"net/http"
	"net/url"
	"os"

	"git.atonline.com/tristantech/gophp/core/stream"
)

type Process struct {
	fHandler         map[string]stream.Handler
	defaultConstants map[ZString]*ZVal
}

func NewProcess() *Process {
	res := &Process{
		fHandler:         make(map[string]stream.Handler),
		defaultConstants: make(map[ZString]*ZVal),
	}
	res.fHandler["file"], _ = stream.NewFileHandler("/")
	res.fHandler["php"] = stream.PhpHandler()
	res.populateConstants()
	return res
}

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

func (p *Process) Handler(docroot string) http.Handler {
	return nil // TODO
}

func (p *Process) populateConstants() {
	for _, e := range globalExtMap {
		for k, v := range e.Constants {
			p.defaultConstants[k] = v
		}
	}

}
