package phpctx

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/stream"
)

type OpenContext int

func (g *Global) getHandler(fn phpv.ZString) (stream.Handler, *url.URL, error) {
	u, err := url.Parse(string(fn))
	if err != nil {
		return nil, nil, err
	}

	s := u.Scheme
	if s == "" {
		s = "file"
	}

	h, ok := g.streamHandlers[s]
	if !ok {
		return nil, u, os.ErrInvalid
	}

	return h, u, nil
}

// Open opens a file using PHP stream wrappers and returns a handler to said
// file.
func (g *Global) Open(fn phpv.ZString, useIncludePath bool) (*stream.Stream, error) {
	h, u, err := g.getHandler(fn)
	if err != nil {
		return nil, err
	}

	f, err := h.Open(u)
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		return f, nil
	}

	// the docs didn't say if it should look in the
	// include first or last, assuming the latter
	if useIncludePath {
		for _, p := range g.includePath {
			var absPath string
			if filepath.IsAbs(p) {
				absPath = string(p)
			} else {
				absPath = filepath.Join(g.fileHandler.Root, p)
			}

			f, err = g.fileHandler.OpenFile(absPath)
			if err == nil {
				return f, nil
			}
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
	}

	return nil, os.ErrNotExist
}

func (g *Global) openForInclusion(ctx phpv.Context, fn phpv.ZString) (*stream.Stream, error) {
	// From the PHP docs:
	//   If the file isn't found in the include_path,
	//   include will finally check in the calling script's own directory
	//   and the current working directory before failing.
	// Note: this behaviour only applies to include_*, require_*.
	// Functions that has $use_include_path (such as fopen)
	// won't look in the script directory.

	h, u, err := g.getHandler(fn)
	if err != nil {
		return nil, err
	}
	localFile := u.Scheme == "file" || u.Scheme == ""
	if !localFile || filepath.IsAbs(u.Path) {
		return h.Open(u)
	}

	var f *stream.Stream
	for _, p := range g.includePath {
		var absPath string
		if filepath.IsAbs(p) {
			absPath = string(p)
		} else {
			absPath = filepath.Join(g.fileHandler.Root, p)
		}

		f, err = g.fileHandler.OpenFile(absPath)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	if f == nil {
		// file is not found in the include path,
		// look in script dir
		scriptDir := filepath.Dir(string(ctx.GetScriptFile()))
		path := phpv.ZString(filepath.Join(scriptDir, string(fn)))
		f, err = g.fileHandler.OpenFile(string(path))
		if err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	if f == nil {
		// file still not found,
		// look in current working directory
		path := phpv.ZString(filepath.Join(g.fileHandler.Cwd, string(fn)))
		f, err = g.fileHandler.OpenFile(string(path))
		if err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}

	if f == nil {
		return nil, os.ErrNotExist
	}

	return f, nil
}

func (g *Global) Exists(fn phpv.ZString) (bool, error) {
	h, u, err := g.getHandler(fn)
	if err != nil {
		return false, err
	}

	return h.Exists(u)
}

func (g *Global) Chdir(d phpv.ZString) error {
	// use file handler for chdir by default
	h, ok := g.streamHandlers["file"]
	if !ok {
		return os.ErrInvalid
	}

	chd, ok := h.(stream.Chdir)
	if !ok {
		return os.ErrInvalid
	}

	return chd.Chdir(string(d))
}

func (g *Global) Getwd() phpv.ZString {
	// use file handler for chdir by default
	h, ok := g.streamHandlers["file"]
	if !ok {
		return ""
	}

	chd, ok := h.(stream.Chdir)
	if !ok {
		return ""
	}

	return phpv.ZString(chd.Getwd())
}
