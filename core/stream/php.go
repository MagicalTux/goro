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

func (h *phpHandler) Open(p *url.URL) (*Stream, error) {
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
