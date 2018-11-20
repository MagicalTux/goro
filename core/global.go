package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/MagicalTux/gophp/core/stream"
)

type Global struct {
	context.Context

	p     *Process
	start time.Time
	root  *RootContext
	req   *http.Request

	globalFuncs   map[ZString]Callable
	globalClasses map[ZString]*ZClass // TODO replace *ZClass with a nice interface
	constant      map[ZString]*ZVal
	environ       []string
	fHandler      map[string]stream.Handler

	out io.Writer
}

func NewGlobal(ctx context.Context, p *Process) *Global {
	res := &Global{
		Context: ctx,
		p:       p,
		out:     os.Stdout,
	}
	res.init()
	return res
}

func NewGlobalReq(req *http.Request, p *Process) *Global {
	res := &Global{
		Context: req.Context(),
		req:     req,
		p:       p,
		out:     os.Stdout,
	}
	res.init()
	return res
}

func (g *Global) init() {
	g.start = time.Now()
	g.globalFuncs = make(map[ZString]Callable)
	g.globalClasses = make(map[ZString]*ZClass)
	g.constant = make(map[ZString]*ZVal)
	g.fHandler = make(map[string]stream.Handler)

	// prepare root context
	g.root = &RootContext{
		Context: g,
		g:       g,
		h:       NewHashTable(),
	}

	g.fHandler["file"], _ = stream.NewFileHandler("/")
	g.fHandler["php"] = stream.PhpHandler()

	// fill constants from process
	for k, v := range g.p.defaultConstants {
		g.constant[k] = v
	}

	// import global funcs & classes from ext
	for _, e := range globalExtMap {
		for k, v := range e.Functions {
			g.globalFuncs[ZString(k)] = v
		}
		for _, c := range e.Classes {
			g.globalClasses[c.Name.ToLower()] = c
		}
	}

	// initialize superglobals

	// TODO
}

func (g *Global) SetOutput(w io.Writer) {
	g.out = w
}

func (g *Global) Root() *RootContext {
	return g.root
}

func (g *Global) RunFile(fn string) error {
	_, err := g.root.Include(g.root, ZString(fn))
	return FilterError(err)
}

func (g *Global) Write(v []byte) (int, error) {
	return g.out.Write(v)
}

func (g *Global) GetGlobal() *Global {
	return g
}

func (g *Global) SetLocalConfig(name ZString, val *ZVal) error {
	// TODO
	return nil
}

func (g *Global) GetConfig(name ZString, def *ZVal) *ZVal {
	// TODO
	return def
}

// Open opens a file using PHP stream wrappers and returns a handler to said
// file.
func (g *Global) Open(u *url.URL) (*stream.Stream, error) {
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

func (g *Global) OffsetGet(ctx Context, name *ZVal) (*ZVal, error) {
	name, err := name.As(ctx, ZtString)
	if err != nil {
		return nil, err
	}

	switch name.AsString(ctx) {
	case "GLOBALS":
		// return GLOBALS as root hash table in a referenced array
		return (&ZVal{g.root}).Ref(), nil
	}

	// handle superglobals by using root context, avoid looping
	return g.root.h.GetString(name.AsString(ctx)), nil
}

func (g *Global) OffsetSet(ctx Context, name, v *ZVal) error {
	name, err := name.As(ctx, ZtString)
	if err != nil {
		return err
	}

	// handle superglobals by using root context, avoid looping
	return g.root.h.SetString(name.AsString(ctx), v)
}

func (g *Global) OffsetUnset(ctx Context, name *ZVal) error {
	name, err := name.As(ctx, ZtString)
	if err != nil {
		return err
	}

	// handle superglobals by using root context, avoid looping
	return g.root.h.UnsetString(name.AsString(ctx))
}

func (g *Global) Count(ctx Context) ZInt {
	return g.root.h.count
}

func (g *Global) NewIterator() ZIterator {
	return g.root.h.NewIterator()
}

func (g *Global) RegisterFunction(name ZString, f Callable) error {
	name = name.ToLower()
	if _, exists := g.globalFuncs[name]; exists {
		return errors.New("duplicate function name in declaration")
	}
	g.globalFuncs[name] = f
	return nil
}

func (g *Global) GetFunction(name ZString) (Callable, error) {
	if f, ok := g.globalFuncs[name.ToLower()]; ok {
		return f, nil
	}
	return nil, fmt.Errorf("Call to undefined function %s", name)
}

func (g *Global) GetConstant(name ZString) (*ZVal, error) {
	if v, ok := g.constant[name]; ok {
		return v, nil
	}
	return nil, nil
}

func (g *Global) GetClass(name ZString) (*ZClass, error) {
	if c, ok := g.globalClasses[name.ToLower()]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("Class '%s' not found", name)
}

func (g *Global) RegisterClass(name ZString, c *ZClass) error {
	name = name.ToLower()
	if _, ok := g.globalClasses[name]; ok {
		return fmt.Errorf("Cannot declare class %s, because the name is already in use", name)
	}
	g.globalClasses[name] = c
	return nil
}

func (g *Global) Getenv(key string) (string, bool) {
	// locate env
	env := g.environ
	if env == nil {
		env = g.p.environ
	}
	pfx := key + "="

	for _, s := range env {
		if strings.HasPrefix(s, pfx) {
			return s[len(pfx):], true
		}
	}
	return "", false
}

func (g *Global) Setenv(key, value string) error {
	if g.environ == nil {
		// if no environ for this global, copy from process
		g.environ = make([]string, len(g.p.environ))
		for k, v := range g.p.environ {
			g.environ[k] = v
		}
	}
	// lookup if it exists
	pfx := key + "="
	for i, s := range g.environ {
		if strings.HasPrefix(s, pfx) {
			// hit
			g.environ[i] = pfx + value
			return nil
		}
	}

	// no hit
	g.environ = append(g.environ, pfx+value)
	return nil
}

func (g *Global) Unsetenv(key string) error {
	if g.environ == nil {
		// if no environ for this global, copy from process
		g.environ = make([]string, len(g.p.environ))
		for k, v := range g.p.environ {
			g.environ[k] = v
		}
	}
	// lookup if it exists
	pfx := key + "="

	for i, s := range g.environ {
		if strings.HasPrefix(s, pfx) {
			g.environ = append(g.environ[:i], g.environ[i+1:]...)
			return nil
		}
	}
	return nil
}
