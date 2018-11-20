package core

import (
	"net/http"
	"os"
)

type Process struct {
	defaultConstants map[ZString]*ZVal
	environ          []string
}

// NewProcess instanciates a new instance of Process, which represents a
// running PHP process.
func NewProcess() *Process {
	res := &Process{
		defaultConstants: make(map[ZString]*ZVal),
		environ:          os.Environ(),
	}
	res.populateConstants()
	return res
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
