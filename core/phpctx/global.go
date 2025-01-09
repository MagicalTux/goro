package phpctx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"log"
	"maps"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/random"
	"github.com/MagicalTux/goro/core/stream"
)

type globalLazyOffset struct {
	r phpv.Runnables
	p int
}

type Global struct {
	context.Context

	p        *Process
	start    time.Time // time at which this request started
	req      *http.Request
	h        *phpv.ZHashTable
	l        *phpv.Loc
	mem      *MemMgr
	deadline time.Time

	IniConfig phpv.IniConfig

	// this is the actual environment (defined functions, classes, etc)
	globalFuncs   map[phpv.ZString]phpv.Callable
	globalClasses map[phpv.ZString]*phpobj.ZClass // TODO replace *ZClass with a nice interface
	constant      map[phpv.ZString]phpv.Val
	environ       *phpv.ZHashTable
	fHandler      map[string]stream.Handler
	included      map[phpv.ZString]bool // included files (used for require_once, etc)

	callStack []phpv.Callable

	globalLazyFunc  map[phpv.ZString]*globalLazyOffset
	globalLazyClass map[phpv.ZString]*globalLazyOffset

	errOut io.Writer
	out    io.Writer
	buf    *Buffer

	rand *random.State

	shownDeprecated map[string]struct{}
}

func NewGlobal(ctx context.Context, p *Process, config phpv.IniConfig) *Global {
	res := createGlobal(p)
	res.Context = ctx
	res.IniConfig = config
	res.init()

	return res
}

func NewGlobalReq(req *http.Request, p *Process, config phpv.IniConfig) *Global {
	res := createGlobal(p)
	res.Context = req.Context()
	res.IniConfig = config
	res.req = req
	res.init()
	return res
}

func NewIniContext(config phpv.IniConfig) *Global {
	p := NewProcess("ini")
	res := createGlobal(p)
	res.Context = context.Background()
	res.IniConfig = config

	for k, v := range res.p.defaultConstants {
		res.constant[k] = v
	}
	return res
}

func createGlobal(p *Process) *Global {
	g := &Global{
		p:               p,
		out:             os.Stdout,
		errOut:          os.Stderr,
		rand:            random.New(),
		start:           time.Now(),
		h:               phpv.NewHashTable(),
		l:               &phpv.Loc{Filename: "unknown", Line: 1},
		globalFuncs:     make(map[phpv.ZString]phpv.Callable),
		globalClasses:   make(map[phpv.ZString]*phpobj.ZClass),
		constant:        make(map[phpv.ZString]phpv.Val),
		fHandler:        make(map[string]stream.Handler),
		included:        make(map[phpv.ZString]bool),
		globalLazyFunc:  make(map[phpv.ZString]*globalLazyOffset),
		globalLazyClass: make(map[phpv.ZString]*globalLazyOffset),
		shownDeprecated: make(map[string]struct{}),
		mem:             NewMemMgr(32 * 1024 * 1024), // limit in bytes TODO read memory_limit from process (.ini file)
	}
	g.deadline = g.start.Add(30 * time.Second) // deadline
	g.fHandler["file"], _ = stream.NewFileHandler("/")
	g.fHandler["php"] = stream.PhpHandler()

	return g
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
	// fill constants from process
	for k, v := range g.p.defaultConstants {
		g.constant[k] = v
	}

	// import global funcs & classes from ext
	for _, e := range globalExtMap {
		for k, v := range e.Functions {
			g.globalFuncs[phpv.ZString(k)] = v
		}
		for _, c := range e.Classes {
			// copy c since class state (i.e. next instance id)
			// should be per context global, and not Go global
			classCopy := *c
			g.globalClasses[c.GetName().ToLower()] = &classCopy
		}
	}

	// get env from process
	g.environ = g.p.environ.Dup()

	g.doGPC()

}

func (g *Global) doGPC() {
	// initialize superglobals
	get := phpv.NewZArray()
	p := phpv.NewZArray()
	c := phpv.NewZArray()
	r := phpv.NewZArray()
	s := phpv.NewZArray()
	e := phpv.NewZArray() // initialize empty
	f := phpv.NewZArray()

	order := g.GetConfig("variables_order", phpv.ZString("EGPCS").ZVal()).String()

	for _, l := range order {
		switch l {
		case 'e', 'E':
			s.MergeTable(g.environ)
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
			s.OffsetSet(g, phpv.ZString("REQUEST_TIME").ZVal(), phpv.ZInt(g.start.Unix()).ZVal())
			s.OffsetSet(g, phpv.ZString("REQUEST_TIME_FLOAT").ZVal(), phpv.ZFloat(float64(g.start.UnixNano())/1e9).ZVal())

			args := phpv.NewZArray()
			for _, elem := range g.p.Argv {
				args.OffsetSet(g, nil, phpv.ZStr(elem))
			}

			argv := args.ZVal()
			argc := args.Count(g).ZVal()
			s.OffsetSet(g, phpv.ZString("argv"), argv)
			s.OffsetSet(g, phpv.ZString("argc"), argc)

			s.OffsetSet(g, phpv.ZString("PHP_SELF"), phpv.ZStr(g.p.ScriptFilename))

			g.h.SetString("argv", argv)
			g.h.SetString("argc", argc)

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
	_, err := g.Require(g, phpv.ZString(fn))
	err = phpv.FilterError(err)
	if err != nil {
		return err
	}
	return g.Close()
}

func (g *Global) Write(v []byte) (int, error) {
	return g.out.Write(v)
}

func (g *Global) writeErr(v []byte) (int, error) {
	return g.errOut.Write(v)
}

func (g *Global) SetLocalConfig(name phpv.ZString, val *phpv.ZVal) error {
	g.IniConfig.SetLocal(name, val.ZVal())
	return nil
}

func (g *Global) GetConfig(name phpv.ZString, def *phpv.ZVal) *phpv.ZVal {
	val := g.IniConfig.Get(name)
	if val != nil {
		if val.Local != nil {
			return val.Local
		}
		if val.Global != nil {
			return val.Global
		}
		return phpv.ZNULL.ZVal()
	}
	return def
}

func (g *Global) IterateConfig() iter.Seq2[string, phpv.IniValue] {
	return g.IniConfig.IterateConfig()
}

func (g *Global) Tick(ctx phpv.Context, l *phpv.Loc) error {
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

func (g *Global) Loc() *phpv.Loc {
	return g.l
}

func (g *Global) Error(err error, t ...phpv.PhpErrorType) error {
	return g.l.Error(err, t...)
}

func (g *Global) Errorf(format string, a ...any) error {
	return g.l.Errorf(phpv.E_ERROR, format, a...)
}

func (g *Global) FuncError(err error, t ...phpv.PhpErrorType) error {
	wrappedErr := g.l.Error(err, t...)
	wrappedErr.FuncName = g.GetFuncName()
	return wrappedErr
}
func (g *Global) FuncErrorf(format string, a ...any) error {
	err := g.l.Errorf(phpv.E_ERROR, format, a...)
	err.FuncName = g.GetFuncName()
	return err
}

func (g *Global) Warn(format string, a ...any) {
	a = append(a, logopt.ErrType(phpv.E_WARNING))
	g.log(format, a...)
}

func (g *Global) Notice(format string, a ...any) {
	g.writeErr([]byte{'\n'})
	a = append(a, logopt.ErrType(phpv.E_NOTICE))
	g.log(format, a...)
}

func (g *Global) WarnDeprecated() {
	funcName := g.GetFuncName()
	if _, ok := g.shownDeprecated[funcName]; !ok {
		g.writeErr([]byte{'\n'})
		g.log("The %s() function is deprecated. This message will be suppressed on further calls", funcName, logopt.NoFuncName(true), logopt.ErrType(phpv.E_DEPRECATED))
		g.shownDeprecated[funcName] = struct{}{}
	}
}

func (g *Global) getLogArgs(args []any) (logopt.Data, []any) {
	var fmtArgs []any
	var option logopt.Data
	for _, arg := range args {
		switch t := arg.(type) {
		case logopt.ErrType:
			option.ErrType = int(t)
		case logopt.NoFuncName:
			option.NoFuncName = bool(t)
		case logopt.NoLoc:
			option.NoLoc = bool(t)
		case logopt.Data:
			option.ErrType = t.ErrType
			option.NoFuncName = t.NoFuncName
			option.NoLoc = t.NoLoc
		default:
			fmtArgs = append(fmtArgs, arg)
		}
	}
	return option, fmtArgs
}

func (g *Global) log(format string, a ...any) {
	funcName := g.GetFuncName()
	loc := g.l.Loc()

	option, fmtArgs := g.getLogArgs(a)

	var output bytes.Buffer
	switch option.ErrType {
	case int(phpv.E_WARNING):
		output.WriteString("Warning")
	case int(phpv.E_NOTICE):
		output.WriteString("Notice")
	case int(phpv.E_DEPRECATED):
		output.WriteString("Deprecated")
	default:
		output.WriteString("Info")
	}
	output.WriteString(": ")
	if !option.NoFuncName {
		output.WriteString(fmt.Sprintf("%s(): ", funcName))
	}
	output.WriteString(fmt.Sprintf(format, fmtArgs...))
	if !option.NoLoc {
		output.WriteString(fmt.Sprintf(" in %s on line %d", loc.Filename, loc.Line))
	}

	g.writeErr([]byte(output.String()))
	g.writeErr([]byte("\n"))
}

func (g *Global) GetFuncName() string {
	if len(g.callStack) > 0 {
		return g.callStack[len(g.callStack)-1].Name()
	}
	return ""
}

func (g *Global) Func() phpv.FuncContext {
	return nil
}

func (g *Global) This() phpv.ZObject {
	return nil
}

func (g *Global) RegisterFunction(name phpv.ZString, f phpv.Callable) error {
	name = name.ToLower()
	if _, exists := g.globalFuncs[name]; exists {
		return g.Errorf("duplicate function name in declaration")
	}
	g.globalFuncs[name] = f
	delete(g.globalLazyFunc, name)
	return nil
}

func (g *Global) GetFunction(ctx phpv.Context, name phpv.ZString) (phpv.Callable, error) {
	if f, ok := g.globalFuncs[name.ToLower()]; ok {
		return f, nil
	}
	if f, ok := g.globalLazyFunc[name.ToLower()]; ok {
		_, err := f.r[f.p].Run(ctx)
		if err != nil {
			return nil, err
		}
		f.r[f.p] = phpv.RunNull{} // remove function declaration from tree now that his as been run
		if f, ok := g.globalFuncs[name.ToLower()]; ok {
			return f, nil
		}
	}

	return nil, g.Errorf("Call to undefined function %s", name)
}

func (g *Global) ConstantGet(name phpv.ZString) (phpv.Val, bool) {
	if v, ok := g.constant[name]; ok {
		return v, true
	}
	return nil, false
}

func (g *Global) ConstantSet(k phpv.ZString, v phpv.Val) bool {
	if _, ok := g.constant[k]; ok {
		return false
	}

	g.constant[k] = v
	return true
}

func (g *Global) GetClass(ctx phpv.Context, name phpv.ZString, autoload bool) (phpv.ZClass, error) {
	switch name {
	case "self":
		// check for func
		f := ctx.Func().(*FuncContext)
		if f == nil {
			return nil, ctx.Errorf("Cannot access self:: when no method scope is active")
		}
		cfunc, ok := f.c.(phpv.ZClosure)
		if !ok || cfunc.GetClass() == nil {
			return nil, ctx.Errorf("Cannot access self:: when no class scope is active")
		}
		return cfunc.GetClass(), nil
	case "parent":
		// check for func
		f := ctx.Func().(*FuncContext)
		if f == nil {
			return nil, ctx.Errorf("Cannot access parent:: when no method scope is active")
		}
		cfunc, ok := f.c.(phpv.ZClosure)
		if !ok || cfunc.GetClass() == nil {
			return nil, ctx.Errorf("Cannot access parent:: when no class scope is active")
		}
		if cfunc.GetClass().GetParent() == nil {
			return nil, ctx.Errorf("Cannot access parent:: when current class scope has no parent")
		}
		return cfunc.GetClass().GetParent(), nil
	case "static":
		// check for func
		f := ctx.Func().(*FuncContext)
		if f == nil || f.this == nil {
			return nil, ctx.Errorf("Cannot access static:: when no class scope is active")
		}
		return f.this.GetClass(), nil
	}
	if c, ok := g.globalClasses[name.ToLower()]; ok {
		return c, nil
	}
	if r, ok := g.globalLazyClass[name.ToLower()]; ok {
		_, err := r.r[r.p].Run(ctx)
		if err != nil {
			return nil, err
		}
		r.r[r.p] = phpv.RunNull{} // remove function declaration from tree now that his as been run
		if c, ok := g.globalClasses[name.ToLower()]; ok {
			return c, nil
		}
	}
	// TODO if autoload { do autoload }
	return nil, ctx.Errorf("Class '%s' not found", name)
}

func (g *Global) RegisterClass(name phpv.ZString, c phpv.ZClass) error {
	name = name.ToLower()
	if _, ok := g.globalClasses[name]; ok {
		return fmt.Errorf("Cannot declare class %s, because the name is already in use", name)
	}
	g.globalClasses[name] = c.(*phpobj.ZClass)
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

func (g *Global) Argv() []string {
	return g.p.Argv
}

func (g *Global) RegisterLazyFunc(name phpv.ZString, r phpv.Runnables, p int) {
	g.globalLazyFunc[name.ToLower()] = &globalLazyOffset{r, p}
}

func (g *Global) RegisterLazyClass(name phpv.ZString, r phpv.Runnables, p int) {
	g.globalLazyClass[name.ToLower()] = &globalLazyOffset{r, p}
}

func (g *Global) Global() phpv.GlobalContext {
	return g
}

func (g *Global) MemAlloc(ctx phpv.Context, s uint64) error {
	return g.mem.Alloc(ctx, s)
}

func (g *Global) GetLoadedExtensions() []string {
	return slices.Collect(maps.Keys(globalExtMap))
}

func (g *Global) Random() *random.State {
	return g.rand
}
