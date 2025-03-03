package stream

import (
	"net/url"
	"os"

	"github.com/MagicalTux/goro/core/phpv"
)

var phpH *phpHandler

var Stdin *Stream
var Stdout *Stream
var Stderr *Stream

func init() {
	Stdin = NewStream(os.Stdin)
	Stdout = NewStream(os.Stdout)
	Stderr = NewStream(os.Stderr)

	Stdin.SetAttr("stream_type", "Go")
	Stdin.SetAttr("mode", "r")
	Stdin.ResourceType = phpv.ResourceStream
	Stdin.ResourceID = 1

	Stdout.SetAttr("stream_type", "Go")
	Stdout.SetAttr("mode", "w")
	Stdout.ResourceType = phpv.ResourceStream
	Stdout.ResourceID = 2

	Stderr.SetAttr("stream_type", "Go")
	Stderr.SetAttr("mode", "w")
	Stderr.ResourceType = phpv.ResourceStream
	Stderr.ResourceID = 3

	phpH = &phpHandler{Stdin, Stdout, Stderr}
}

type phpHandler struct {
	stdin, stdout, stderr *Stream
}

func PhpHandler() Handler {
	return phpH
}

func (h *phpHandler) getPath(p *url.URL) string {
	if p.Path != "" {
		return p.Path
	}
	// some urls such as php://stdin has an empty path
	// so return the host instead, which would return
	// the expected "stdin" part
	return p.Host
}

func (h *phpHandler) Open(ctx phpv.Context, p *url.URL, mode ...string) (*Stream, error) {
	switch h.getPath(p) {
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
	switch h.getPath(p) {
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
