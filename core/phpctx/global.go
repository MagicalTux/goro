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

	IniConfig phpv.IniConfig

	// this is the actual environment (defined functions, classes, etc)
	globalInternalFuncs map[phpv.ZString]phpv.Callable
	globalUserFuncs     map[phpv.ZString]phpv.Callable
	disabledFuncs       map[phpv.ZString]struct{}

	globalClasses map[phpv.ZString]*phpobj.ZClass // TODO replace *ZClass with a nice interface
	shutdownFuncs []phpv.Callable
	callDepth     int
	constant      map[phpv.ZString]phpv.Val
	environ       *phpv.ZHashTable
	included      map[phpv.ZString]bool // included files (used for require_once, etc)
	includePath   []string              // TODO: initialize

	streamHandlers map[string]stream.Handler
	fileHandler    *stream.FileHandler

	globalLazyFunc  map[phpv.ZString]*globalLazyOffset
	globalLazyClass map[phpv.ZString]*globalLazyOffset

	errOut        io.Writer
	out           io.Writer
	buf           *Buffer
	lastOutChar   byte
	ImplicitFlush bool

	rand *random.State

	shownDeprecated map[string]struct{}

	userErrorHandler     phpv.Callable
	userErrorFilter      phpv.PhpErrorType
	userExceptionHandler phpv.Callable

	autoloadFuncs    []phpv.Callable
	autoloadingClass map[phpv.ZString]bool // prevent infinite recursion in autoload

	header *phpv.HeaderContext

	nextResourceID int
	nextObjectID   int

	DefaultStreamContext *stream.Context

	destructObjects []phpv.ZObject // objects with __destruct to call at shutdown

	compilingClass phpv.ZClass // class currently being compiled (for self:: resolution)

	rawRequestBody []byte // stored POST body for php://input

	customStdin *stream.Stream // custom stdin for testing

	startupWarnings []byte // warnings from request startup (before output is set)

	tempFiles     []string            // temporary files to clean up (e.g., uploaded files)
	uploadedFiles map[string]struct{} // set of uploaded file paths for is_uploaded_file()
	obDisabled    bool                // OB system disabled after re-entrant fatal error
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

		header: &phpv.HeaderContext{Headers: http.Header{}},

		// the first 3 are reserved for STDIN, STDOUT and STDERR
		nextResourceID: 4,
	}
	g.SetDeadline(g.start.Add(30 * time.Second))

	g.fileHandler, _ = stream.NewFileHandler("/")
	g.streamHandlers["file"] = g.fileHandler
	g.streamHandlers["php"] = stream.PhpHandler()
	g.streamHandlers["http"] = stream.NewHttpHandler()

	g.initLocale()

	return g
}

func (g *Global) initLocale() {
	locale.SetLocale(locale.LC_ALL, "")
}

func (g *Global) RegisterStreamHandler(scheme string, handler stream.Handler) {
	g.streamHandlers[scheme] = handler
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

// ReinitSuperglobals re-initializes superglobals ($_GET, $_POST, $_SERVER, etc.)
// based on current INI settings. Call this after changing INI settings like
// variables_order or register_argc_argv that affect superglobal population.
func (g *Global) ReinitSuperglobals() {
	// Re-process disable_functions in case INI was changed after init
	if v := g.IniConfig.Get(phpv.ZString("disable_functions")); v != nil {
		for _, name := range strings.Split(string(v.GetString(g)), ",") {
			name = strings.TrimSpace(name)
			if name != "" {
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
			g.disabledFuncs[phpv.ZString(name)] = struct{}{}
		}
	}
}

func (g *Global) doGPC() {
	// Clear any startup warnings from a previous doGPC() call (e.g., during reinit)
	g.startupWarnings = nil

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

	switch innerErr := phpv.UnwrapError(err).(type) {
	case *phpv.PhpExit:
	case *phperr.PhpTimeout:
		g.WriteErr([]byte("\n"))
		if g.GetConfig("display_errors", phpv.ZFalse.ZVal()).AsBool(g) {
			g.WriteErr([]byte(innerErr.String()))
		}
	default:
		if err != nil {
			// Handle uncaught exceptions via user exception handler
			if ex, ok := err.(*phperr.PhpThrow); ok && g.userExceptionHandler != nil {
				handler := g.userExceptionHandler
				g.userExceptionHandler = nil // prevent re-entrancy
				_, handlerErr := g.CallZVal(g, handler, []*phpv.ZVal{ex.Obj.ZVal()})
				if handlerErr != nil {
					return handlerErr
				}
				err = nil
			}
			// Format uncaught exceptions as PHP Fatal error
			if err != nil {
				if ex, ok := err.(*phperr.PhpThrow); ok {
					trace := ex.ErrorTrace(g)
					loc := ex.Loc
					if loc == nil {
						loc = &phpv.Loc{}
					}
					g.WriteErr([]byte(fmt.Sprintf("\nFatal error: %s\n  thrown in %s on line %d\n", trace, loc.Filename, loc.Line)))
					err = nil
				} else if phpErr, ok := err.(*phpv.PhpError); ok {
					// Clean buffered output on fatal error
					g.CleanBuffers()
					// Defer fatal PHP errors until after shutdown/destructors
					deferredErr = phpErr
					err = nil
				} else {
					return err
				}
			}
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
					g.WriteErr([]byte("\n"))
					g.WriteErr([]byte(timeout.String()))
					break
				}
				return err
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
			g.mem.SetLimit(uint64(bytes))
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
		g.mem.SetLimit(uint64(memBytes))
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
	a = append(a, logopt.ErrType(phpv.E_WARNING))
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
		case logopt.Data:
			option.ErrType = t.ErrType
			option.NoFuncName = t.NoFuncName
			option.NoLoc = t.NoLoc
			option.IsInternal = t.IsInternal
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
		if htmlErrors {
			output.WriteString(fmt.Sprintf(" in <b>%s</b> on line <b>%d</b>", htmlEscapeString(err.Loc.Filename), err.Loc.Line))
		} else {
			output.WriteString(fmt.Sprintf(" in %s on line %d", err.Loc.Filename, err.Loc.Line))
		}
	}

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
	for _, fn := range g.shutdownFuncs {
		_, err := g.CallZVal(g, fn, nil, nil)
		if err != nil {
			if phpv.IsExit(err) {
				break
			}
			if timeout, ok := phpv.UnwrapError(err).(*phperr.PhpTimeout); ok {
				g.WriteErr([]byte("\n"))
				g.WriteErr([]byte(timeout.String()))
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

	// Namespace fallback: if Foo\bar is not found, try bar (global)
	if idx := strings.LastIndexByte(string(name), '\\'); idx >= 0 {
		globalName := name[idx+1:]
		if f, ok := g.globalUserFuncs[globalName.ToLower()]; ok {
			return f, nil
		}
		if f, ok := g.globalInternalFuncs[globalName.ToLower()]; ok {
			if _, ok := g.disabledFuncs[globalName]; ok {
				ctx.Warn("%s() has been disabled for security reasons",
					globalName, logopt.NoFuncName(true))
				return noOp, nil
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

	return nil, g.Errorf("Call to undefined function %s()", name)
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
		fc := ctx.Func()
		if fc == nil {
			// During class compilation, self:: refers to the class being compiled.
			// Check if there's a compilingClass set on the global context.
			if g.compilingClass != nil {
				return g.compilingClass, nil
			}
			return nil, ctx.Errorf("Cannot access self:: when no method scope is active")
		}
		f, ok := fc.(*FuncContext)
		if !ok || f == nil {
			return nil, ctx.Errorf("Cannot access self:: when no method scope is active")
		}
		// Check FuncContext.class first (set by MethodCallable/static calls)
		if f.class != nil {
			return f.class, nil
		}
		cfunc, ok := f.c.(phpv.ZClosure)
		if !ok || cfunc.GetClass() == nil {
			return nil, ctx.Errorf("Cannot access self:: when no class scope is active")
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
					return nil, ctx.Errorf("Cannot access parent:: when current class scope has no parent")
				}
				return parentClass, nil
			}
			return nil, ctx.Errorf("Cannot access parent:: when no method scope is active")
		}
		f, ok := fc.(*FuncContext)
		if !ok || f == nil {
			return nil, ctx.Errorf("Cannot access parent:: when no method scope is active")
		}
		var selfClass phpv.ZClass
		if f.class != nil {
			selfClass = f.class
		} else if cfunc, ok := f.c.(phpv.ZClosure); ok {
			selfClass = cfunc.GetClass()
		}
		if selfClass == nil {
			return nil, ctx.Errorf("Cannot access parent:: when no class scope is active")
		}
		if selfClass.GetParent() == nil {
			return nil, ctx.Errorf("Cannot access parent:: when current class scope has no parent")
		}
		return selfClass.GetParent(), nil
	case "static":
		// check for func
		fc := ctx.Func()
		if fc == nil {
			return nil, ctx.Errorf("Cannot access static:: when no class scope is active")
		}
		f, ok := fc.(*FuncContext)
		if !ok || f == nil || f.this == nil {
			// In static context, fall back to the class
			if ok && f != nil && f.class != nil {
				return f.class, nil
			}
			return nil, ctx.Errorf("Cannot access static:: when no class scope is active")
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
	// PHP does not call autoloaders for class names containing path separators or NUL bytes
	if strings.ContainsAny(string(name), "/\\\x00") {
		autoload = false
	}
	if autoload && len(g.autoloadFuncs) > 0 {
		if g.autoloadingClass == nil {
			g.autoloadingClass = make(map[phpv.ZString]bool)
		}
		if !g.autoloadingClass[nameLower] {
			g.autoloadingClass[nameLower] = true
			defer delete(g.autoloadingClass, nameLower)
			for _, loader := range g.autoloadFuncs {
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
		// Use the existing class's original name for proper casing
		displayName := existing.Name
		if displayName == "" {
			displayName = name
		}
		return fmt.Errorf("Cannot redeclare %s %s%s", kind, displayName, prevLoc)
	}
	g.globalClasses[lowerName] = c.(*phpobj.ZClass)
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
	result := make([]phpv.ZString, 0, len(g.globalClasses))
	for _, c := range g.globalClasses {
		result = append(result, c.GetName())
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

func (g *Global) Close() error {
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
			return err
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
	return g.mem.Alloc(ctx, s)
}

// MemLimit returns the current memory limit in bytes (0 = unlimited).
func (g *Global) MemLimit() uint64 {
	return g.mem.Limit()
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

func (g *Global) GetUserErrorHandler() (phpv.Callable, phpv.PhpErrorType) {
	return g.userErrorHandler, g.userErrorFilter
}

func (g *Global) SetUserErrorHandler(handler phpv.Callable, filter phpv.PhpErrorType) {
	g.userErrorHandler = handler
	g.userErrorFilter = filter
}

func (g *Global) GetUserExceptionHandler() phpv.Callable {
	return g.userExceptionHandler
}

func (g *Global) SetUserExceptionHandler(handler phpv.Callable) phpv.Callable {
	prev := g.userExceptionHandler
	g.userExceptionHandler = handler
	return prev
}

func (g *Global) RegisterAutoload(handler phpv.Callable) {
	g.autoloadFuncs = append(g.autoloadFuncs, handler)
}

func (g *Global) UnregisterAutoload(handler phpv.Callable) bool {
	for i, f := range g.autoloadFuncs {
		if f == handler {
			g.autoloadFuncs = append(g.autoloadFuncs[:i], g.autoloadFuncs[i+1:]...)
			return true
		}
	}
	return false
}

func (g *Global) GetAutoloadFunctions() []phpv.Callable {
	return g.autoloadFuncs
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
				FuncName:     fc.GetFuncNameForTrace(),
				BareFuncName: fc.c.Name(),
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

func (g *Global) ShownDeprecated(key string) bool {
	_, exists := g.shownDeprecated[key]
	g.shownDeprecated[key] = struct{}{}
	return !exists
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
	g.nextObjectID++
	return g.nextObjectID
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

// CallDestructors calls __destruct on all remaining tracked objects
func (g *Global) CallDestructors() {
	// Take the current list and clear it to prevent infinite loop
	// if destructors create new objects
	objs := g.destructObjects
	g.destructObjects = nil
	// Process in LIFO order
	for i := len(objs) - 1; i >= 0; i-- {
		obj := objs[i]
		zobj, isZObj := obj.(*phpobj.ZObject)
		if isZObj && zobj.Destructed {
			continue // Already destructed (e.g. during variable reassignment)
		}
		if m, ok := obj.GetClass().GetMethod("__destruct"); ok {
			if isZObj {
				zobj.Destructed = true
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
			g.CallZVal(g, m.Method, nil, obj)
		}
	}
}
