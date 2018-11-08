package core

import "net/http"

type Process struct{}

func NewProcess() *Process {
	return &Process{}
}

func (p *Process) Handler(path string) http.Handler {
	return nil // TODO
}
