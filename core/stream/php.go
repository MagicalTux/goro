package stream

import (
	"net/url"
	"os"
)

var phpH = &phpHandler{
	stdin:  NewStream(os.Stdin),
	stdout: NewStream(os.Stdout),
	stderr: NewStream(os.Stderr),
}

type phpHandler struct {
	stdin, stdout, stderr *Stream
}

func PhpHandler() Handler {
	return phpH
}

func (h *phpHandler) Open(p *url.URL, mode ...string) (*Stream, error) {
	switch p.Path {
	case "stdin":
		return h.stdin, nil
	case "stdout":
		return h.stdout, nil
	case "stderr":
		return h.stderr, nil
	default:
		return nil, os.ErrNotExist
	}
}

func (h *phpHandler) Exists(p *url.URL) (bool, error) {
	switch p.Path {
	case "stdin", "stdout", "stderr":
		return true, nil
	case "memory", "temp":
		return true, nil
	}
	return false, nil
}

func (f *phpHandler) Stat(p *url.URL) (os.FileInfo, error) {
	return nil, ErrNotSupported
}

func (f *phpHandler) Lstat(p *url.URL) (os.FileInfo, error) {
	return nil, ErrNotSupported
}
