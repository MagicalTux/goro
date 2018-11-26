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

	"github.com/MagicalTux/goro/core/stream"
)

type globalLazyOffset struct {
	r Runnables
	p int
}

type Global struct {
	context.Context

	p        *Process
	start    time.Time // time at which this request started
	req      *http.Request
	h        *ZHashTable
	l        *Loc
	mem      *MemMgr
	deadline time.Time

	// this is the actual environment (defined functions, classes, etc)
	globalFuncs   map[ZString]Callable
	globalClasses map[ZString]*ZClass // TODO replace *ZClass with a nice interface
	constant      map[ZString]*ZVal
	environ       *ZHashTable
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
	// initialize variables & memory for global context
	g.start = time.Now()
	g.h = NewHashTable()
	g.l = &Loc{Filename: "unknown", Line: 1}
	g.globalFuncs = make(map[ZString]Callable)
	g.globalClasses = make(map[ZString]*ZClass)
	g.constant = make(map[ZString]*ZVal)
	g.fHandler = make(map[string]stream.Handler)
	g.included = make(map[ZString]bool)
	g.globalLazyFunc = make(map[ZString]*globalLazyOffset)
	g.globalLazyClass = make(map[ZString]*globalLazyOffset)
	g.mem = NewMemMgr(32 * 1024 * 1024)        // limit in bytes TODO read memory_limit from process (.ini file)
	g.deadline = g.start.Add(30 * time.Second) // deadline

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

	// get env from process
	g.environ = g.p.environ.Dup()

	g.doGPC()
}

func (g *Global) doGPC() {
	// initialize superglobals
	get := NewZArray()
	p := NewZArray()
	c := NewZArray()
	r := NewZArray()
	s := NewZArray()
	e := NewZArray() // initialize empty
	f := NewZArray()

	order := g.GetConfig("variables_order", ZString("EGPCS").ZVal()).String()

	for _, l := range order {
		switch l {
		case 'e', 'E':
			e = &ZArray{h: g.environ}
			s.MergeArray(e)
		case 'p', 'P':
			if g.req != nil && g.req.Method == "POST" {
				err := g.parsePost(p, f)
				if err != nil {
					log.Printf("failed to parse POST data: %s", err)
				}
				r.MergeArray(p)
			}
		case 'g', 'G':
			if g.req != nil {
				err := ParseQueryToArray(g, g.req.URL.RawQuery, get)
				if err != nil {
					log.Printf("failed to parse GET data: %s", err)
				}
				r.MergeArray(get)
			}
		case 's', 'S':
			// SERVER
			s.OffsetSet(g, ZString("REQUEST_TIME").ZVal(), ZInt(g.start.Unix()).ZVal())
			s.OffsetSet(g, ZString("REQUEST_TIME_FLOAT").ZVal(), ZFloat(float64(g.start.UnixNano())/1e9).ZVal())
			// TODO...
		}
	}
	g.h.SetString("_GET", get.ZVal())
	g.h.SetString("_POST", p.ZVal())
	g.h.SetString("_COOKIE", c.ZVal())
	g.h.SetString("_REQUEST", r.ZVal())
	g.h.SetString("_SERVER", s.ZVal())
	g.h.SetString("_ENV", e.ZVal())
	g.h.SetString("_FILES", f.ZVal())
	// _SESSION will only be set if a session is initialized

	// parse post if any
	// TODO
}

func (g *Global) SetOutput(w io.Writer) {
	g.out = w
	g.buf = nil
}

func (g *Global) RunFile(fn string) error {
	_, err := g.Require(g, ZString(fn))
	err = FilterError(err)
	if err != nil {
		return err
	}
	return g.Close()
}

func (g *Global) Write(v []byte) (int, error) {
	return g.out.Write(v)
}

func (g *Global) SetLocalConfig(name ZString, val *ZVal) error {
	// TODO
	return nil
}

func (g *Global) GetConfig(name ZString, def *ZVal) *ZVal {
	// TODO
	return def
}

func (g *Global) Tick(ctx Context, l *Loc) error {
	// TODO check run deadline, context cancellation and memory limit
	if time.Until(g.deadline) <= 0 {
		return errors.New("Maximum execution time of TODO second exceeded") // TODO
	}
	g.l = l
	return nil
}

func (g *Global) Deadline() (deadline time.Time, ok bool) {
	return g.deadline, true
}

func (g *Global) SetDeadline(t time.Time) {
	g.deadline = t
}

func (g *Global) Loc() *Loc {
	return g.l
}

func (g *Global) Func() *FuncContext {
	return nil
}

func (g *Global) This() *ZObject {
	return nil
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
		f.r[f.p] = RunNull{} // remove function declaration from tree now that his as been run
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
	if r, ok := g.globalLazyClass[name.ToLower()]; ok {
		_, err := r.r[r.p].Run(ctx)
		if err != nil {
			return nil, err
		}
		r.r[r.p] = RunNull{} // remove function declaration from tree now that his as been run
		if c, ok := g.globalClasses[name.ToLower()]; ok {
			return c, nil
		}
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

func (g *Global) Global() *Global {
	return g
}

func (g *Global) MemAlloc(ctx Context, s uint64) error {
	return g.mem.Alloc(ctx, s)
}
