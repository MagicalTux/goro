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
	"strings"
	"time"

	"github.com/MagicalTux/goro/core/locale"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
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

	p     *Process
	start time.Time // time at which this request started
	req   *http.Request
	h     *phpv.ZHashTable
	l     *phpv.Loc
	mem   *MemMgr

	deadlineDuration time.Duration
	timerStart       time.Time

	IniConfig phpv.IniConfig

	// this is the actual environment (defined functions, classes, etc)
	globalInternalFuncs map[phpv.ZString]phpv.Callable
	globalUserFuncs     map[phpv.ZString]phpv.Callable
	disabledFuncs       map[phpv.ZString]struct{}

	globalClasses map[phpv.ZString]*phpobj.ZClass // TODO replace *ZClass with a nice interface
	shutdownFuncs []phpv.Callable
	constant      map[phpv.ZString]phpv.Val
	environ       *phpv.ZHashTable
	included      map[phpv.ZString]bool // included files (used for require_once, etc)
	includePath   []string              // TODO: initialize

	streamHandlers map[string]stream.Handler
	fileHandler    *stream.FileHandler

	globalLazyFunc  map[phpv.ZString]*globalLazyOffset
	globalLazyClass map[phpv.ZString]*globalLazyOffset

	errOut      io.Writer
	out         io.Writer
	buf         *Buffer
	lastOutChar byte

	rand *random.State

	shownDeprecated map[string]struct{}

	userErrorHandler phpv.Callable
	userErrorFilter  phpv.PhpErrorType
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

func createGlobal(p *Process) *Global {
	g := &Global{
		p:                   p,
		out:                 os.Stdout,
		errOut:              os.Stderr,
		rand:                random.New(),
		start:               time.Now(),
		h:                   phpv.NewHashTable(),
		l:                   &phpv.Loc{Filename: "unknown", Line: 1},
		globalInternalFuncs: make(map[phpv.ZString]phpv.Callable),
		globalUserFuncs:     make(map[phpv.ZString]phpv.Callable),
		disabledFuncs:       make(map[phpv.ZString]struct{}),
		globalClasses:       make(map[phpv.ZString]*phpobj.ZClass),
		constant:            make(map[phpv.ZString]phpv.Val),
		streamHandlers:      make(map[string]stream.Handler),
		included:            make(map[phpv.ZString]bool),
		globalLazyFunc:      make(map[phpv.ZString]*globalLazyOffset),
		globalLazyClass:     make(map[phpv.ZString]*globalLazyOffset),
		shownDeprecated:     make(map[string]struct{}),
		mem:                 NewMemMgr(32 * 1024 * 1024), // limit in bytes TODO read memory_limit from process (.ini file)

	}
	g.SetDeadline(g.start.Add(30 * time.Second))

	g.fileHandler, _ = stream.NewFileHandler("/")
	g.streamHandlers["file"] = g.fileHandler
	g.streamHandlers["php"] = stream.PhpHandler()

	g.initLocale()

	return g
}

func (g *Global) initLocale() {
	locale.SetLocale(locale.LC_ALL, "")
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
	g.constant["STDIN"] = stream.Stdin
	g.constant["STDOUT"] = stream.Stdout
	g.constant["STDERR"] = stream.Stderr

	// import global funcs & classes from ext
	for _, e := range globalExtMap {
		for k, v := range e.Functions {
			g.globalInternalFuncs[phpv.ZString(k)] = v
		}
		for _, c := range e.Classes {
			// TODO: use class ID for comparing classes
			// copying the class here will break class comparison
			//
			// copy c since class state (i.e. next instance id)
			// should be per context global, and not Go global
			//classCopy := *c
			g.globalClasses[c.GetName().ToLower()] = c
		}
	}

	// get env from process
	g.environ = g.p.environ.Dup()

	g.setupIni()
	g.doGPC()
}

func (g *Global) setupIni() {
	options := g.p.Options
	cfg := g.IniConfig

	cfg.LoadDefaults(g)

	if options.IniFile != "" {
		file, err := os.Open(options.IniFile)
		if err != nil {
			println("error:", err.Error())
			os.Exit(1)
		}
		defer file.Close()
		if err = cfg.Parse(g, file); err != nil {
			println("error:", err.Error())
			os.Exit(1)
		}
	}
	for k, v := range options.IniEntries {
		val, err := cfg.EvalConfigValue(g, phpv.ZString(v))
		if err != nil {
			val = phpv.ZString(v).ZVal()
		}
		cfg.SetGlobal(g, phpv.ZString(k), val)

	}
	if v := cfg.Get(phpv.ZString("disable_functions")); v != nil {
		for _, name := range strings.Split(string(v.GetString(g)), ",") {
			name = strings.TrimSpace(name)
			g.disabledFuncs[phpv.ZString(name)] = struct{}{}
		}
	}
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
	err = phpv.FilterExitError(err)

	switch innerErr := phpv.UnwrapError(err).(type) {
	case *phpv.PhpExit:
	case *phperr.PhpTimeout:
		if g.GetConfig("display_errors", phpv.ZFalse.ZVal()).AsBool(g) {
			g.WriteErr([]byte(innerErr.String()))
		}
	default:
		if err != nil {
			return err
		}
	}

	if len(g.shutdownFuncs) > 0 {
		g.ResetDeadline()
		for _, fn := range g.shutdownFuncs {
			_, err := g.CallZVal(g, fn, nil, nil)
			if err != nil {
				if phpv.IsExit(err) {
					break
				}
				if timeout, ok := phpv.UnwrapError(err).(*phperr.PhpTimeout); ok {
					g.WriteErr([]byte(timeout.String()))
					break
				}
				return err
			}
		}
	}

	return g.Close()
}

func (g *Global) Write(v []byte) (int, error) {
	if len(v) > 0 {
		g.lastOutChar = v[len(v)-1]
	}
	return g.out.Write(v)
}

func (g *Global) WriteErr(v []byte) (int, error) {
	return g.errOut.Write(v)
}

func (g *Global) RestoreConfig(name phpv.ZString) {
	g.IniConfig.RestoreConfig(g, name)
}

func (g *Global) SetLocalConfig(name phpv.ZString, value *phpv.ZVal) (*phpv.ZVal, bool) {
	if !g.IniConfig.CanIniSet(name) {
		return nil, false

	}
	old := g.IniConfig.SetLocal(g, name, value)
	return old, true
}

func (g *Global) GetConfig(name phpv.ZString, def *phpv.ZVal) *phpv.ZVal {
	val := g.IniConfig.Get(name)
	if val != nil && val.Get() != nil {
		return val.Get()
	}
	return def
}

func (g *Global) GetGlobalConfig(name phpv.ZString, def *phpv.ZVal) *phpv.ZVal {
	val := g.IniConfig.Get(name)
	if val != nil && val.Global != nil {
		return val.Global
	}
	return def
}

func (g *Global) IterateConfig() iter.Seq2[string, phpv.IniValue] {
	return g.IniConfig.IterateConfig()
}

func (g *Global) Tick(ctx phpv.Context, l *phpv.Loc) error {
	// TODO check run deadline, context cancellation and memory limit
	deadline := g.timerStart.Add(g.deadlineDuration)
	// println("deadline", deadline.String())
	if time.Until(deadline) <= 0 {
		return &phperr.PhpTimeout{L: g.l, Seconds: int(g.deadlineDuration.Seconds())}
	}
	g.l = l
	return nil
}

func (g *Global) Deadline() (time.Time, bool) {
	deadline := g.timerStart.Add(g.deadlineDuration)
	return deadline, true
}

func (g *Global) SetDeadline(t time.Time) {
	g.timerStart = time.Now()
	g.deadlineDuration = t.Sub(g.timerStart)
}

func (g *Global) ResetDeadline() {
	g.timerStart = time.Now()
}

func (g *Global) Loc() *phpv.Loc {
	return g.l
}

func (g *Global) Error(err error, t ...phpv.PhpErrorType) error {
	wrappedErr := g.l.Error(g, err, t...)
	return phperr.HandleUserError(g, wrappedErr)
}

func (g *Global) Errorf(format string, a ...any) error {
	err := g.l.Errorf(g, phpv.E_ERROR, format, a...)
	return phperr.HandleUserError(g, err)
}

func (g *Global) FuncError(err error, t ...phpv.PhpErrorType) error {
	wrappedErr := g.l.Error(g, err, t...)
	wrappedErr.FuncName = g.GetFuncName()
	return phperr.HandleUserError(g, wrappedErr)
}
func (g *Global) FuncErrorf(format string, a ...any) error {
	err := g.l.Errorf(g, phpv.E_ERROR, format, a...)
	err.FuncName = g.GetFuncName()
	return phperr.HandleUserError(g, err)
}

func (g *Global) Warn(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_WARNING))
	return logWarning(g, format, a...)
}

func (g *Global) Notice(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_NOTICE))
	return logWarning(g, "\n"+format, a...)
}

func (g *Global) Deprecated(format string, a ...any) error {
	format = "\n" + format
	a = append(a, logopt.ErrType(phpv.E_DEPRECATED))
	err := logWarning(g, format, a...)
	if err == nil {
		g.ShownDeprecated(format)
	}
	return err
}

func (g *Global) WarnDeprecated() error {
	funcName := g.GetFuncName()
	if ok := g.ShownDeprecated(funcName); ok {
		err := logWarning(
			g,
			"\nThe %s() function is deprecated. This message will be suppressed on further calls",
			funcName, logopt.NoFuncName(true), logopt.ErrType(phpv.E_DEPRECATED),
		)
		return err
	}
	return nil
}

func getLogArgs(args []any) (logopt.Data, []any) {
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

func logWarning(ctx phpv.Context, format string, a ...any) error {
	funcName := ctx.GetFuncName()
	loc := ctx.Loc()
	option, fmtArgs := getLogArgs(a)
	message := fmt.Sprintf(format, fmtArgs...)

	phpErr := &phpv.PhpError{
		Err:      errors.New(message),
		FuncName: funcName,
		Code:     phpv.PhpErrorType(option.ErrType),
		Loc:      loc,
	}

	err := phperr.HandleUserError(ctx, phpErr)
	if err != nil {
		return err
	}

	errorLevel := ctx.GetConfig("error_reporting", phpv.ZInt(0).ZVal()).AsInt(ctx)
	logError := int(errorLevel)&int(phpErr.Code) > 0

	if logError {
		ctx.Global().LogError(phpErr, option)
	}

	return nil
}

func (g *Global) LogError(err *phpv.PhpError, optionArg ...logopt.Data) {
	var option logopt.Data
	if len(optionArg) > 0 {
		option = optionArg[0]
	}

	var output bytes.Buffer
	switch err.Code {
	case phpv.E_ERROR:
		output.WriteString("Fatal error")
	case phpv.E_WARNING:
		output.WriteString("Warning")
	case phpv.E_NOTICE:
		output.WriteString("Notice")
	case phpv.E_DEPRECATED:
		output.WriteString("Deprecated")
	default:
		output.WriteString("Info")
	}

	output.WriteString(": ")
	if !option.NoFuncName && err.FuncName != "" {
		output.WriteString(fmt.Sprintf("%s(): ", err.FuncName))
	}
	output.WriteString(err.Err.Error())
	if !option.NoLoc {
		output.WriteString(fmt.Sprintf(" in %s on line %d", err.Loc.Filename, err.Loc.Line))
	}

	if (g.lastOutChar != '\n' || err.Code == phpv.E_WARNING) && err.Code != phpv.E_NOTICE {
		g.Write([]byte("\n"))
	}

	g.Write(output.Bytes())
	g.Write([]byte("\n"))
}

func (g *Global) GetFuncName() string {
	return ""
}

func (g *Global) Func() phpv.FuncContext {
	return nil
}

func (g *Global) This() phpv.ZObject {
	return nil
}
func (g *Global) Class() phpv.ZClass {
	return nil
}

func (g *Global) RegisterFunction(name phpv.ZString, f phpv.Callable) error {
	name = name.ToLower()
	if _, exists := g.globalUserFuncs[name]; exists {
		return g.Errorf("duplicate function name in declaration")
	}
	g.globalUserFuncs[name] = f
	delete(g.globalLazyFunc, name)
	return nil
}

func (g *Global) RegisterShutdownFunction(f phpv.Callable) {
	g.shutdownFuncs = append(g.shutdownFuncs, f)
}

func (g *Global) GetFunction(ctx phpv.Context, name phpv.ZString) (phpv.Callable, error) {
	if f, ok := g.globalUserFuncs[name.ToLower()]; ok {
		return f, nil
	}
	if f, ok := g.globalInternalFuncs[name.ToLower()]; ok {
		if _, ok := g.disabledFuncs[name]; ok {
			ctx.Warn("%s() has been disabled for security reasons",
				name, logopt.NoFuncName(true))
			return noOp, nil
		}
		return f, nil
	}
	if f, ok := g.globalLazyFunc[name.ToLower()]; ok {
		_, err := f.r[f.p].Run(ctx)
		if err != nil {
			return nil, err
		}
		f.r[f.p] = phpv.RunNull{} // remove function declaration from tree now that his as been run
		if f, ok := g.globalUserFuncs[name.ToLower()]; ok {
			return f, nil
		}
	}

	return nil, g.Errorf("Call to undefined function %s", name)
}

func (g *Global) GetDefinedFunctions(ctx phpv.Context, excludeDisabled bool) (*phpv.ZArray, error) {
	result := phpv.NewZArray()
	user := phpv.NewZArray()
	internal := phpv.NewZArray()

	result.OffsetSet(ctx, phpv.ZStr("user"), user.ZVal())
	result.OffsetSet(ctx, phpv.ZStr("internal"), internal.ZVal())

	for k := range g.globalUserFuncs {
		user.OffsetSet(ctx, nil, k.ZVal())
	}

	if excludeDisabled {
		for k := range g.globalInternalFuncs {
			if _, ok := g.disabledFuncs[k]; !ok {
				internal.OffsetSet(ctx, nil, k.ZVal())
			}
		}
	} else {
		for k := range g.globalInternalFuncs {
			internal.OffsetSet(ctx, nil, k.ZVal())
		}
	}

	return result, nil
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

func (g *Global) GetScriptFile() phpv.ZString {
	return phpv.ZString(g.p.ScriptFilename)
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

func (g *Global) GetUserErrorHandler() (phpv.Callable, phpv.PhpErrorType) {
	return g.userErrorHandler, g.userErrorFilter
}

func (g *Global) SetUserErrorHandler(handler phpv.Callable, filter phpv.PhpErrorType) {
	g.userErrorHandler = handler
	g.userErrorFilter = filter
}

func (g *Global) GetStackTrace(ctx phpv.Context) []*phpv.StackTraceEntry {
	var context phpv.Context = ctx
	var trace []*phpv.StackTraceEntry
	for context != nil {
		if fc, ok := context.(*FuncContext); ok {
			var className string
			if fc.class != nil {
				className = string(fc.class.GetName())
			}
			trace = append(trace, &phpv.StackTraceEntry{
				FuncName:   fc.GetFuncName(),
				Filename:   fc.loc.Filename,
				ClassName:  className,
				MethodType: fc.methodType,
				Line:       fc.loc.Line,
				Args:       fc.Args,
			})
		}
		context = context.Parent(1)
	}
	return trace
}

func (g *Global) ShownDeprecated(key string) bool {
	_, exists := g.shownDeprecated[key]
	g.shownDeprecated[key] = struct{}{}
	return !exists
}
