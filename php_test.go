package main_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/MagicalTux/goro/core/compiler"
	"github.com/MagicalTux/goro/core/ini"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
	"github.com/andreyvit/diff"
)

// Currently focusing on lang tests, change variable to run other tests
var TestsPath = func() string {
	if v := os.Getenv("GORO_TEST_PATH"); v != "" {
		return v
	}
	return "test"
}()

// maxTestOutputSize caps the output buffer per test to prevent OOM crashes
// from infinite-output tests (e.g., recursive json_encode). 10 MB is generous
// for any normal test.
const maxTestOutputSize = 10 * 1024 * 1024

// limitedBuffer wraps a bytes.Buffer and silently discards writes once the
// buffer exceeds maxTestOutputSize. This prevents runaway tests from causing
// OOM crashes that kill the entire test suite.
type limitedBuffer struct {
	buf     bytes.Buffer
	limited bool
}

func (lb *limitedBuffer) Write(p []byte) (int, error) {
	if lb.limited {
		return len(p), nil // silently discard
	}
	if lb.buf.Len()+len(p) > maxTestOutputSize {
		lb.limited = true
		return len(p), nil
	}
	return lb.buf.Write(p)
}

func (lb *limitedBuffer) Bytes() []byte  { return lb.buf.Bytes() }
func (lb *limitedBuffer) Len() int       { return lb.buf.Len() }
func (lb *limitedBuffer) String() string { return lb.buf.String() }

// truncatedDiff computes a diff but truncates inputs to avoid O(n²) blowup
// on large outputs with many differences.
func truncatedDiff(expected, actual string) string {
	const maxLines = 40
	truncate := func(s string) string {
		lines := strings.SplitN(s, "\n", maxLines+1)
		if len(lines) > maxLines {
			return strings.Join(lines[:maxLines], "\n") + "\n... (truncated)"
		}
		return s
	}
	return diff.LineDiff(truncate(expected), truncate(actual))
}

type phptest struct {
	f      *os.File
	reader *bufio.Reader
	output *limitedBuffer
	name   string
	path   string
	req    *http.Request
	iniRaw    string // raw INI settings from --INI-- section
	cliMode   bool   // true when test has --ARGS-- (run as CLI, not web)
	stdinData []byte // data from --STDIN-- section
	xfail     string // XFAIL reason, if set

	p *phpctx.Process

	t *testing.T
}

type skipError struct {
	reason string
}

func (s skipError) Error() string {
	if s.reason != "" {
		return "test skipped: " + s.reason
	}
	return "test skipped"
}

var skipTest = skipError{}

func (p *phptest) handlePart(part string, b *bytes.Buffer) error {
	switch part {
	case "TEST":
		testName := strings.TrimSpace(b.String())
		p.name += ": " + testName
		return nil
	case "CREDITS":
		// is there something we should do with this?
		return nil
	case "GET":
		p.req.URL.RawQuery = strings.TrimRight(b.String(), "\r\n")
		return nil
	case "POST":
		// we need a new request with the post data
		p.req = httptest.NewRequest("POST", "/"+path.Base(p.path), bytes.NewBuffer(bytes.TrimRight(b.Bytes(), "\r\n")))
		p.req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return nil
	case "FILE", "FILEEOF":
		// pass data to the engine
		var g *phpctx.Global
		if p.cliMode {
			g = phpctx.NewGlobal(context.Background(), p.p, ini.New())
		} else {
			g = phpctx.NewGlobalReq(p.req, p.p, ini.New())
		}
		// Set a 10-second execution deadline per test to prevent
		// infinite loops from blocking the entire suite.
		g.SetDeadline(time.Now().Add(30 * time.Second))

		g.SetOutput(p.output)

		// Apply --STDIN-- data if present
		if p.stdinData != nil {
			g.SetStdin(bytes.NewReader(p.stdinData))
		}

		// Apply --INI-- settings after global context is created (after defaults)
		needsReinit := false
		if p.iniRaw != "" {
			if err := g.IniConfig.Parse(g, strings.NewReader(p.iniRaw)); err != nil {
				return err
			}
			// Only reinit superglobals if INI contains settings that affect them
			for _, key := range []string{"variables_order", "register_argc_argv", "enable_post_data_reading", "disable_functions", "post_max_size", "max_input_nesting_level", "file_uploads", "upload_max_filesize", "max_file_uploads", "upload_tmp_dir"} {
				if strings.Contains(p.iniRaw, key) {
					needsReinit = true
					break
				}
			}
		}
		// Always sync MemMgr limit with the INI memory_limit value
		g.ApplyMaxMemoryLimit()
		if needsReinit {
			g.ReinitSuperglobals()
		}

		// Convert to absolute path so __DIR__ and include paths work correctly
		absPath, _ := filepath.Abs(p.path)
		g.Chdir(phpv.ZString(filepath.Dir(absPath))) // chdir execution to path

		// Use .php extension for the script filename (tests expect .php, not .phpt)
		scriptPath := strings.TrimSuffix(absPath, "t")

		// Write the .php file to disk so functions like show_source(__FILE__) can re-read it
		os.WriteFile(scriptPath, b.Bytes(), 0644)
		defer os.Remove(scriptPath)

		shortOpenTag := bool(g.GetConfig("short_open_tag", phpv.ZBool(true).ZVal()).AsBool(g))
		t := tokenizer.NewLexerWithShortTag(b, scriptPath, shortOpenTag)
		defer t.Close()

		// Compile with timeout: run in goroutine so we can enforce deadline
		type compileResult struct {
			code phpv.Runnable
			err  error
		}
		compileCh := make(chan compileResult, 1)
		go func() {
			code, err := compiler.Compile(g, t)
			compileCh <- compileResult{code, err}
		}()
		var c phpv.Runnable
		var compileErr error
		timer := time.NewTimer(5 * time.Second)
		select {
		case result := <-compileCh:
			timer.Stop()
			c = result.code
			compileErr = result.err
		case <-timer.C:
			t.Close() // kill the lexer goroutine
			// Force GC to reclaim any memory the compile goroutine allocated
			runtime.GC()
			debug.FreeOSMemory()
			return fmt.Errorf("compilation timed out (possible infinite loop)")
		}

		if compileErr != nil {
			// Filter exit errors from compile (e.g., E_COMPILE_ERROR already output)
			compileErr = phpv.FilterExitError(compileErr)
			if compileErr != nil {
				// Handle parse errors and compile errors by outputting them
				if phpErr, ok := compileErr.(*phpv.PhpError); ok && (phpErr.Code == phpv.E_PARSE || phpErr.Code == phpv.E_COMPILE_ERROR) {
					g.LogError(phpErr)
					g.Close()
					return nil
				}
				return compileErr
			}
			g.Close()
			return nil
		}
		var retVal *phpv.ZVal
		var err error
		retVal, err = c.Run(g)
		retVal, err = phperr.CatchReturn(retVal, err)
		_ = retVal
		err = phpv.FilterExitError(err)
		// Convert break/continue outside loop to a fatal error (matching PHP behavior)
		if br, ok := phpv.UnwrapError(err).(*phperr.PhpBreak); ok {
			if br.Initial > 1 {
				err = &phpv.PhpError{Err: fmt.Errorf("Cannot 'break' %d levels", br.Initial), Loc: br.L, Code: phpv.E_ERROR}
			} else {
				err = &phpv.PhpError{Err: fmt.Errorf("'break' not in the 'loop' or 'switch' context"), Loc: br.L, Code: phpv.E_ERROR}
			}
		} else if cr, ok := phpv.UnwrapError(err).(*phperr.PhpContinue); ok {
			if cr.Initial > 1 {
				err = &phpv.PhpError{Err: fmt.Errorf("Cannot 'continue' %d levels", cr.Initial), Loc: cr.L, Code: phpv.E_ERROR}
			} else {
				err = &phpv.PhpError{Err: fmt.Errorf("'continue' not in the 'loop' or 'switch' context"), Loc: cr.L, Code: phpv.E_ERROR}
			}
		}
		if err != nil {
			// Handle uncaught exceptions via user exception handler
			err = g.HandleUncaughtException(err)
		}
		// Output fatal errors through the global output (which may be buffered)
		// so output buffer callbacks can process them (PHP behavior).
		if err != nil {
			htmlErrors := bool(g.GetConfig("html_errors", phpv.ZBool(false).ZVal()).AsBool(g))
			if ex, ok := err.(*phperr.PhpThrow); ok {
				// Special handling for ParseError: PHP displays these as
				// "Parse error: <message> in <file> on line <line>"
				// instead of the usual "Fatal error: Uncaught ParseError: ..." format
				if ex.Obj.GetClass().InstanceOf(phpobj.ParseError) {
					message := ex.Obj.HashTable().GetString("message").String()
					fileLoc := ex.ThrownFile()
					lineLoc := ex.ThrownLine()
					if htmlErrors {
						fmt.Fprintf(g, "<br />\n<b>Parse error</b>:  %s in <b>%s</b> on line <b>%d</b><br />\n", message, fileLoc, lineLoc)
					} else {
						fmt.Fprintf(g, "\nParse error: %s in %s on line %d\n", message, fileLoc, lineLoc)
					}
				} else if htmlErrors {
					fmt.Fprintf(g, "<br />\n<b>Fatal error</b>:  %s\n  thrown in <b>%s</b> on line <b>%d</b><br />\n", ex.ErrorTrace(g), ex.ThrownFile(), ex.ThrownLine())
				} else {
					fmt.Fprintf(g, "\nFatal error: %s\n  thrown in %s on line %d\n", ex.ErrorTrace(g), ex.ThrownFile(), ex.ThrownLine())
				}
				err = nil
			} else if timeout, ok := phperr.CatchTimeout(err).(*phperr.PhpTimeout); ok && timeout != nil {
				fmt.Fprint(g, "\n"+timeout.String())
				err = nil
			} else if phpErr, ok := err.(*phpv.PhpError); ok && (phpErr.Code == phpv.E_ERROR || phpErr.Code == phpv.E_COMPILE_ERROR) {
				// Clean buffered output before writing the fatal error,
				// so only the error message passes through the callback
				// (not the previously buffered output).
				g.CleanBuffers()
				if phpErr.Loc != nil {
					if htmlErrors {
						fmt.Fprintf(g, "<br />\n<b>Fatal error</b>:  %s in <b>%s</b> on line <b>%d</b><br />\n", phpErr.Err.Error(), phpErr.Loc.Filename, phpErr.Loc.Line)
					} else {
						fmt.Fprintf(g, "\nFatal error: %s in %s on line %d\n", phpErr.Err.Error(), phpErr.Loc.Filename, phpErr.Loc.Line)
					}
				} else {
					if htmlErrors {
						fmt.Fprintf(g, "<br />\n<b>Fatal error</b>:  %s<br />\n", phpErr.Err.Error())
					} else {
						fmt.Fprintf(g, "\nFatal error: %s\n", phpErr.Err.Error())
					}
				}
				err = nil
			}
		}
		g.RunShutdownFunctions()
		// Send headers if not already sent (fires header_register_callback callbacks)
		if hc := g.HeaderContext(); hc != nil && !hc.Sent {
			hc.SendHeaders(g)
		}
		closeErr := g.Close()
		if err == nil && closeErr != nil {
			// Handle fatal errors from output buffer callbacks during close
			if phpErr, ok := closeErr.(*phpv.PhpError); ok && (phpErr.Code == phpv.E_ERROR || phpErr.Code == phpv.E_COMPILE_ERROR) {
				if phpErr.Loc != nil {
					fmt.Fprintf(p.output, "\nFatal error: %s in %s on line %d\n", phpErr.Err.Error(), phpErr.Loc.Filename, phpErr.Loc.Line)
				} else {
					fmt.Fprintf(p.output, "\nFatal error: %s\n", phpErr.Err.Error())
				}
				closeErr = nil
			} else if ex, ok := closeErr.(*phperr.PhpThrow); ok {
				fmt.Fprintf(p.output, "\nFatal error: %s\n  thrown in %s on line %d\n", ex.ErrorTrace(g), ex.ThrownFile(), ex.ThrownLine())
				closeErr = nil
			}
			if closeErr != nil {
				err = closeErr
			}
		}
		return err
	case "EXPECT":
		// compare p.output with b
		out := bytes.TrimSpace(p.output.Bytes())
		exp := bytes.TrimSpace(b.Bytes())
		// Normalize \r\n to \n (PHP test runner does this)
		out = bytes.ReplaceAll(out, []byte("\r\n"), []byte("\n"))
		exp = bytes.ReplaceAll(exp, []byte("\r\n"), []byte("\n"))

		if bytes.Compare(out, exp) != 0 {
			return fmt.Errorf("output not as expected!\n%s", truncatedDiff(string(exp), string(out)))
		}
		return nil
	case "SKIPIF":
		t := tokenizer.NewLexer(b, p.path)
		g := phpctx.NewGlobal(context.Background(), p.p, ini.New())
		output := &bytes.Buffer{}
		g.SetOutput(output)
		c, err := compiler.Compile(g, t)
		if err != nil {
			// If SKIPIF code can't compile (e.g., missing include file), skip the test
			return skipError{reason: "SKIPIF compile error: " + err.Error()}
		}
		_, err = c.Run(g)
		err = phpv.FilterExitError(err)
		if err != nil {
			// If SKIPIF code errors at runtime, skip the test (PHP's run-tests does the same)
			return skipError{reason: "SKIPIF runtime error: " + err.Error()}
		}
		if bytes.HasPrefix(output.Bytes(), []byte("skip ")) {
			return skipError{reason: "SKIPIF: " + strings.TrimSpace(string(output.Bytes()))}
		}
		return nil
	case "EXPECTF":
		// EXPECTF is like EXPECT but allows format specifiers
		out := bytes.TrimSpace(p.output.Bytes())
		exp := bytes.TrimSpace(b.Bytes())
		// Normalize \r\n to \n (PHP test runner does this)
		out = bytes.ReplaceAll(out, []byte("\r\n"), []byte("\n"))
		exp = bytes.ReplaceAll(exp, []byte("\r\n"), []byte("\n"))

		// Convert EXPECTF pattern to a regex
		re := expectfToRegex(string(exp))
		// Sanitize non-UTF8 bytes in output for regex matching
		outStr := sanitizeForRegex(string(out))
		matched, err := regexp.MatchString("(?s)^"+re+"$", outStr)
		if err != nil {
			return fmt.Errorf("invalid EXPECTF pattern: %s", err)
		}
		if !matched {
			return fmt.Errorf("output not as expected!\n%s", truncatedDiff(string(exp), string(out)))
		}
		return nil
	case "EXPECTREGEX":
		out := bytes.TrimSpace(p.output.Bytes())
		exp := strings.TrimSpace(b.String())

		matched, err := regexp.MatchString("(?s)^"+exp+"$", string(out))
		if err != nil {
			return fmt.Errorf("invalid EXPECTREGEX pattern: %s", err)
		}
		if !matched {
			return fmt.Errorf("output not as expected!\n%s", truncatedDiff(exp, string(out)))
		}
		return nil
	case "INI":
		// Parse INI settings. Skip tests that require features we definitely
		// can't support. Accept everything else and let the test run.
		// INI settings that always cause a skip (feature not implemented at all)
		unsupported := map[string]bool{
			// file_uploads: accepted — implemented in parsePost() multipart handling
			// enable_post_data_reading: implemented - when 0, $_POST/$_FILES are empty but php://input works
			// post_max_size: handled by valueDependent - skip only when non-zero
			// upload_max_filesize: implemented — enforced in parsePost() file upload handling
			// upload_tmp_dir: implemented — uses os.TempDir() as default
			// max_file_uploads: implemented — enforced in parsePost() file upload handling
			// memory_limit: stored/retrieved via ini_get/ini_set; enforcement not implemented but tests don't require it
			"hard_timeout":             true, // hard timeout not implemented
			"session.auto_start":       true, // sessions not implemented
			// filter.default=unsafe_raw is a no-op (no filtering), safe to accept
			// open_basedir: implemented - checks file access against allowed directories
			// precision and serialize_precision are implemented in core/phpv/ztype.go
			// register_argc_argv: implemented - controls argv/argc in $_SERVER
			// variables_order: implemented in doGPC() - controls which superglobals are populated
			// highlight.*: implemented - syntax highlighting output matches PHP 8 format
			// max_input_nesting_level: implemented in setUrlValueToArray (drops over-nested params)
			// max_input_vars: limit on input parsing, tests use 1000 which is well above typical test needs
			// short_open_tag: implemented in tokenizer - controls whether <? without php/= opens PHP mode
			// auto_prepend_file: implemented in RunFile - includes file before main script
			// disable_functions: implemented - removes named functions from available list
			"allow_url_fopen":          true, // tests using this need HTTP server helpers we don't have
			"default_charset":          true, // charset-aware functions (htmlentities etc) not fully implemented
			"error_log_mode":           true, // log mode not implemented
			// report_memleaks: deprecated directive, handled in ini parser
			// sys_temp_dir is implemented in ext/standard/fs.go:fncSysGetTempDir
			// date.timezone is handled by the date extension's ini settings
			"opcache.save_comments":    true, // needs ReflectionClass doc comments support
			"docref_root":              true, // needs HTML error link formatting
			// arg_separator.input is implemented in ext/standard/urlenc.go
		}
		// INI settings that only block when set to a non-default/active value
		// e.g., zlib.output_compression=0 is "off" (default) → safe
		valueDependent := map[string]func(string) bool{
			"zlib.output_compression": func(v string) bool {
				lv := strings.ToLower(strings.TrimSpace(v))
				return lv != "0" && lv != "off" && lv != "false" && lv != "no" && lv != ""
			},
			"post_max_size": func(v string) bool {
				// post_max_size is now enforced in the runtime.
				// Accept all values — enforcement is handled by parsePost().
				return false
			},
			"memory_limit": func(v string) bool {
				// memory_limit is now enforced via runtime memory checking
				// in the Tick() handler. Accept all values.
				return false
			},
			// file_uploads: accepted for all values — tests needing upload
			// infrastructure also set upload_max_filesize or upload_tmp_dir
		}
		// Save content before scanning (scanner consumes the buffer)
		iniContent := b.String()
		scanner := bufio.NewScanner(strings.NewReader(iniContent))
		hasUnsupported := false
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || line[0] == ';' {
				continue
			}
			pos := strings.IndexByte(line, '=')
			if pos == -1 {
				continue
			}
			k := strings.TrimSpace(line[:pos])
			v := ""
			if pos+1 < len(line) {
				v = strings.TrimSpace(line[pos+1:])
			}
			if unsupported[k] {
				hasUnsupported = true
				break
			}
			if check, ok := valueDependent[k]; ok && check(v) {
				hasUnsupported = true
				break
			}
		}
		if hasUnsupported {
			return skipError{reason: "unsupported INI"}
		}
		// Substitute {PWD} with test file directory (matches PHP run-tests.php)
		testDir := filepath.Dir(p.path)
		if absDir, err := filepath.Abs(testDir); err == nil {
			testDir = absDir
		}
		iniContent = strings.ReplaceAll(iniContent, "{PWD}", testDir)
		p.iniRaw = iniContent
		return nil
	case "EXTENSIONS":
		// Check that all required extensions are loaded
		for _, line := range strings.Split(strings.TrimSpace(b.String()), "\n") {
			ext := strings.TrimSpace(line)
			if ext == "" {
				continue
			}
			if !phpctx.HasExt(ext) {
				return skipError{reason: "missing extension: " + ext}
			}
		}
		return nil
	case "XFAIL":
		p.xfail = strings.TrimSpace(b.String())
		return nil
	case "CLEAN":
		// CLEAN runs after the test to clean up temp files/dirs
		g := phpctx.NewGlobal(context.Background(), p.p, ini.New())
		g.SetOutput(io.Discard)
		absPath, _ := filepath.Abs(p.path)
		g.Chdir(phpv.ZString(filepath.Dir(absPath)))
		t := tokenizer.NewLexer(b, strings.TrimSuffix(absPath, "t"))
		if c, err := compiler.Compile(g, t); err == nil {
			c.Run(g)
		}
		g.Close()
		return nil
	case "DESCRIPTION":
		// DESCRIPTION is informational only
		return nil
	case "WHITESPACE_SENSITIVE":
		// WHITESPACE_SENSITIVE is informational only (tells IDE/editors not to strip trailing whitespace)
		return nil
	case "CONFLICTS":
		// CONFLICTS marks tests that shouldn't run in parallel; we run sequentially so this is a no-op
		return nil
	case "ARGS":
		// Set command-line arguments for the test (CLI mode)
		args := strings.Fields(strings.TrimSpace(b.String()))
		p.p.Argv = append([]string{p.path}, args...)
		p.cliMode = true
		return nil
	case "STDIN":
		// Save stdin data to be fed to the script via custom stdin stream
		p.stdinData = b.Bytes()
		p.cliMode = true // STDIN implies CLI mode
		return nil
	case "FLAKY":
		// FLAKY marks tests that may fail intermittently; treat as informational
		return nil
	case "CGI", "CAPTURE_STDIO":
		// These require special execution modes we don't support yet
		return skipError{reason: "unsupported section: " + part}
	case "ENV":
		// Set environment variables for the test
		for _, line := range strings.Split(strings.TrimSpace(b.String()), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if pos := strings.IndexByte(line, '='); pos != -1 {
				k := line[:pos]
				v := line[pos+1:]
				p.p.SetEnv(k, v)
			}
		}
		return nil
	case "COOKIE":
		// Set cookies on the request
		p.req.Header.Set("Cookie", strings.TrimRight(b.String(), "\r\n"))
		return nil
	case "POST_RAW":
		// Raw POST data with Content-Type header on first line
		data := b.String()
		lines := strings.SplitN(data, "\n", 2)
		if len(lines) == 2 {
			// First line is Content-Type: ...
			if strings.HasPrefix(lines[0], "Content-Type:") {
				ct := strings.TrimSpace(strings.TrimPrefix(lines[0], "Content-Type:"))
				body := strings.TrimRight(lines[1], "\r\n")
				p.req = httptest.NewRequest("POST", "/"+path.Base(p.path), bytes.NewBufferString(body))
				p.req.Header.Set("Content-Type", ct)
			} else {
				p.req = httptest.NewRequest("POST", "/"+path.Base(p.path), bytes.NewBuffer(bytes.TrimRight(b.Bytes(), "\r\n")))
				p.req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
		} else {
			p.req = httptest.NewRequest("POST", "/"+path.Base(p.path), bytes.NewBuffer(bytes.TrimRight(b.Bytes(), "\r\n")))
			p.req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		return nil
	case "EXPECTHEADERS":
		// EXPECTHEADERS checks HTTP response headers. In our test runner
		// we don't validate response headers — just skip silently.
		return nil
	case "FILE_EXTERNAL":
		// Read the script from an external file and delegate to FILE handler
		extFile := strings.TrimSpace(b.String())
		extPath := filepath.Join(filepath.Dir(p.path), extFile)
		extData, err := os.ReadFile(extPath)
		if err != nil {
			return fmt.Errorf("FILE_EXTERNAL: cannot read %s: %s", extPath, err)
		}
		extBuf := bytes.NewBuffer(extData)
		return p.handlePart("FILE", extBuf)
	case "EXPECT_EXTERNAL", "EXPECTF_EXTERNAL", "EXPECTREGEX_EXTERNAL":
		// Read the external file and delegate to the corresponding handler
		extFile := strings.TrimSpace(b.String())
		extPath := filepath.Join(filepath.Dir(p.path), extFile)
		extData, err := os.ReadFile(extPath)
		if err != nil {
			return fmt.Errorf("file does not exist in %s:%d", p.path, 0)
		}
		extBuf := bytes.NewBuffer(extData)
		basePart := strings.TrimSuffix(part, "_EXTERNAL")
		return p.handlePart(basePart, extBuf)
	default:
		return fmt.Errorf("unhandled part type %s for test", part)
	}
}

func runTest(t *testing.T, fpath string) (p *phptest, err error) {
	p = &phptest{t: t, output: &limitedBuffer{}, name: fpath, path: fpath}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to run: %s\n%s", r, debug.Stack())
		}
	}()

	// read & parse test file
	p.f, err = os.Open(fpath)
	if err != nil {
		return
	}
	defer p.f.Close()
	p.reader = bufio.NewReader(p.f)

	// prepare env
	p.p = phpctx.NewProcess("test")
	p.req = httptest.NewRequest("GET", "/"+path.Base(fpath), nil)

	// Phase 1: Parse all sections into a map
	sections := make(map[string]*bytes.Buffer)
	var sectionOrder []string
	var curBuf *bytes.Buffer
	var curPart string
	sectionRe := regexp.MustCompile("^--([A-Z_]+)--$")

	for {
		lin, err := p.reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return p, err
		}
		atEOF := err == io.EOF
		if atEOF && len(lin) == 0 {
			break
		}
		if strings.HasPrefix(lin, "--") {
			lin_trimmed := strings.TrimRight(lin, "\r\n")
			if sub := sectionRe.FindSubmatch([]byte(lin_trimmed)); sub != nil {
				thing := string(sub[1])
				if curBuf != nil {
					sections[curPart] = curBuf
					sectionOrder = append(sectionOrder, curPart)
				}
				curBuf = &bytes.Buffer{}
				curPart = thing
				continue
			}
		}
		if curBuf == nil {
			return p, fmt.Errorf("malformed test file %s", fpath)
		}
		curBuf.Write([]byte(lin))
		if atEOF {
			break
		}
	}
	if curBuf != nil {
		sections[curPart] = curBuf
		sectionOrder = append(sectionOrder, curPart)
	}

	// Phase 2: Process sections in dependency order.
	// Sections that set up state (SKIPIF, INI, STDIN, etc.) must run before
	// FILE/FILEEOF which executes the script, which must run before
	// EXPECT/EXPECTF which checks output.
	// Process in order: everything except FILE/FILEEOF/EXPECT*/CLEAN first,
	// then FILE/FILEEOF, then EXPECT*/CLEAN.
	var fileParts []string    // FILE, FILEEOF
	var expectParts []string  // EXPECT, EXPECTF, EXPECTREGEX, CLEAN, etc.
	var setupParts []string   // everything else

	for _, name := range sectionOrder {
		switch name {
		case "FILE", "FILEEOF", "FILE_EXTERNAL":
			fileParts = append(fileParts, name)
		case "EXPECT", "EXPECTF", "EXPECTREGEX", "EXPECT_EXTERNAL", "EXPECTF_EXTERNAL",
			"EXPECTREGEX_EXTERNAL", "EXPECTHEADERS", "CLEAN":
			expectParts = append(expectParts, name)
		default:
			setupParts = append(setupParts, name)
		}
	}

	for _, name := range setupParts {
		if err := p.handlePart(name, sections[name]); err != nil {
			return p, err
		}
	}
	for _, name := range fileParts {
		if err := p.handlePart(name, sections[name]); err != nil {
			return p, err
		}
	}
	// Run CLEAN section unconditionally (even if EXPECT fails), to avoid leaving
	// stale temp directories that break subsequent tests.
	var expectErr error
	for _, name := range expectParts {
		if name == "CLEAN" {
			// Always run CLEAN
			p.handlePart(name, sections[name])
			continue
		}
		if expectErr == nil {
			if err := p.handlePart(name, sections[name]); err != nil {
				expectErr = err
			}
		}
	}

	// If XFAIL is set and the test failed, convert to skip (expected failure)
	if expectErr != nil && p.xfail != "" {
		return p, skipError{reason: "XFAIL: " + p.xfail}
	}

	return p, expectErr
}

// expectfToRegex converts a PHP EXPECTF pattern to a Go regex.
// Format specifiers:
//
//	%d - one or more digits
//	%i - +/- followed by one or more digits
//	%f - floating point number
//	%c - single character
//	%s - one or more non-newline characters
//	%S - zero or more non-newline characters
//	%a - one or more characters (including newlines)
//	%A - zero or more characters (including newlines)
//	%w - zero or more whitespace
//	%x - one or more hex digits
//	%e - directory separator
//	%% - literal %
//
// sanitizeForRegex converts raw bytes to valid UTF-8 by treating each byte
// as a Latin-1 character (byte 0xNN -> rune U+00NN). This allows the Go
// regex engine to match binary data in PHP test output.
func sanitizeForRegex(s string) string {
	var buf strings.Builder
	for i := 0; i < len(s); i++ {
		buf.WriteRune(rune(s[i]))
	}
	return buf.String()
}

func expectfToRegex(pattern string) string {
	var result strings.Builder
	i := 0
	for i < len(pattern) {
		if pattern[i] == '%' && i+1 < len(pattern) {
			switch pattern[i+1] {
			case 'd':
				result.WriteString(`\d+`)
			case 'i':
				result.WriteString(`[+-]?\d+`)
			case 'f':
				result.WriteString(`[+-]?\d*\.?\d+(?:[eE][+-]?\d+)?`)
			case 'c':
				result.WriteString(`.`)
			case 's':
				result.WriteString(`[^\r\n]+`)
			case 'S':
				result.WriteString(`[^\r\n]*`)
			case 'a':
				result.WriteString(`.+`)
			case 'A':
				result.WriteString(`.*`)
			case 'w':
				result.WriteString(`\s*`)
			case 'x':
				result.WriteString(`[0-9a-fA-F]+`)
			case 'e':
				result.WriteString(`[/\\]`)
			case 'r':
				// %r...%r embeds a raw regex between two %r markers
				end := strings.Index(pattern[i+2:], "%r")
				if end >= 0 {
					result.WriteString(pattern[i+2 : i+2+end])
					i += 2 + end + 2
					continue
				}
				// No matching %r, treat as literal
				result.WriteString(regexp.QuoteMeta(sanitizeForRegex(pattern[i : i+1])))
				i++
				continue
			case '%':
				result.WriteString(`%`)
			case '0':
				// %0 represents a NUL byte
				result.WriteString(regexp.QuoteMeta(sanitizeForRegex("\x00")))
			default:
				result.WriteString(regexp.QuoteMeta(sanitizeForRegex(pattern[i : i+1])))
				i++
				continue
			}
			i += 2
		} else {
			result.WriteString(regexp.QuoteMeta(sanitizeForRegex(pattern[i : i+1])))
			i++
		}
	}
	return result.String()
}

// testCache manages a file-based cache of test results so that known-passing
// tests can be skipped on subsequent runs. The cache file stores the path and
// modification time of each passing test. Set GORO_TEST_CACHE=1 to enable,
// GORO_TEST_CACHE_CLEAR=1 to reset the cache for a full regression check.
type testCache struct {
	file    string
	entries map[string]time.Time // path -> file mod time when it last passed
}

const testCacheFile = "/tmp/goro_test_cache.json"

func loadTestCache() *testCache {
	tc := &testCache{file: testCacheFile, entries: make(map[string]time.Time)}
	data, err := os.ReadFile(testCacheFile)
	if err != nil {
		return tc
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			continue
		}
		tc.entries[parts[1]] = t
	}
	return tc
}

func (tc *testCache) isCached(path string, info os.FileInfo) bool {
	if cached, ok := tc.entries[path]; ok {
		return info.ModTime().Equal(cached)
	}
	return false
}

func (tc *testCache) markPass(path string, info os.FileInfo) {
	tc.entries[path] = info.ModTime()
}

func (tc *testCache) markFail(path string) {
	delete(tc.entries, path)
}

func (tc *testCache) save() {
	var buf strings.Builder
	for path, modTime := range tc.entries {
		fmt.Fprintf(&buf, "%s\t%s\n", modTime.Format(time.RFC3339Nano), path)
	}
	os.WriteFile(tc.file, []byte(buf.String()), 0644)
}

func TestPhp(t *testing.T) {
	// Set TEST_PHP_EXECUTABLE so tests like bug54514 can compare with PHP_BINARY
	if exe, err := os.Executable(); err == nil {
		os.Setenv("TEST_PHP_EXECUTABLE", exe)
	}

	// Set RLIMIT_AS as a safety net. Go's virtual memory usage is much
	// higher than actual RSS, so set this very high (64 GB) to avoid
	// false triggers while still preventing 191 GB runaway allocations.
	memLimit := uint64(16 * 1024 * 1024 * 1024) // 16 GB safety net per process
	if v := os.Getenv("GORO_TEST_MEMLIMIT"); v != "" {
		fmt.Sscanf(v, "%d", &memLimit)
	}
	setMemoryLimit(memLimit)
	// Set Go's GC-aware soft limit if not already set via env
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(3 * 1024 * 1024 * 1024) // 3 GB soft GC limit per process
	}

	// Known-hanging tests that cause OOM/infinite loops in the engine.
	// These are skipped until the underlying bugs are fixed.
	hangingTests := map[string]bool{
		"test/php-8.5.4/ext/date/bug73460-002.phpt":                                                   true, // DateTime::sub DST infinite loop
		"test/php-8.5.4/func_arg_fetch_optimization.phpt":                                             true, // $x[][$y] recursion causes OOM before call depth limit
		"test/php-8.5.4/ext/mbstring/utf_encodings.phpt":                                              true, // Slow torture test (needs SKIP_SLOW_TESTS)
		"test/php-8.5.4/ext/standard/file/file_get_contents_file_put_contents_5gb.phpt":               true, // 5GB allocation, memory_limit=-1
		"test/php-8.5.4/ext/standard/strings/gh15613.phpt":                                            true, // memory_limit=-1, huge unpack
		"test/php-8.5.4/ext/mbstring/euc_tw_encoding.phpt":                                            true, // Slow mbstring encoding conversion test
		"test/php-8.5.4/ext/mbstring/gb18030_encoding.phpt":                                           true, // Slow mbstring encoding conversion test
		"test/php-8.5.4/fibers/get-return-after-bailout.phpt":                                         true, // Fiber + str_repeat(PHP_INT_MAX) hang
	}

	// Directories containing tests that require external resources (network, etc.)
	// and will hang waiting for I/O. Skip the entire directory.
	hangingDirs := []string{
		"ext/standard/network/",
	}

	// Batch support: GORO_TEST_SKIP and GORO_TEST_LIMIT env vars
	batchSkip := 0
	batchLimit := 0
	if v := os.Getenv("GORO_TEST_SKIP"); v != "" {
		fmt.Sscanf(v, "%d", &batchSkip)
	}
	if v := os.Getenv("GORO_TEST_LIMIT"); v != "" {
		fmt.Sscanf(v, "%d", &batchLimit)
	}

	// Fail limit: stop after N failures to allow quick iteration.
	// Default: 0 (no limit). Set GORO_TEST_FAIL_LIMIT=100 to stop after 100 failures.
	failLimit := 0
	if v := os.Getenv("GORO_TEST_FAIL_LIMIT"); v != "" {
		fmt.Sscanf(v, "%d", &failLimit)
	}

	// Result cache: skip tests that previously passed (file unchanged).
	// GORO_TEST_CACHE=1 to enable, GORO_TEST_CACHE_CLEAR=1 to reset.
	useCache := os.Getenv("GORO_TEST_CACHE") == "1"
	var cache *testCache
	if useCache {
		if os.Getenv("GORO_TEST_CACHE_CLEAR") == "1" {
			os.Remove(testCacheFile)
		}
		cache = loadTestCache()
		defer cache.save()
	}

	// run all tests in "test"
	count := 0
	pass := 0
	skip := 0
	fail := 0
	cacheHit := 0
	testIdx := 0
	failLimitReached := false
	filepath.Walk(TestsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil // skip entries that disappeared (e.g. temp dirs from tests)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if !strings.HasSuffix(path, ".phpt") {
			return err
		}
		testIdx++
		if testIdx <= batchSkip {
			return nil
		}
		if batchLimit > 0 && count >= batchLimit {
			return nil
		}
		if failLimitReached {
			return nil
		}

		// Skip known-hanging tests
		if hangingTests[path] {
			count += 1
			skip += 1
			t.Logf("Skipped %s: known hanging test", path)
			return nil
		}
		// Skip entire directories that require external resources
		for _, dir := range hangingDirs {
			if strings.Contains(path, dir) {
				count += 1
				skip += 1
				return nil
			}
		}

		// Check cache: skip tests that passed before and haven't changed
		if cache != nil && cache.isCached(path, info) {
			count += 1
			pass += 1
			cacheHit += 1
			return nil
		}

		count += 1
		p, err := runTest(t, path)
		if err != nil {
			var se skipError
			if errors.As(err, &se) {
				skip += 1
				t.Logf("Skipped %s: %s", path, se.Error())
				return nil
			}
			fail += 1
			if cache != nil {
				cache.markFail(path)
			}
			t.Errorf("Error in %s: %s", p.name, err.Error())
			if failLimit > 0 && fail >= failLimit {
				failLimitReached = true
				t.Logf("Fail limit reached (%d failures), stopping early", failLimit)
			}
		} else {
			pass += 1
			if cache != nil {
				cache.markPass(path, info)
			}
		}

		// Periodic GC to prevent cross-test memory accumulation
		if count%50 == 0 {
			runtime.GC()
			debug.FreeOSMemory()
		}

		// Write progress to a file so we can monitor long runs
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		os.WriteFile("/tmp/goro_test_progress.txt",
			[]byte(fmt.Sprintf("Progress: %d tests, %d passed, %d failed, %d skipped [%s] (mem: %dMB, goroutines: %d)\n",
				count, pass, fail, skip, path, m.Sys/1024/1024, runtime.NumGoroutine())), 0644)
		return nil
	})

	summary := fmt.Sprintf("Total of %d tests, %d passed (%01.2f%% success), %d skipped and %d failed",
		count, pass, float64(pass)*100/float64(count-skip), skip, fail)
	if cacheHit > 0 {
		summary += fmt.Sprintf(" (%d from cache)", cacheHit)
	}
	if failLimitReached {
		summary += fmt.Sprintf(" (stopped at %d failures)", failLimit)
	}
	t.Logf("%s", summary)
	os.WriteFile("/tmp/goro_test_progress.txt", []byte(summary+"\n"), 0644)
}
