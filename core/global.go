package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
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
	start    time.Time
	req      *http.Request
	h        *ZHashTable
	l        *Loc
	mem      *MemMgr
	deadline time.Time

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
				err := parseQueryToArray(g, g.req.URL.RawQuery, get)
				if err != nil {
					log.Printf("failed to parse GET data: %s", err)
				}
				r.MergeArray(get)
			}
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

func (g *Global) parsePost(p, f *ZArray) error {
	if g.req.Body == nil {
		return errors.New("missing form body")
	}
	ct := g.req.Header.Get("Content-Type")
	// RFC 7231, section 3.1.1.5 - empty type MAY be treated as application/octet-stream
	if ct == "" {
		ct = "application/octet-stream"
	}
	ct, params, _ := mime.ParseMediaType(ct)

	switch {
	case ct == "application/x-www-form-urlencoded":
		var reader io.Reader = g.req.Body
		maxFormSize := int64(10 << 20) // 10 MB is a lot of text.
		reader = io.LimitReader(g.req.Body, maxFormSize+1)
		b, err := ioutil.ReadAll(reader)
		if err != nil {
			return err
		}
		if int64(len(b)) > maxFormSize {
			return errors.New("http: POST too large")
		}
		err = g.MemAlloc(g, uint64(len(b)))
		if err != nil {
			return err
		}
		vs, err := url.ParseQuery(string(b))
		if err != nil {
			return err
		}
		return setUrlValuesToArray(g, vs, p)
	case ct == "multipart/form-data": //, "multipart/mixed": // should we allow mixed?
		boundary, ok := params["boundary"]
		if !ok {
			return errors.New("http: POST form-data missing boundary")
		}
		read := multipart.NewReader(g.req.Body, boundary)

		for {
			part, err := read.NextPart()
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}

			k := part.FormName()
			fn := part.FileName()
			if fn != "" {
				// THIS IS A FILE
				// TODO
				continue
			}
			if k == "" {
				// TODO what should we do with these?
				continue
			}

			b := &bytes.Buffer{}
			_, err = g.mem.Copy(b, part) // count size against memory usage
			if err != nil {
				return err
			}

			err = setUrlValueToArray(g, k, ZString(b.Bytes()), p)
			if err != nil {
				return err
			}
		}
		return nil
	default:
		return errors.New("Failed to parse POST: unknown content type")
	}
}

func parseQueryToArray(ctx Context, q string, a *ZArray) error {
	// parse this ourselves instead of using url.Values so we can keep the order right
	for len(q) > 0 {
		p := strings.IndexByte(q, '&')
		if p == -1 {
			return parseQueryFragmentToArray(ctx, q, a)
		} else {
			err := parseQueryFragmentToArray(ctx, q[:p], a)
			if err != nil {
				return err
			}
			q = q[p+1:]
		}
	}
	return nil
}

func parseQueryFragmentToArray(ctx Context, f string, a *ZArray) error {
	p := strings.IndexByte(f, '=')
	if p == -1 {
		f, _ = url.QueryUnescape(f) // ignore errors
		return setUrlValueToArray(ctx, f, ZNULL, a)
	}
	k, _ := url.QueryUnescape(f[:p])
	f, _ = url.QueryUnescape(f[p+1:])
	return setUrlValueToArray(ctx, k, ZString(f), a)
}

func setUrlValuesToArray(ctx Context, vs url.Values, a *ZArray) error {
	for k, s := range vs {
		for _, v := range s {
			err := setUrlValueToArray(ctx, k, ZString(v), a)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func setUrlValueToArray(ctx Context, k string, v Val, a *ZArray) error {
	p := strings.IndexByte(k, '[')
	if p == -1 {
		// simple
		return a.OffsetSet(ctx, ZString(k).ZVal(), v.ZVal())
	}
	if p == 0 {
		// failure
		return errors.New("invalid key")
	}

	var n *ZArray
	zk := ZString(k[:p]).ZVal()

	if has, err := a.OffsetExists(ctx, zk); err != nil {
		return err
	} else if has {
		z, err := a.OffsetGet(ctx, zk)
		if err != nil {
			return err
		}
		z, err = z.As(ctx, ZtArray)
		if err != nil {
			return err
		}
		n = z.Value().(*ZArray)
	} else {
		n = NewZArray()
		err = a.OffsetSet(ctx, zk, n.ZVal())
		if err != nil {
			return err
		}
	}

	// loop through what remains of k
	k = k[p:]

	for {
		if len(k) == 0 {
			break
		}
		if k[0] != '[' {
			// php will ignore data after last bracket
			break
		}
		k = k[1:]
		p = strings.IndexByte(k, ']')
		if p == -1 {
			break // php will ignore data after last bracket
		}
		if p == 0 {
			// append
			xn := NewZArray()
			n.OffsetSet(ctx, nil, xn.ZVal())
			n = xn
			k = k[1:]
			continue
		}

		zk = ZString(k[:p]).ZVal()
		k = k[p+1:]

		if len(k) == 0 {
			// perform set
			break
		}

		if has, err := n.OffsetExists(ctx, zk); err != nil {
			return err
		} else if has {
			z, err := n.OffsetGet(ctx, zk)
			if err != nil {
				return err
			}
			z, err = z.As(ctx, ZtArray)
			if err != nil {
				return err
			}
			n = z.Value().(*ZArray)
		} else {
			xn := NewZArray()
			err = n.OffsetSet(ctx, zk, xn.ZVal())
			if err != nil {
				return err
			}
		}
	}
	return n.OffsetSet(ctx, zk, v.ZVal())
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
