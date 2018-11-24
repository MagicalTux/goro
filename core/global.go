package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/MagicalTux/gophp/core/stream"
)

type globalLazyOffset struct {
	r Runnables
	p int
}

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
	included      map[ZString]bool // included files (used for require_once, etc)

	globalLazyFunc  map[ZString]*globalLazyOffset
	globalLazyClass map[ZString]*globalLazyOffset

	out io.Writer
	buf *Buffer
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

func (g *Global) AppendBuffer() *Buffer {
	b := makeBuffer(g, g.out)
	g.out = b
	g.buf = b
	return b
}

func (g *Global) Buffer() *Buffer {
	return g.buf
}

func (g *Global) init() {
	g.start = time.Now()
	g.globalFuncs = make(map[ZString]Callable)
	g.globalClasses = make(map[ZString]*ZClass)
	g.constant = make(map[ZString]*ZVal)
	g.fHandler = make(map[string]stream.Handler)
	g.included = make(map[ZString]bool)
	g.globalLazyFunc = make(map[ZString]*globalLazyOffset)
	g.globalLazyClass = make(map[ZString]*globalLazyOffset)

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
	g.buf = nil
}

func (g *Global) Root() *RootContext {
	return g.root
}

func (g *Global) RunFile(fn string) error {
	_, err := g.Require(g.root, ZString(fn))
	err = FilterError(err)
	if err != nil {
		return err
	}
	return g.Close()
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
	delete(g.globalLazyFunc, name)
	return nil
}

func (g *Global) GetFunction(ctx Context, name ZString) (Callable, error) {
	if f, ok := g.globalFuncs[name.ToLower()]; ok {
		return f, nil
	}
	if f, ok := g.globalLazyFunc[name.ToLower()]; ok {
		_, err := f.r[f.p].Run(ctx)
		if err != nil {
			return nil, err
		}
		f.r[f.p] = f.r[f.p].Loc() // remove function declaration from tree now that his as been run
		if f, ok := g.globalFuncs[name.ToLower()]; ok {
			return f, nil
		}
	}
	return nil, fmt.Errorf("Call to undefined function %s", name)
}

func (g *Global) GetConstant(name ZString) (*ZVal, error) {
	if v, ok := g.constant[name]; ok {
		return v, nil
	}
	return nil, nil
}

func (g *Global) GetClass(ctx Context, name ZString) (*ZClass, error) {
	switch name {
	case "self":
		// check for func
		f := ctx.Func()
		if f == nil {
			return nil, errors.New("Cannot access self:: when no method scope is active")
		}
		cfunc, ok := f.c.(*ZClosure)
		if !ok || cfunc.class == nil {
			log.Printf("cfunc=%#v", f.c)
			return nil, errors.New("Cannot access self:: when no class scope is active")
		}
		return cfunc.class, nil
	case "parent":
		// check for func
		f := ctx.Func()
		if f == nil {
			return nil, errors.New("Cannot access parent:: when no method scope is active")
		}
		cfunc, ok := f.c.(*ZClosure)
		if !ok || cfunc.class == nil {
			return nil, errors.New("Cannot access parent:: when no class scope is active")
		}
		if cfunc.class.Extends == nil {
			return nil, errors.New("Cannot access parent:: when current class scope has no parent")
		}
		return cfunc.class.Extends, nil
	case "static":
		// check for func
		f := ctx.Func()
		if f == nil || f.this == nil {
			return nil, errors.New("Cannot access static:: when no class scope is active")
		}
		return f.this.Class, nil
	}
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
	delete(g.globalLazyClass, name)
	return nil
}

func (g *Global) Close() error {
	for {
		if g.buf == nil {
			return nil
		}
		err := g.buf.Close()
		if err != nil {
			return err
		}
	}
}

func (g *Global) Flush() {
	// flush io (not buffers)
	if f, ok := g.out.(http.Flusher); ok {
		f.Flush()
	}
}

func (g *Global) RegisterLazyFunc(name ZString, r Runnables, p int) {
	g.globalLazyFunc[name.ToLower()] = &globalLazyOffset{r, p}
}

func (g *Global) RegisterLazyClass(name ZString, r Runnables, p int) {
	g.globalLazyClass[name.ToLower()] = &globalLazyOffset{r, p}
}
