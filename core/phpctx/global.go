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
	"math"
	"net/http"
	"os"
	"path/filepath"
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

type errorHandlerEntry struct {
	handler     phpv.Callable
	filter      phpv.PhpErrorType
	originalVal *phpv.ZVal // original ZVal passed to set_error_handler
}

type exceptionHandlerEntry struct {
	handler     phpv.Callable
	originalVal *phpv.ZVal // original ZVal passed to set_exception_handler
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
	tickCount        uint32
	baselineAlloc    uint64 // runtime.MemStats.Alloc at script start
	lastMemCheck     uint64 // last observed Alloc from runtime

	IniConfig phpv.IniConfig

	// this is the actual environment (defined functions, classes, etc)
	globalInternalFuncs map[phpv.ZString]phpv.Callable
	globalUserFuncs     map[phpv.ZString]phpv.Callable
	disabledFuncs       map[phpv.ZString]struct{}

	globalClasses   map[phpv.ZString]*phpobj.ZClass // TODO replace *ZClass with a nice interface
	classOrigNames  map[phpv.ZString]phpv.ZString  // lowercase -> original-case name for class aliases
	classOrder      []phpv.ZString                 // ordered list of class names (lowercase) for get_declared_classes()
	shutdownFuncs []phpv.Callable
	callDepth     int
	constant      map[phpv.ZString]phpv.Val
	constantAttrs map[phpv.ZString][]*phpv.ZAttribute // attributes on global constants
	environ       *phpv.ZHashTable
	included      map[phpv.ZString]bool // included files (used for require_once, etc)
	includePath   []string              // TODO: initialize

	streamHandlers      map[string]stream.Handler
	fileHandler         *stream.FileHandler
	StreamFilterRegistry *stream.FilterRegistry

	globalLazyFunc  map[phpv.ZString]*globalLazyOffset
	globalLazyClass map[phpv.ZString]*globalLazyOffset

	errOut        io.Writer
	out           io.Writer
	buf           *Buffer
	lastOutChar   byte
	ImplicitFlush bool

	rand *random.State

	shownDeprecated map[string]struct{}

	userErrorHandlerStack  []errorHandlerEntry
	userExceptionHandlerStack []exceptionHandlerEntry

	autoloadFuncs    []phpv.Callable
	autoloadingClass map[phpv.ZString]bool // prevent infinite recursion in autoload
	autoloadExts     string                // file extensions for spl_autoload (default ".inc,.php")

	header *phpv.HeaderContext

	nextResourceID int
	nextObjectID   int
	freeObjectIDs  []int // recycled object IDs (free list)

	DefaultStreamContext *stream.Context

	destructObjects []phpv.ZObject // objects with __destruct to call at shutdown

	compilingClass              phpv.ZClass
	noDiscardPending            bool // class currently being compiled (for self:: resolution)
	nextCallSuppressCalledIn    bool // suppress "called in" for next Call

	rawRequestBody []byte // stored POST body for php://input

	customStdin *stream.Stream // custom stdin for testing

	startupWarnings []byte // warnings from request startup (before output is set)

	tempFiles     []string            // temporary files to clean up (e.g., uploaded files)
	uploadedFiles map[string]struct{} // set of uploaded file paths for is_uploaded_file()
	obDisabled    bool                // OB system disabled after re-entrant fatal error

	lastCallable phpv.Callable // for NoDiscard checks

	// Last error tracked for error_get_last() / error_clear_last()
	LastError *phpv.PhpError

	// Tick functions for declare(ticks=N)
	tickFuncs []tickFuncEntry

	// JSON encoding recursion detection across nested json_encode calls
	jsonEncodingObjects map[phpv.ZObject]bool

	// Serialize recursion detection across nested serialize calls
	// (especially Serializable::serialize() calling serialize() internally)
	SerializeSeenObjects map[phpv.ZObject]bool

	// StrictTypes tracks whether the currently executing file has declare(strict_types=1).
	// This is a per-file flag that affects type coercion at call sites.
	StrictTypes bool
}

type tickFuncEntry struct {
	callable phpv.Callable
	args     []*phpv.ZVal // extra args passed to register_tick_function
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
		classOrigNames:      make(map[phpv.ZString]phpv.ZString),
		constant:            make(map[phpv.ZString]phpv.Val),
		constantAttrs:       make(map[phpv.ZString][]*phpv.ZAttribute),
		streamHandlers:      make(map[string]stream.Handler),
		included:            make(map[phpv.ZString]bool),
		globalLazyFunc:      make(map[phpv.ZString]*globalLazyOffset),
		globalLazyClass:     make(map[phpv.ZString]*globalLazyOffset),
		shownDeprecated:     make(map[string]struct{}),
		mem:                 NewMemMgr(134217728), // 128MB default (PHP default memory_limit)

		header: &phpv.HeaderContext{Headers: http.Header{}},

		// the first 3 are reserved for STDIN, STDOUT and STDERR
		nextResourceID: 4,
	}
	g.SetDeadline(g.start.Add(30 * time.Second))

	// Record baseline memory so Tick() can measure per-script usage
	g.InitBaselineMemory()

	g.fileHandler, _ = stream.NewFileHandler("/")
	g.streamHandlers["file"] = g.fileHandler
	g.streamHandlers["php"] = stream.PhpHandler()
	g.streamHandlers["http"] = stream.NewHttpHandler()
	g.streamHandlers["data"] = stream.DataHandler
	g.StreamFilterRegistry = stream.NewFilterRegistry()

	g.initLocale()

	return g
}

func (g *Global) initLocale() {
	locale.SetLocale(locale.LC_ALL, "")
}

func (g *Global) RegisterStreamHandler(scheme string, handler stream.Handler) {
	g.streamHandlers[scheme] = handler
}

// GetFilterRegistry returns the per-request stream filter registry.
// Implements stream.FilterRegistryProvider.
func (g *Global) GetFilterRegistry() *stream.FilterRegistry {
	if g.StreamFilterRegistry == nil {
		g.StreamFilterRegistry = stream.NewFilterRegistry()
	}
	return g.StreamFilterRegistry
}

// HasStreamHandler checks if a scheme has a registered stream handler.
func (g *Global) HasStreamHandler(scheme string) bool {
	_, ok := g.streamHandlers[scheme]
	return ok
}

// GetStreamHandler returns the registered stream handler for a scheme.
func (g *Global) GetStreamHandler(scheme string) (stream.Handler, bool) {
	h, ok := g.streamHandlers[scheme]
	return h, ok
}

func (g *Global) UnregisterStreamHandler(scheme string) bool {
	if _, ok := g.streamHandlers[scheme]; !ok {
		return false
	}
	delete(g.streamHandlers, scheme)
	return true
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

	// Mark E_STRICT as deprecated (PHP 8.4+)
	g.ConstantSetAttributes("E_STRICT", []*phpv.ZAttribute{
		{
			ClassName: "Deprecated",
			Args:      []*phpv.ZVal{phpv.ZString("the error level was removed").ZVal(), phpv.ZString("8.4").ZVal()},
		},
	})

	// Mark DATE_RFC7231 as deprecated (PHP 8.5)
	rfc7231DeprecationMsg := phpv.ZString("as this format ignores the associated timezone and always uses GMT").ZVal()
	rfc7231DeprecationSince := phpv.ZString("8.5").ZVal()
	g.ConstantSetAttributes("DATE_RFC7231", []*phpv.ZAttribute{
		{
			ClassName: "Deprecated",
			Args:      []*phpv.ZVal{rfc7231DeprecationMsg, rfc7231DeprecationSince},
		},
	})

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
			g.classOrigNames[c.GetName().ToLower()] = c.GetName()
		}
	}

	// get env from process
	g.environ = g.p.environ.Dup()

	g.setupIni()
	g.doGPC()
}

// ReinitSuperglobals re-initializes superglobals ($_GET, $_POST, $_SERVER, etc.)
// based on current INI settings. Call this after changing INI settings like
// variables_order or register_argc_argv that affect superglobal population.
func (g *Global) ReinitSuperglobals() {
	// Re-process disable_functions in case INI was changed after init
	if v := g.IniConfig.Get(phpv.ZString("disable_functions")); v != nil {
		for _, name := range strings.Split(string(v.GetString(g)), ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				// PHP 8.5: exit/die cannot be disabled
				if name == "exit" || name == "die" {
					g.WriteStartupWarning(fmt.Sprintf("Warning: Cannot disable function %s() in Unknown on line 0\n", name))
					continue
				}
				g.disabledFuncs[phpv.ZString(name)] = struct{}{}
			}
		}
	}
	g.doGPC()
}

func (g *Global) setupIni() {
	options := g.p.Options
	cfg := g.IniConfig

	cfg.LoadDefaults(g)

	// In web SAPI, register_argc_argv defaults to "0" (PHP 8.1+ deprecates it for web)
	if g.req != nil {
		cfg.SetGlobal(g, "register_argc_argv", phpv.ZString("0").ZVal())
	}

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
			if name == "" {
				continue
			}
			if name == "exit" || name == "die" {
				g.startupWarnings = append(g.startupWarnings, []byte(fmt.Sprintf("Warning: Cannot disable function %s() in Unknown on line 0\n", name))...)
				continue
			}
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
	e := phpv.NewZArray()
	f := phpv.NewZArray()

	// Check enable_post_data_reading setting
	enablePostDataReading := g.GetConfig("enable_post_data_reading", phpv.ZString("1").ZVal()).String() != "0"

	// Store the raw POST body for php://input before parsing
	if g.req != nil && g.req.Body != nil && g.req.Method == "POST" {
		g.storeRequestBody()
	}
	// Reset body reader from stored raw body (needed for reinit after INI changes)
	if g.req != nil && g.rawRequestBody != nil {
		g.req.Body = io.NopCloser(bytes.NewReader(g.rawRequestBody))
	}

	order := g.GetConfig("variables_order", phpv.ZString("EGPCS").ZVal()).String()

	for _, l := range order {
		switch l {
		case 'e', 'E':
			// 'E' populates $_ENV with environment variables
			e.MergeTable(g.environ)
		case 'c', 'C':
			if g.req != nil {
				parseCookiesToArray(g, g.req.Header.Get("Cookie"), c)
				for k, v := range c.Iterate(g) {
					r.OffsetSet(g, k, v)
				}
			}
		case 'p', 'P':
			if enablePostDataReading && g.req != nil && g.req.Method == "POST" {
				err := g.parsePost(p, f)
				if err != nil {
					log.Printf("failed to parse POST data: %s", err)
				}
				for k, v := range p.Iterate(g) {
					r.OffsetSet(g, k, v)
				}
			}
		case 'g', 'G':
			if g.req != nil {
				err := ParseQueryToArray(g, g.req.URL.RawQuery, get)
				if err != nil {
					log.Printf("failed to parse GET data: %s", err)
				}
				for k, v := range get.Iterate(g) {
					r.OffsetSet(g, k, v)
				}
			}
		case 's', 'S':
			// Merge environment variables into $_SERVER (PHP behavior)
			s.MergeTable(g.environ)

			s.OffsetSet(g, phpv.ZString("REQUEST_TIME").ZVal(), phpv.ZInt(g.start.Unix()).ZVal())
			s.OffsetSet(g, phpv.ZString("REQUEST_TIME_FLOAT").ZVal(), phpv.ZFloat(float64(g.start.UnixNano())/1e9).ZVal())
			s.OffsetSet(g, phpv.ZString("PHP_SELF"), phpv.ZStr(g.p.ScriptFilename))

			// Add request-related SERVER variables
			if g.req != nil {
				s.OffsetSet(g, phpv.ZString("REQUEST_METHOD").ZVal(), phpv.ZString(g.req.Method).ZVal())
				s.OffsetSet(g, phpv.ZString("QUERY_STRING").ZVal(), phpv.ZString(g.req.URL.RawQuery).ZVal())
				if g.req.Host != "" {
					s.OffsetSet(g, phpv.ZString("HTTP_HOST").ZVal(), phpv.ZString(g.req.Host).ZVal())
				}
				s.OffsetSet(g, phpv.ZString("REQUEST_URI").ZVal(), phpv.ZString(g.req.URL.RequestURI()).ZVal())
				s.OffsetSet(g, phpv.ZString("SCRIPT_NAME").ZVal(), phpv.ZString(g.req.URL.Path).ZVal())
			}

			// Handle register_argc_argv
			registerArgcArgv := g.GetConfig("register_argc_argv", phpv.ZString("1").ZVal()).String() != "0"

			if g.req != nil {
				// Web SAPI mode: when register_argc_argv=1, derive argv from query string
				if registerArgcArgv {
					// Emit deprecation warning - this is a PHP 8.1+ deprecation for web SAPI
					g.Deprecated("Deriving $_SERVER['argv'] from the query string is deprecated. Configure register_argc_argv=0 to turn this message off")
					args := phpv.NewZArray()
					if g.req.URL.RawQuery != "" {
						for _, part := range strings.Split(g.req.URL.RawQuery, "+") {
							args.OffsetSet(g, nil, phpv.ZString(part).ZVal())
						}
					}
					argv := args.ZVal()
					argc := args.Count(g).ZVal()
					s.OffsetSet(g, phpv.ZString("argv"), argv)
					s.OffsetSet(g, phpv.ZString("argc"), argc)
				}
				// When register_argc_argv=0, don't populate argv/argc in $_SERVER
			} else {
				// CLI mode: always populate from process argv
				args := phpv.NewZArray()
				for _, elem := range g.p.Argv {
					args.OffsetSet(g, nil, phpv.ZStr(elem))
				}
				argv := args.ZVal()
				argc := args.Count(g).ZVal()
				s.OffsetSet(g, phpv.ZString("argv"), argv)
				s.OffsetSet(g, phpv.ZString("argc"), argc)
				g.h.SetString("argv", argv)
				g.h.SetString("argc", argc)
			}
		}
	}
	g.h.SetString("_GET", get.ZVal())
	g.h.SetString("_POST", p.ZVal())
	g.h.SetString("_COOKIE", c.ZVal())
	g.h.SetString("_REQUEST", r.ZVal())
	g.h.SetString("_SERVER", s.ZVal())
	g.h.SetString("_ENV", e.ZVal())
	g.h.SetString("_FILES", f.ZVal())
}

func (g *Global) SetOutput(w io.Writer) {
	g.out = w
	g.buf = nil
}

// SetStdin replaces the default stdin with a custom reader (useful for testing).
func (g *Global) SetStdin(r io.Reader) {
	s := stream.NewStream(r)
	s.SetAttr("stream_type", "Go")
	s.SetAttr("mode", "r")
	s.ResourceType = phpv.ResourceStream
	g.customStdin = s
	g.constant["STDIN"] = s
}

// GetStdin returns the custom stdin stream if set, or nil.
func (g *Global) GetStdin() *stream.Stream {
	return g.customStdin
}

func (g *Global) RunFile(fn string) error {
	// Handle auto_prepend_file: include a file before the main script
	if prepend := g.GetConfig("auto_prepend_file", phpv.ZString("").ZVal()).String(); prepend != "" {
		_, err := g.Include(g, phpv.ZString(prepend))
		if err != nil {
			return err
		}
	}
	_, err := g.requireMain(phpv.ZString(fn))
	err = phpv.FilterExitError(err)

	// deferredErr holds fatal PHP errors that should be logged after
	// shutdown functions and destructors have run (matching PHP behavior
	// where fatal errors are displayed after destructor output).
	var deferredErr *phpv.PhpError

	// Convert break/continue outside loop to a fatal error
	if br, ok := phpv.UnwrapError(err).(*phperr.PhpBreak); ok {
		if br.Initial > 1 {
			err = &phpv.PhpError{
				Err:  fmt.Errorf("Cannot 'break' %d levels", br.Initial),
				Loc:  br.L,
				Code: phpv.E_ERROR,
			}
		} else {
			err = &phpv.PhpError{
				Err:  fmt.Errorf("'break' not in the 'loop' or 'switch' context"),
				Loc:  br.L,
				Code: phpv.E_ERROR,
			}
		}
	} else if cr, ok := phpv.UnwrapError(err).(*phperr.PhpContinue); ok {
		if cr.Initial > 1 {
			err = &phpv.PhpError{
				Err:  fmt.Errorf("Cannot 'continue' %d levels", cr.Initial),
				Loc:  cr.L,
				Code: phpv.E_ERROR,
			}
		} else {
			err = &phpv.PhpError{
				Err:  fmt.Errorf("'continue' not in the 'loop' or 'switch' context"),
				Loc:  cr.L,
				Code: phpv.E_ERROR,
			}
		}
	}

	switch innerErr := phpv.UnwrapError(err).(type) {
	case *phpv.PhpExit:
	case *phperr.PhpTimeout:
		g.WriteErr([]byte("\n"))
		if g.GetConfig("display_errors", phpv.ZFalse.ZVal()).AsBool(g) {
			g.WriteErr([]byte(innerErr.String()))
		}
	default:
		if err != nil {
			err = g.handleUncaughtException(err)
			if err != nil {
				if phpErr, ok := err.(*phpv.PhpError); ok {
					// Clean buffered output on fatal error
					g.CleanBuffers()
					// Defer fatal PHP errors until after shutdown/destructors
					deferredErr = phpErr
					err = nil
				} else if _, ok := err.(*phperr.PhpThrow); !ok {
					return err
				}
			}
		}
	}

	if len(g.shutdownFuncs) > 0 {
		g.ResetDeadline()
		noActiveFile := &phpv.Loc{Filename: "[no active file]", Line: 0}
		for _, fn := range g.shutdownFuncs {
			// Reset location before each shutdown function so that exceptions
			// thrown by shutdown functions report "[no active file]:0".
			g.l = noActiveFile
			_, serr := g.CallZValInternal(g, fn, nil)
			if serr != nil {
				if phpv.IsExit(serr) {
					break
				}
				if timeout, ok := phpv.UnwrapError(serr).(*phperr.PhpTimeout); ok {
					g.WriteErr([]byte("\n"))
					g.WriteErr([]byte(timeout.String()))
					break
				}
				serr = g.handleUncaughtException(serr)
				if serr != nil {
					// Non-exception, non-handled error during shutdown
					break
				}
			}
		}
	}

	// send headers even if there's no output
	if !g.header.Sent && g.lastOutChar == 0 {
		g.header.SendHeaders(g)
	}

	closeErr := g.Close()

	// Log the deferred fatal error after destructors have run
	if deferredErr != nil {
		g.LogError(deferredErr)
	}

	return closeErr
}

// HandleUncaughtException handles an uncaught exception by calling the user
// exception handler if one is registered. Returns nil if the exception was
// handled, or the (possibly new) error if it wasn't.
//
// PHP behavior:
//   - The exception handler is temporarily removed during invocation to prevent
//     re-entrancy.
//   - After the handler completes (successfully or not), the handler is restored.
//   - If the handler throws a new exception, check if the handler registered a
//     new handler during its execution; if so, use the new handler for the new
//     exception (recursively).
//   - If no handler is available for a thrown exception, return it as-is.
func (g *Global) HandleUncaughtException(err error) error {
	ex, ok := err.(*phperr.PhpThrow)
	if !ok {
		return err
	}

	for {
		handler := g.GetUserExceptionHandler()
		if handler == nil {
			// No handler available - return the exception as-is
			return ex
		}

		// PHP behavior: the exception handler stays on the stack during
		// invocation. The handler can manipulate the stack via
		// restore_exception_handler() or set_exception_handler().
		// After the handler returns:
		// - If the handler modified the stack, respect those changes
		// - If the handler didn't modify the stack, leave it as-is
		stackLenBefore := len(g.userExceptionHandlerStack)

		_, handlerErr := g.CallZValInternal(g, handler, []*phpv.ZVal{ex.Obj.ZVal()})
		stackChanged := len(g.userExceptionHandlerStack) != stackLenBefore

		if handlerErr == nil {
			// Handler succeeded. Nothing more to do.
			return nil
		}

		// Handler threw a new exception.
		newEx, isThrow := handlerErr.(*phperr.PhpThrow)
		if !isThrow {
			if inner, ok2 := phpv.UnwrapError(handlerErr).(*phperr.PhpThrow); ok2 {
				newEx = inner
				isThrow = true
			}
		}
		if !isThrow {
			return handlerErr
		}

		if stackChanged {
			// The handler modified the stack. Check if a new (different)
			// handler is available. If so, use it for the new exception.
			newHandler := g.GetUserExceptionHandler()
			if newHandler != nil && newHandler != handler {
				ex = newEx
				continue
			}
		}

		// Either the handler didn't change the stack (same handler is on top,
		// which would cause infinite recursion) or no new handler was
		// registered. Return the new exception as an unhandled fatal error.
		return newEx
	}
}

// handleUncaughtException is a convenience wrapper that formats unhandled
// exceptions as fatal errors to stderr (web server mode).
func (g *Global) handleUncaughtException(err error) error {
	result := g.HandleUncaughtException(err)
	if result == nil {
		return nil
	}
	if ex, ok := result.(*phperr.PhpThrow); ok {
		g.formatUncaughtFatal(ex)
		return nil
	}
	return result
}

// formatUncaughtFatal formats an uncaught exception as a PHP Fatal error to stderr.
func (g *Global) formatUncaughtFatal(ex *phperr.PhpThrow) {
	// Set location to "[no active file]:0" before calling ErrorTrace so that
	// any errors thrown inside __toString() during formatting get that location.
	savedLoc := g.l
	noActiveFile := &phpv.Loc{Filename: "[no active file]", Line: 0}
	g.l = noActiveFile
	trace, replacement := ex.ErrorTrace(g)
	g.l = savedLoc
	// If __toString() threw, use the replacement exception for file/line
	src := ex
	if replacement != nil {
		src = replacement
	}
	thrownFile := src.ThrownFile()
	if thrownFile == "" {
		thrownFile = "Unknown"
	}
	thrownLine := src.ThrownLine()
	g.WriteErr([]byte(fmt.Sprintf("\nFatal error: %s\n  thrown in %s on line %d\n", trace, thrownFile, thrownLine)))
	// Update LastError so that error_get_last() in shutdown functions returns this fatal error.
	// The message is "trace\n  thrown" (truncated at "thrown"), matching PHP's behavior.
	msg := trace + "\n  thrown"
	g.LastError = &phpv.PhpError{
		Err:  fmt.Errorf("%s", msg),
		Code: phpv.E_ERROR,
		Loc:  &phpv.Loc{Filename: thrownFile, Line: thrownLine},
	}
}

func (g *Global) Write(v []byte) (int, error) {
	if !g.header.Sent {
		err := g.header.SendHeaders(g)
		if err != nil {
			return 0, err
		}
	}
	// Flush any startup warnings that were buffered before output was set
	if len(g.startupWarnings) > 0 {
		sw := g.startupWarnings
		g.startupWarnings = nil
		g.out.Write(sw)
	}
	if len(v) > 0 {
		g.lastOutChar = v[len(v)-1]
	}
	return g.out.Write(v)
}

// ErrorPrefix returns "\n" if prior output exists and didn't end with a
// newline, matching PHP's behavior of separating inline output from error
// messages. Returns "" when the error is the first output.
func (g *Global) ErrorPrefix() string {
	if g.lastOutChar != 0 && g.lastOutChar != '\n' {
		return "\n"
	}
	return ""
}

func (g *Global) WriteErr(v []byte) (int, error) {
	return g.errOut.Write(v)
}

// WriteStartupWarning buffers a warning message emitted during request startup
// (before output is configured). These warnings are flushed on first Write.
func (g *Global) WriteStartupWarning(msg string) {
	g.startupWarnings = append(g.startupWarnings, []byte(msg)...)
}

func (g *Global) RestoreConfig(name phpv.ZString) {
	g.IniConfig.RestoreConfig(g, name)
}

func (g *Global) SetLocalConfig(name phpv.ZString, value *phpv.ZVal) (*phpv.ZVal, bool) {
	if !g.IniConfig.CanIniSet(name) {
		return nil, false
	}

	// max_memory_limit capping: when setting memory_limit, check against max_memory_limit
	if name == "memory_limit" {
		value = g.capMemoryLimit(value)
		// Update the actual MemMgr limit
		bytes := parseIniBytes(string(value.AsString(g)))
		if bytes <= 0 {
			g.mem.SetLimit(0) // unlimited
		} else {
			g.mem.SetLimit(bytes)
		}
	}

	// zend.exception_string_param_max_len: range 0..1000000
	if name == "zend.exception_string_param_max_len" {
		n := value.AsInt(g)
		if n < 0 || n > 1000000 {
			return nil, false
		}
	}

	// open_basedir: resolve paths and enforce narrowing (can only restrict further, not widen)
	if name == "open_basedir" {
		newValue := value.String()
		if !g.checkOpenBasedirNarrowing(newValue) {
			return nil, false
		}
		// Resolve relative paths to absolute at set time (prevents ".." bypass)
		value = phpv.ZString(g.resolveBasedirPaths(newValue)).ZVal()
	}

	old := g.IniConfig.SetLocal(g, name, value)
	return old, true
}

// resolveBasedirPaths resolves each entry in a colon-separated basedir list to an absolute path.
func (g *Global) resolveBasedirPaths(basedir string) string {
	dirs := strings.Split(basedir, string(filepath.ListSeparator))
	resolved := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(string(g.Getwd()), dir)
		}
		dir = filepath.Clean(dir)
		resolved = append(resolved, dir)
	}
	return strings.Join(resolved, string(filepath.ListSeparator))
}

// checkOpenBasedirNarrowing checks if newBasedir is a narrowing of the current basedir.
// Each entry in newBasedir must be within one of the current basedir entries.
// Returns true if the change is allowed.
func (g *Global) checkOpenBasedirNarrowing(newBasedir string) bool {
	currentBasedir := g.GetConfig("open_basedir", phpv.ZNULL.ZVal()).String()
	if currentBasedir == "" {
		return true // no current restriction, anything is allowed
	}

	// Resolve current basedir entries
	currentDirs := strings.Split(currentBasedir, string(filepath.ListSeparator))
	resolvedCurrent := make([]string, 0, len(currentDirs))
	for _, dir := range currentDirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(string(g.Getwd()), dir)
		}
		resolvedCurrent = append(resolvedCurrent, filepath.Clean(dir))
	}

	// Resolve new basedir entries
	newDirs := strings.Split(newBasedir, string(filepath.ListSeparator))
	for _, dir := range newDirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(string(g.Getwd()), dir)
		}
		dir = filepath.Clean(dir)

		// Check if this new entry is within any current entry
		withinCurrent := false
		for _, cur := range resolvedCurrent {
			curWithSep := cur
			if !strings.HasSuffix(curWithSep, string(filepath.Separator)) {
				curWithSep += string(filepath.Separator)
			}
			dirWithSep := dir
			if !strings.HasSuffix(dirWithSep, string(filepath.Separator)) {
				dirWithSep += string(filepath.Separator)
			}
			if strings.HasPrefix(dirWithSep, curWithSep) || dir == cur {
				withinCurrent = true
				break
			}
		}
		if !withinCurrent {
			return false
		}
	}
	return true
}

// parseIniBytes parses a PHP INI memory size string (e.g. "128M", "1G", "-1")
// into bytes. Returns -1 for unlimited.
func parseIniBytes(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-1" {
		return -1
	}
	var multiplier int64 = 1
	last := s[len(s)-1]
	switch last {
	case 'G', 'g':
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	case 'M', 'm':
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case 'K', 'k':
		multiplier = 1024
		s = s[:len(s)-1]
	}
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n * multiplier
}

// formatIniBytes formats bytes back to a human-readable INI string
func formatIniBytes(b int64) string {
	if b == -1 {
		return "-1"
	}
	if b%(1024*1024*1024) == 0 {
		return fmt.Sprintf("%dG", b/(1024*1024*1024))
	}
	if b%(1024*1024) == 0 {
		return fmt.Sprintf("%dM", b/(1024*1024))
	}
	if b%1024 == 0 {
		return fmt.Sprintf("%dK", b/1024)
	}
	return fmt.Sprintf("%d", b)
}

// ApplyMaxMemoryLimit should be called after INI parsing to enforce max_memory_limit
// on the initial memory_limit value and sync the MemMgr limit.
func (g *Global) ApplyMaxMemoryLimit() {
	maxVal := g.GetConfig("max_memory_limit", phpv.ZStr("-1"))
	maxBytes := parseIniBytes(string(maxVal.AsString(g)))
	if maxBytes != -1 {
		memVal := g.GetConfig("memory_limit", phpv.ZStr("128M"))
		memBytes := parseIniBytes(string(memVal.AsString(g)))
		if memBytes == -1 {
			// Silently cap unlimited to max
			g.IniConfig.SetGlobal(g, "memory_limit", phpv.ZStr(formatIniBytes(maxBytes)))
		} else if memBytes > maxBytes {
			// Warn when a specific limit exceeds the max
			g.LogError(&phpv.PhpError{
				Err:  fmt.Errorf("Failed to set memory_limit to %d bytes. Setting to max_memory_limit instead (currently: %d bytes)", memBytes, maxBytes),
				Code: phpv.E_WARNING,
				Loc:  &phpv.Loc{Filename: "Unknown", Line: 0},
			}, logopt.Data{NoFuncName: true})
			g.IniConfig.SetGlobal(g, "memory_limit", phpv.ZStr(formatIniBytes(maxBytes)))
		}
	}

	// Sync MemMgr limit from the (possibly capped) INI value
	memVal := g.GetConfig("memory_limit", phpv.ZStr("128M"))
	memBytes := parseIniBytes(string(memVal.AsString(g)))
	if memBytes <= 0 {
		g.mem.SetLimit(0)
	} else {
		g.mem.SetLimit(memBytes)
	}
}

// capMemoryLimit checks memory_limit against max_memory_limit and caps it if needed.
// Returns the (possibly capped) value.
func (g *Global) capMemoryLimit(value *phpv.ZVal) *phpv.ZVal {
	maxVal := g.GetConfig("max_memory_limit", phpv.ZStr("-1"))
	maxBytes := parseIniBytes(string(maxVal.AsString(g)))
	if maxBytes == -1 {
		return value // no cap
	}

	memBytes := parseIniBytes(string(value.AsString(g)))
	if memBytes == -1 {
		// Silently cap unlimited to max
		return phpv.ZStr(formatIniBytes(maxBytes))
	}
	if memBytes > maxBytes {
		// Warn when a specific limit exceeds the max
		g.Warn("Failed to set memory_limit to %d bytes. Setting to max_memory_limit instead (currently: %d bytes)", memBytes, maxBytes)
		return phpv.ZStr(formatIniBytes(maxBytes))
	}
	return value
}

func (g *Global) GetConfig(name phpv.ZString, def *phpv.ZVal) *phpv.ZVal {
	val := g.IniConfig.Get(name)
	if val != nil && val.Get() != nil {
		return val.Get()
	}
	return def
}

// GetConfigEntry returns the raw IniValue for a config entry, or nil if the
// config option doesn't exist. Unlike GetConfig, this allows callers to
// distinguish between "not set" and "set to empty".
func (g *Global) GetConfigEntry(name phpv.ZString) *phpv.IniValue {
	return g.IniConfig.Get(name)
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
	g.l = l
	g.tickCount++
	if g.tickCount&0xFF == 0 {
		deadline := g.timerStart.Add(g.deadlineDuration)
		if time.Until(deadline) <= 0 {
			seconds := math.Round(g.deadlineDuration.Seconds())
			return &phperr.PhpTimeout{L: g.l, Seconds: int(seconds)}
		}

		// Check memory usage using the shared snapshot (updated by background goroutine)
		if err := g.checkMemoryLimit(); err != nil {
			return err
		}
	}
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
	// If the error is already a catchable exception (PhpThrow), pass it through
	// directly. This preserves the exception type so try/catch can handle it.
	if _, ok := err.(*phperr.PhpThrow); ok {
		return err
	}
	wrappedErr := g.l.Error(g, err, t...)
	result := phperr.HandleUserError(g, wrappedErr)
	if result == phperr.ErrHandledByUser {
		return nil
	}
	return result
}

func (g *Global) Errorf(format string, a ...any) error {
	err := g.l.Errorf(g, phpv.E_ERROR, format, a...)
	result := phperr.HandleUserError(g, err)
	if result == phperr.ErrHandledByUser {
		return nil
	}
	return result
}

func (g *Global) FuncError(err error, t ...phpv.PhpErrorType) error {
	// If the error is already a catchable exception (PhpThrow), pass it through.
	if _, ok := err.(*phperr.PhpThrow); ok {
		return err
	}
	wrappedErr := g.l.Error(g, err, t...)
	wrappedErr.FuncName = g.GetFuncName()
	result := phperr.HandleUserError(g, wrappedErr)
	if result == phperr.ErrHandledByUser {
		return nil
	}
	return result
}
func (g *Global) FuncErrorf(format string, a ...any) error {
	err := g.l.Errorf(g, phpv.E_ERROR, format, a...)
	err.FuncName = g.GetFuncName()
	result := phperr.HandleUserError(g, err)
	if result == phperr.ErrHandledByUser {
		return nil
	}
	return result
}

func (g *Global) Warn(format string, a ...any) error {
	// Only default to E_WARNING if caller didn't provide an explicit ErrType
	hasErrType := false
	for _, arg := range a {
		if _, ok := arg.(logopt.ErrType); ok {
			hasErrType = true
			break
		}
		if d, ok := arg.(logopt.Data); ok && d.ErrType != 0 {
			hasErrType = true
			break
		}
	}
	if !hasErrType {
		a = append(a, logopt.ErrType(phpv.E_WARNING))
	}
	return logWarning(g, format, a...)
}

func (g *Global) Notice(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_NOTICE))
	return logWarning(g, format, a...)
}

func (g *Global) Deprecated(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_DEPRECATED))
	err := logWarning(g, format, a...)
	if err == nil {
		g.ShownDeprecated(format)
	}
	return err
}

func (g *Global) UserDeprecated(format string, a ...any) error {
	a = append(a, logopt.ErrType(phpv.E_USER_DEPRECATED))
	return logWarning(g, format, a...)
}

func (g *Global) WarnDeprecated() error {
	funcName := g.GetFuncName()
	if ok := g.ShownDeprecated(funcName); ok {
		err := logWarning(
			g,
			"The %s() function is deprecated. This message will be suppressed on further calls",
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
		case logopt.IsInternal:
			option.IsInternal = bool(t)
		case logopt.LocNewLine:
			option.LocNewLine = bool(t)
		case logopt.Data:
			option.ErrType = t.ErrType
			option.NoFuncName = t.NoFuncName
			option.NoLoc = t.NoLoc
			option.IsInternal = t.IsInternal
			option.LocNewLine = t.LocNewLine
			if t.Loc != nil {
				option.Loc = t.Loc
			}
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
	if l, ok := option.Loc.(*phpv.Loc); ok && l != nil {
		loc = l
	}
	// For internal calls (engine-invoked callbacks like exception handlers),
	// use the internal location (Unknown:0) for deprecation/warning messages.
	if fc, ok := ctx.Func().(*FuncContext); ok && fc != nil {
		if intLoc := fc.InternalLoc(); intLoc != nil {
			loc = intLoc
		}
	}
	if option.NoFuncName {
		funcName = ""
	}
	message := fmt.Sprintf(format, fmtArgs...)

	phpErr := &phpv.PhpError{
		Err:        errors.New(message),
		FuncName:   funcName,
		Code:       phpv.PhpErrorType(option.ErrType),
		Loc:        loc,
		IsInternal: option.IsInternal,
	}

	err := phperr.HandleUserError(ctx, phpErr)
	if err == phperr.ErrHandledByUser {
		// User error handler handled it - suppress default output
		return nil
	}
	if err != nil {
		return err
	}

	// Always track last error for error_get_last(), even if suppressed by @
	ctx.Global().(*Global).LastError = phpErr

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

	// Track last error for error_get_last()
	g.LastError = err

	htmlErrors := bool(g.GetConfig("html_errors", phpv.ZBool(false).ZVal()).AsBool(g))

	var output bytes.Buffer
	var errType string
	switch err.Code {
	case phpv.E_ERROR, phpv.E_USER_ERROR, phpv.E_COMPILE_ERROR:
		errType = "Fatal error"
	case phpv.E_PARSE:
		errType = "Parse error"
	case phpv.E_WARNING, phpv.E_USER_WARNING:
		errType = "Warning"
	case phpv.E_NOTICE, phpv.E_USER_NOTICE:
		errType = "Notice"
	case phpv.E_DEPRECATED, phpv.E_USER_DEPRECATED:
		errType = "Deprecated"
	default:
		errType = "Info"
	}

	if htmlErrors {
		output.WriteString(fmt.Sprintf("<b>%s</b>:  ", errType))
	} else {
		output.WriteString(errType)
		output.WriteString(": ")
	}
	if !option.NoFuncName && err.FuncName != "" {
		output.WriteString(fmt.Sprintf("%s(): ", err.FuncName))
	}
	msg := err.Err.Error()
	if htmlErrors {
		msg = htmlEscapeString(msg)
	}
	output.WriteString(msg)
	if !option.NoLoc && err.Loc != nil {
		// When LocNewLine is set (e.g. for INI parser errors that already contain their
		// own "in <file> on line <N>" info), put the PHP script location on a new line
		// starting with " in" rather than appending directly after the message.
		locSep := " "
		if option.LocNewLine {
			locSep = "\n "
		}
		if htmlErrors {
			output.WriteString(fmt.Sprintf("%sin <b>%s</b> on line <b>%d</b>", locSep, htmlEscapeString(err.Loc.Filename), err.Loc.Line))
		} else {
			output.WriteString(fmt.Sprintf("%sin %s on line %d", locSep, err.Loc.Filename, err.Loc.Line))
		}
	}

	// When fatal_error_backtraces=On, include stack trace for fatal errors
	if (err.Code == phpv.E_ERROR || err.Code == phpv.E_COMPILE_ERROR) && len(err.PhpStackTrace) > 0 {
		fatalBacktraces := g.GetConfig("fatal_error_backtraces", phpv.ZBool(false).ZVal())
		if fatalBacktraces != nil && bool(fatalBacktraces.AsBool(g)) {
			// Check zend.exception_ignore_args for fatal error backtraces
			ignoreArgs := false
			ignoreVal := g.GetConfig("zend.exception_ignore_args", phpv.ZBool(false).ZVal())
			if ignoreVal != nil && bool(ignoreVal.AsBool(g)) {
				ignoreArgs = true
			}
			if ignoreArgs {
				// Strip args from trace entries
				for _, entry := range err.PhpStackTrace {
					entry.Args = nil
				}
			}
			output.WriteString("\nStack trace:\n")
			output.WriteString(string(phpv.StackTrace(err.PhpStackTrace).FormatWithMaxLen(phpv.TraceArgMaxLen)))
		}
	}

	// Check display_errors setting before outputting
	displayErrors := g.GetConfig("display_errors", phpv.ZBool(true).ZVal())
	shouldDisplay := true
	if displayErrors != nil {
		dv := displayErrors.String()
		// PHP treats "0", "", "false", "off" as disabled; "stderr" means display to stderr
		if dv == "0" || dv == "" || dv == "Off" || dv == "off" || dv == "false" {
			shouldDisplay = false
		}
	}

	if shouldDisplay {
		if htmlErrors {
			g.Write([]byte("<br />\n"))
			g.Write(output.Bytes())
			g.Write([]byte("<br />\n"))
		} else {
			g.Write([]byte("\n"))
			g.Write(output.Bytes())
			g.Write([]byte("\n"))
		}
	}
}

func htmlEscapeString(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
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

func (g *Global) RunShutdownFunctions() {
	if len(g.shutdownFuncs) == 0 {
		return
	}
	g.ResetDeadline()
	noActiveFile := &phpv.Loc{Filename: "[no active file]", Line: 0}
	for _, fn := range g.shutdownFuncs {
		// Reset location before each shutdown function so that exceptions
		// thrown by shutdown functions report "[no active file]:0".
		g.l = noActiveFile
		_, err := g.CallZValInternal(g, fn, nil)
		if err != nil {
			if phpv.IsExit(err) {
				break
			}
			if timeout, ok := phpv.UnwrapError(err).(*phperr.PhpTimeout); ok {
				g.WriteErr([]byte("\n"))
				g.WriteErr([]byte(timeout.String()))
				break
			}
			// Handle uncaught exceptions from shutdown functions.
			// Write to main output (not stderr) so test infrastructure can capture it.
			result := g.HandleUncaughtException(err)
			if result == nil {
				continue
			}
			if ex, ok := result.(*phperr.PhpThrow); ok {
				trace, replacement := ex.ErrorTrace(g)
				src := ex
				if replacement != nil {
					src = replacement
				}
				thrownFile := src.ThrownFile()
				if thrownFile == "" {
					thrownFile = "Unknown"
				}
				g.Write([]byte(fmt.Sprintf("\nFatal error: %s\n  thrown in %s on line %d\n", trace, thrownFile, src.ThrownLine())))
				break
			}
		}
	}
	g.shutdownFuncs = nil
}

func (g *Global) GetFunction(ctx phpv.Context, name phpv.ZString) (phpv.Callable, error) {
	// Strip leading namespace separator (e.g. \array_map → array_map)
	if len(name) > 0 && name[0] == '\\' {
		name = name[1:]
	}
	if f, ok := g.globalUserFuncs[name.ToLower()]; ok {
		return f, nil
	}
	if f, ok := g.globalInternalFuncs[name.ToLower()]; ok {
		if _, ok := g.disabledFuncs[name]; ok {
			// PHP 8.0+: disabled functions are treated as undefined (throwable Error)
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined function %s()", name))
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

	// Namespace fallback: if Foo\bar is not found, try bar (global)
	if idx := strings.LastIndexByte(string(name), '\\'); idx >= 0 {
		globalName := name[idx+1:]
		if f, ok := g.globalUserFuncs[globalName.ToLower()]; ok {
			return f, nil
		}
		if f, ok := g.globalInternalFuncs[globalName.ToLower()]; ok {
			if _, ok := g.disabledFuncs[globalName]; ok {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined function %s()", name))
			}
			return f, nil
		}
		if f, ok := g.globalLazyFunc[globalName.ToLower()]; ok {
			_, err := f.r[f.p].Run(ctx)
			if err != nil {
				return nil, err
			}
			f.r[f.p] = phpv.RunNull{}
			if f, ok := g.globalUserFuncs[globalName.ToLower()]; ok {
				return f, nil
			}
		}
	}

	return nil, phpobj.ThrowError(g, phpobj.Error, fmt.Sprintf("Call to undefined function %s()", name))
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

// ConstantForceSet sets a constant value, overwriting any existing value.
// Used for __COMPILER_HALT_OFFSET__ which is per-file.
func (g *Global) ConstantForceSet(k phpv.ZString, v phpv.Val) {
	g.constant[k] = v
}

func (g *Global) ConstantSetAttributes(k phpv.ZString, attrs []*phpv.ZAttribute) {
	g.constantAttrs[k] = attrs
}

func (g *Global) ConstantGetAttributes(k phpv.ZString) []*phpv.ZAttribute {
	return g.constantAttrs[k]
}

// GetAllConstants returns all defined constants.
func (g *Global) GetAllConstants() map[phpv.ZString]phpv.Val {
	return g.constant
}

func (g *Global) GetClass(ctx phpv.Context, name phpv.ZString, autoload bool) (phpv.ZClass, error) {
	switch name.ToLower() {
	case "self":
		// When compilingClass is set (e.g., during attribute argument evaluation
		// or class constant resolution), it takes priority over the function context.
		if g.compilingClass != nil {
			return g.compilingClass, nil
		}
		// check for func
		fc := ctx.Func()
		if fc == nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "self" when no class scope is active`)
		}
		f, ok := fc.(*FuncContext)
		if !ok || f == nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "self" when no class scope is active`)
		}
		// Check FuncContext.class first (set by MethodCallable/static calls)
		if f.class != nil {
			return f.class, nil
		}
		cfunc, ok := f.c.(phpv.ZClosure)
		if !ok || cfunc.GetClass() == nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "self" when no class scope is active`)
		}
		return cfunc.GetClass(), nil
	case "parent":
		// check for func
		fc := ctx.Func()
		if fc == nil {
			// During class compilation, parent:: refers to the parent of the class being compiled.
			// Check if there's a compilingClass set on the global context.
			if g.compilingClass != nil {
				parentClass := g.compilingClass.GetParent()
				if parentClass == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "parent" when current class scope has no parent`)
				}
				return parentClass, nil
			}
			return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "parent" when no class scope is active`)
		}
		f, ok := fc.(*FuncContext)
		if !ok || f == nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "parent" when no class scope is active`)
		}
		var selfClass phpv.ZClass
		if f.class != nil {
			selfClass = f.class
		} else if cfunc, ok := f.c.(phpv.ZClosure); ok {
			selfClass = cfunc.GetClass()
		}
		if selfClass == nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "parent" when no class scope is active`)
		}
		if selfClass.GetParent() == nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "parent" when current class scope has no parent`)
		}
		return selfClass.GetParent(), nil
	case "static":
		// check for func
		fc := ctx.Func()
		if fc == nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "static" when no class scope is active`)
		}
		f, ok := fc.(*FuncContext)
		if !ok || f == nil || f.this == nil {
			// In static context, check calledClass first (for late static binding
			// in static closures), then fall back to the class
			if ok && f != nil {
				if f.calledClass != nil {
					return f.calledClass, nil
				}
				if f.class != nil {
					return f.class, nil
				}
			}
			return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "static" when no class scope is active`)
		}
		return f.this.GetClass(), nil
	}
	// Strip leading namespace separator (e.g. \Exception → Exception)
	if len(name) > 0 && name[0] == '\\' {
		name = name[1:]
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
	// Try autoload
	nameLower := name.ToLower()
	// PHP does not call autoloaders for empty class names or names containing NUL bytes.
	// Note: backslashes are normal namespace separators (e.g. "space1\C") and must NOT
	// prevent autoloading. Forward slashes are also allowed in autoloader calls.
	if name == "" || strings.ContainsRune(string(name), '\x00') {
		autoload = false
	}
	if autoload && len(g.autoloadFuncs) > 0 {
		if g.autoloadingClass == nil {
			g.autoloadingClass = make(map[phpv.ZString]bool)
		}
		if !g.autoloadingClass[nameLower] {
			g.autoloadingClass[nameLower] = true
			defer delete(g.autoloadingClass, nameLower)
			// Iterate by index: new autoloaders registered during the loop
			// (e.g., by the first autoloader calling spl_autoload_register)
			// should be tried in the same cycle.
			for i := 0; i < len(g.autoloadFuncs); i++ {
				loader := g.autoloadFuncs[i]
				// Check deadline before calling each autoloader
				if err := g.Tick(ctx, g.l); err != nil {
					return nil, err
				}
				_, err := ctx.CallZVal(ctx, loader, []*phpv.ZVal{name.ZVal()})
				if err != nil {
					return nil, err
				}
				// Check if the class was loaded
				if c, ok := g.globalClasses[nameLower]; ok {
					return c, nil
				}
			}
		}
	}
	return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Class \"%s\" not found", name))
}

// ErrClassRedeclare is returned when a class name is already registered.
// IsAlias indicates the existing registration was under a different name (i.e. via class_alias).
type ErrClassRedeclare struct {
	Kind         string        // "class", "interface", or "trait"
	ExistingName phpv.ZString  // original name of existing class
	RegisterName phpv.ZString  // name being registered
	PrevLoc      string        // " (previously declared in ...)" or ""
	IsAlias      bool          // existing name differs from registered name (class_alias case)
}

func (e *ErrClassRedeclare) Error() string {
	// Use the existing class's canonical name for display.
	// For alias conflicts, the caller (class_alias) constructs its own message.
	displayName := e.ExistingName
	if displayName == "" {
		displayName = e.RegisterName
	}
	return fmt.Sprintf("Cannot redeclare %s %s%s", e.Kind, displayName, e.PrevLoc)
}

func (e *ErrClassRedeclare) IsAliasConflict() bool {
	return e.IsAlias
}

func (e *ErrClassRedeclare) RedeclareKind() string {
	return e.Kind
}

func (e *ErrClassRedeclare) RedeclarePrevLoc() string {
	return e.PrevLoc
}

func (g *Global) RegisterClass(name phpv.ZString, c phpv.ZClass) error {
	lowerName := name.ToLower()
	if existing, ok := g.globalClasses[lowerName]; ok {
		prevLoc := ""
		if existing.L != nil {
			prevLoc = fmt.Sprintf(" (previously declared in %s:%d)", existing.L.Filename, existing.L.Line)
		}
		kind := "class"
		switch existing.Type {
		case phpv.ZClassTypeInterface:
			kind = "interface"
		case phpv.ZClassTypeTrait:
			kind = "trait"
		}
		// Detect if the existing registration is an alias (registered under a different name)
		isAlias := existing.Name.ToLower() != lowerName
		return &ErrClassRedeclare{
			Kind:         kind,
			ExistingName: existing.Name,
			RegisterName: name,
			PrevLoc:      prevLoc,
			IsAlias:      isAlias,
		}
	}
	g.globalClasses[lowerName] = c.(*phpobj.ZClass)
	g.classOrigNames[lowerName] = name
	g.classOrder = append(g.classOrder, lowerName)
	delete(g.globalLazyClass, lowerName)
	return nil
}

func (g *Global) UnregisterClass(name phpv.ZString) {
	delete(g.globalClasses, name.ToLower())
}

func (g *Global) SetCompilingClass(c phpv.ZClass) {
	g.compilingClass = c
}

func (g *Global) GetCompilingClass() phpv.ZClass {
	return g.compilingClass
}

func (g *Global) GetDeclaredClasses() []phpv.ZString {
	result := make([]phpv.ZString, 0, len(g.classOrder))
	for _, lowerName := range g.classOrder {
		// Skip classes that have been unregistered
		if _, exists := g.globalClasses[lowerName]; !exists {
			continue
		}
		if origName, ok := g.classOrigNames[lowerName]; ok {
			result = append(result, origName)
		} else {
			result = append(result, g.globalClasses[lowerName].GetName())
		}
	}
	return result
}

// storeRequestBody reads and stores the raw request body for php://input.
// Must be called before parsePost() since that consumes the body reader.
func (g *Global) storeRequestBody() {
	if g.rawRequestBody != nil {
		return // already stored
	}
	if g.req == nil || g.req.Body == nil {
		return
	}
	body, err := io.ReadAll(io.LimitReader(g.req.Body, 64*1024*1024)) // 64MB limit
	if err != nil {
		return
	}
	g.rawRequestBody = body
	// Replace the body with a new reader so parsePost can still consume it
	g.req.Body = io.NopCloser(bytes.NewReader(body))
}

// GetRequestBody returns the raw request body for php://input
func (g *Global) GetRequestBody() []byte {
	return g.rawRequestBody
}

// RegisterTickFunction adds a tick function to be called every N statements
func (g *Global) RegisterTickFunction(cb phpv.Callable, args []*phpv.ZVal) {
	g.tickFuncs = append(g.tickFuncs, tickFuncEntry{callable: cb, args: args})
}

// UnregisterTickFunction removes a tick function by callable identity
func (g *Global) UnregisterTickFunction(cb phpv.Callable) {
	for i, tf := range g.tickFuncs {
		if tf.callable == cb {
			g.tickFuncs = append(g.tickFuncs[:i], g.tickFuncs[i+1:]...)
			return
		}
	}
}

// CallTickFunctions invokes all registered tick functions
func (g *Global) CallTickFunctions(ctx phpv.Context) error {
	for _, tf := range g.tickFuncs {
		_, err := tf.callable.Call(ctx, tf.args)
		if err != nil {
			return err
		}
	}
	return nil
}

// HasTickFunctions returns true if any tick functions are registered
func (g *Global) HasTickFunctions() bool {
	return len(g.tickFuncs) > 0
}

// SetStrictTypes sets the strict_types flag for the current execution context.
func (g *Global) SetStrictTypes(v bool) {
	g.StrictTypes = v
}

// GetStrictTypes returns the current strict_types flag.
func (g *Global) GetStrictTypes() bool {
	return g.StrictTypes
}

func (g *Global) Close() error {
	// Flush any startup warnings that were never flushed (e.g., if exit() was
	// called before any output was produced).
	if len(g.startupWarnings) > 0 {
		sw := g.startupWarnings
		g.startupWarnings = nil
		g.out.Write(sw)
	}

	// Call destructors for any remaining objects before closing
	g.CallDestructors()

	// Clean up temporary files (uploaded files, etc.)
	for _, f := range g.tempFiles {
		os.Remove(f)
	}
	g.tempFiles = nil

	for {
		if g.buf == nil {
			return nil
		}
		err := g.buf.Close()
		if err != nil {
			// Try to handle exceptions from OB callbacks via the
			// exception handler (GH-10695). If the handler catches
			// the exception, continue closing remaining buffers.
			handled := g.handleUncaughtException(err)
			if handled != nil {
				return handled
			}
		}
	}
}

// DiscardBuffers discards all output buffer content without flushing.
// Used on fatal errors to prevent buffered output from leaking.
func (g *Global) DiscardBuffers() {
	for g.buf != nil {
		g.buf.CloseClean()
	}
}

// CleanBuffers discards buffered content in all active output buffers
// without closing them. The buffers remain active so subsequent writes
// (like fatal error messages) still pass through their callbacks.
func (g *Global) CleanBuffers() {
	for b := g.buf; b != nil; b = b.Parent() {
		b.b = nil
	}
}

// RegisterTempFile registers a temporary file path for cleanup when the
// request ends. Used for uploaded file temp files.
func (g *Global) RegisterTempFile(path string) {
	g.tempFiles = append(g.tempFiles, path)
}

// RegisterUploadedFile registers a file path as an uploaded file for
// is_uploaded_file() and move_uploaded_file() checks.
func (g *Global) RegisterUploadedFile(path string) {
	if g.uploadedFiles == nil {
		g.uploadedFiles = make(map[string]struct{})
	}
	g.uploadedFiles[path] = struct{}{}
}

// IsUploadedFile checks if the given path was registered as an uploaded file.
func (g *Global) IsUploadedFile(path string) bool {
	if g.uploadedFiles == nil {
		return false
	}
	_, ok := g.uploadedFiles[path]
	return ok
}

// UnregisterUploadedFile removes a file from the uploaded files set (after move).
func (g *Global) UnregisterUploadedFile(path string) {
	if g.uploadedFiles != nil {
		delete(g.uploadedFiles, path)
	}
}

// IsObDisabled returns whether the OB system has been disabled.
// IsFunctionDisabled returns true if the named function is disabled via disable_functions INI.
func (g *Global) IsFunctionDisabled(name phpv.ZString) bool {
	_, ok := g.disabledFuncs[name]
	return ok
}

func (g *Global) IsObDisabled() bool {
	return g.obDisabled
}

// SetObDisabled marks the OB system as disabled (after re-entrant fatal error).
func (g *Global) SetObDisabled() {
	g.obDisabled = true
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
	return g.mem.Alloc(int64(s))
}

// MemLimit returns the current memory limit in bytes (0 = unlimited).
func (g *Global) MemLimit() uint64 {
	l := g.mem.Limit()
	if l < 0 {
		return 0
	}
	return uint64(l)
}

// MemUsage returns the current tracked PHP memory usage in bytes.
func (g *Global) MemUsage() int64 { return g.mem.Usage() }

// MemPeakUsage returns the peak tracked PHP memory usage in bytes.
func (g *Global) MemPeakUsage() int64 { return g.mem.PeakUsage() }

// MemResetPeak sets peak = current tracked usage.
func (g *Global) MemResetPeak() { g.mem.ResetPeak() }

// MemMgr returns the memory manager, which implements phpv.MemTracker.
func (g *Global) MemMgrTracker() phpv.MemTracker { return g.mem }

// MarkJsonEncoding marks an object as currently being json-encoded.
// Returns true if the object was already being encoded (recursion detected).
func (g *Global) MarkJsonEncoding(obj phpv.ZObject) bool {
	if g.jsonEncodingObjects == nil {
		g.jsonEncodingObjects = make(map[phpv.ZObject]bool)
	}
	if g.jsonEncodingObjects[obj] {
		return true // recursion
	}
	g.jsonEncodingObjects[obj] = true
	return false
}

// UnmarkJsonEncoding removes the json-encoding mark from an object.
func (g *Global) UnmarkJsonEncoding(obj phpv.ZObject) {
	if g.jsonEncodingObjects != nil {
		delete(g.jsonEncodingObjects, obj)
	}
}

// GetSerializeSeenObjects returns the current serialize object tracking map (may be nil).
func (g *Global) GetSerializeSeenObjects() map[phpv.ZObject]bool {
	return g.SerializeSeenObjects
}

// SetSerializeSeenObjects sets the serialize object tracking map.
func (g *Global) SetSerializeSeenObjects(m map[phpv.ZObject]bool) {
	g.SerializeSeenObjects = m
}

// GetIncludedFiles returns a list of all included/required file paths.
func (g *Global) GetIncludedFiles() []string {
	result := make([]string, 0, len(g.included))
	for f := range g.included {
		result = append(result, string(f))
	}
	return result
}

// OpenFile opens a file for reading through the global file access layer.
// This centralizes file access so it can later be scoped to an fs.FS.
func (g *Global) OpenFile(ctx phpv.Context, path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (g *Global) GetLoadedExtensions() []string {
	return slices.Collect(maps.Keys(globalExtMap))
}

func (g *Global) Random() *random.State {
	return g.rand
}

func (g *Global) GetUserErrorHandler() (phpv.Callable, phpv.PhpErrorType, *phpv.ZVal) {
	if len(g.userErrorHandlerStack) == 0 {
		return nil, 0, nil
	}
	top := g.userErrorHandlerStack[len(g.userErrorHandlerStack)-1]
	return top.handler, top.filter, top.originalVal
}

func (g *Global) SetUserErrorHandler(handler phpv.Callable, filter phpv.PhpErrorType, originalVal *phpv.ZVal) {
	// Push onto the stack (even null entries, to match PHP behavior)
	g.userErrorHandlerStack = append(g.userErrorHandlerStack, errorHandlerEntry{handler: handler, filter: filter, originalVal: originalVal})
}

func (g *Global) RestoreUserErrorHandler() {
	if len(g.userErrorHandlerStack) > 0 {
		g.userErrorHandlerStack = g.userErrorHandlerStack[:len(g.userErrorHandlerStack)-1]
	}
}

func (g *Global) GetUserExceptionHandler() phpv.Callable {
	if len(g.userExceptionHandlerStack) == 0 {
		return nil
	}
	return g.userExceptionHandlerStack[len(g.userExceptionHandlerStack)-1].handler
}

// SetUserExceptionHandler sets the exception handler and returns the original
// ZVal of the previous handler (so that set_exception_handler can return it).
func (g *Global) SetUserExceptionHandler(handler phpv.Callable, originalVal *phpv.ZVal) *phpv.ZVal {
	var prev *phpv.ZVal
	if len(g.userExceptionHandlerStack) > 0 {
		prev = g.userExceptionHandlerStack[len(g.userExceptionHandlerStack)-1].originalVal
	}
	g.userExceptionHandlerStack = append(g.userExceptionHandlerStack, exceptionHandlerEntry{
		handler:     handler,
		originalVal: originalVal,
	})
	return prev
}

func (g *Global) RestoreUserExceptionHandler() {
	if len(g.userExceptionHandlerStack) > 0 {
		g.userExceptionHandlerStack = g.userExceptionHandlerStack[:len(g.userExceptionHandlerStack)-1]
	}
}

func (g *Global) RegisterAutoload(handler phpv.Callable, prepend bool) {
	// Check for duplicate - PHP considers two autoloaders identical only if they
	// are the same function name, or the same object instance + method name.
	// Two different instances of the same class with the same method are NOT duplicates.
	for _, f := range g.autoloadFuncs {
		if autoloadCallablesEqual(f, handler) {
			return // already registered, skip
		}
	}
	if prepend {
		g.autoloadFuncs = append([]phpv.Callable{handler}, g.autoloadFuncs...)
	} else {
		g.autoloadFuncs = append(g.autoloadFuncs, handler)
	}
}

// autoloadCallablesEqual returns true if two callables represent the same autoloader.
// For plain functions: same name. For object methods: same object instance AND same method.
// For static methods: same class AND same method.
func autoloadCallablesEqual(a, b phpv.Callable) bool {
	// Unwrap to get the inner callable and object
	aObj, aName := autoloadCallableIdentity(a)
	bObj, bName := autoloadCallableIdentity(b)

	if aName != bName {
		return false
	}
	// If both have object identity, compare by object pointer
	if aObj != nil && bObj != nil {
		return aObj == bObj
	}
	// If neither has object identity, same name means same callable
	if aObj == nil && bObj == nil {
		return true
	}
	return false
}

// autoloadCallableIdentity returns the bound object (if any) and a canonical name for matching.
func autoloadCallableIdentity(c phpv.Callable) (phpv.ZObject, string) {
	switch v := c.(type) {
	case *phpv.BoundedCallable:
		if v.This != nil {
			return v.This, string(v.This.GetClass().GetName()) + "::" + v.Callable.Name()
		}
		return autoloadCallableIdentity(v.Callable)
	case *phpv.MethodCallable:
		// Static method - no object instance
		return nil, string(v.Class.GetName()) + "::" + v.Callable.Name()
	default:
		return nil, phpv.CallableDisplayName(c)
	}
}

func (g *Global) UnregisterAutoload(handler phpv.Callable) bool {
	for i, f := range g.autoloadFuncs {
		if autoloadCallablesEqual(f, handler) {
			g.autoloadFuncs = append(g.autoloadFuncs[:i], g.autoloadFuncs[i+1:]...)
			return true
		}
	}
	// Fall back to name-based matching
	name := phpv.CallableDisplayName(handler)
	return g.UnregisterAutoloadByName(name)
}

func (g *Global) UnregisterAutoloadByName(name string) bool {
	// Strip leading backslash for comparison (PHP normalizes class names)
	normName := name
	if len(normName) > 0 && normName[0] == '\\' {
		normName = normName[1:]
	}
	for i, f := range g.autoloadFuncs {
		displayName := phpv.CallableDisplayName(f)
		normDisplay := displayName
		if len(normDisplay) > 0 && normDisplay[0] == '\\' {
			normDisplay = normDisplay[1:]
		}
		if normDisplay == normName || displayName == name {
			g.autoloadFuncs = append(g.autoloadFuncs[:i], g.autoloadFuncs[i+1:]...)
			return true
		}
	}
	return false
}

func (g *Global) ClearAutoloadFunctions() {
	g.autoloadFuncs = nil
}

func (g *Global) GetAutoloadFunctions() []phpv.Callable {
	return g.autoloadFuncs
}

func (g *Global) GetAutoloadExtensions() string {
	if g.autoloadExts == "" {
		return ".inc,.php"
	}
	return g.autoloadExts
}

func (g *Global) SetAutoloadExtensions(exts string) {
	g.autoloadExts = exts
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
			bareName := fc.c.Name()
			if bareName == "" && fc.class != nil && fc.methodType != "" {
				bareName = "__construct"
			}
			trace = append(trace, &phpv.StackTraceEntry{
				FuncName:     fc.GetFuncNameForTrace(),
				BareFuncName: bareName,
				Filename:     fc.loc.Filename,
				ClassName:    className,
				MethodType:   fc.methodType,
				Line:         fc.loc.Line,
				Args:         fc.Args,
				Object:       fc.this,
				IsInternal:   fc.isInternal,
			})
		}
		context = context.Parent(1)
	}
	return trace
}

func (g *Global) LastCallable() phpv.Callable { return g.lastCallable }
func (g *Global) ClearLastCallable() { g.lastCallable = nil }

func (g *Global) ShownDeprecated(key string) bool {
	_, exists := g.shownDeprecated[key]
	g.shownDeprecated[key] = struct{}{}
	return !exists
}

func (g *Global) SetNoDiscardPending(v bool) { g.noDiscardPending = v }
func (g *Global) ConsumeNoDiscardPending() bool {
	was := g.noDiscardPending
	g.noDiscardPending = false
	return was
}

func (g *Global) HeaderContext() *phpv.HeaderContext {
	return g.header
}

func (g *Global) NextResourceID() int {
	id := g.nextResourceID
	g.nextResourceID++
	return id
}

func (g *Global) NextObjectID() int {
	if n := len(g.freeObjectIDs); n > 0 {
		id := g.freeObjectIDs[n-1]
		g.freeObjectIDs = g.freeObjectIDs[:n-1]
		return id
	}
	g.nextObjectID++
	return g.nextObjectID
}

func (g *Global) ReleaseObjectID(id int) {
	if id > 0 {
		g.freeObjectIDs = append(g.freeObjectIDs, id)
	}
}

func (g *Global) SetNextCallSuppressCalledIn(v bool) {
	g.nextCallSuppressCalledIn = v
}

func (g *Global) RegisterDestructor(obj phpv.ZObject) {
	g.destructObjects = append(g.destructObjects, obj)
}

func (g *Global) UnregisterDestructor(obj phpv.ZObject) {
	for i, o := range g.destructObjects {
		if o == obj {
			g.destructObjects = append(g.destructObjects[:i], g.destructObjects[i+1:]...)
			return
		}
	}
}

// CallDestructors calls __destruct on all remaining tracked objects.
// Exceptions thrown by destructors are passed to handleUncaughtException.
func (g *Global) CallDestructors() {
	// Take the current list and clear it to prevent infinite loop
	// if destructors create new objects
	objs := g.destructObjects
	g.destructObjects = nil
	// Process in LIFO order
	for i := len(objs) - 1; i >= 0; i-- {
		obj := objs[i]
		zobj, isZObj := obj.(*phpobj.ZObject)
		if isZObj && zobj.IsDestructed() {
			continue // Already destructed (e.g. during variable reassignment)
		}
		// Skip destructors for uninitialized lazy objects
		if isZObj && zobj.IsLazy() {
			continue
		}
		if m, ok := obj.GetClass().GetMethod("__destruct"); ok {
			if isZObj {
				zobj.SetDestructed(true)
			}
			// Check visibility during shutdown — private/protected from global scope
			// should emit a warning and skip. PHP behavior varies by version.
			if m.Modifiers.IsPrivate() || m.Modifiers.IsProtected() {
				vis := "private"
				if m.Modifiers.IsProtected() {
					vis = "protected"
				}
				g.LogError(&phpv.PhpError{
					Err:  fmt.Errorf("Call to %s %s::__destruct() from global scope during shutdown ignored", vis, obj.GetClass().GetName()),
					Code: phpv.E_WARNING,
					Loc:  &phpv.Loc{Filename: "Unknown", Line: 0},
				}, logopt.Data{NoFuncName: true})
				continue
			}
			_, derr := g.CallZVal(g, m.Method, nil, obj)
			if derr != nil {
				// Try to handle via the exception handler (GH-10695)
				g.handleUncaughtException(derr)
			}
		}
	}
}
