package core

import (
	"net/url"
	"os"

	"github.com/MagicalTux/gophp/core/stream"
)

// Open opens a file using PHP stream wrappers and returns a handler to said
// file.
func (g *Global) Open(fn ZString) (*stream.Stream, error) {
	u, err := url.Parse(string(fn))
	if err != nil {
		return nil, err
	}

	s := u.Scheme
	if s == "" {
		s = "file"
	}

	h, ok := g.fHandler[s]
	if !ok {
		return nil, os.ErrInvalid
	}

	return h.Open(u)
}

func (g *Global) Chdir(d ZString) error {
	// use file handler for chdir by default
	h, ok := g.fHandler["file"]
	if !ok {
		return os.ErrInvalid
	}

	chd, ok := h.(stream.Chdir)
	if !ok {
		return os.ErrInvalid
	}

	return chd.Chdir(string(d))
}
