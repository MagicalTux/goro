package main_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"testing"

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
const TestsPath = "test"

type phptest struct {
	f      *os.File
	reader *bufio.Reader
	output *bytes.Buffer
	name   string
	path   string
	req    *http.Request
	iniRaw string // raw INI settings from --INI-- section

	p *phpctx.Process

	t *testing.T
}

type skipError struct{}

func (s skipError) Error() string {
	return "test skipped"
}

var skipTest skipError

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
		g := phpctx.NewGlobalReq(p.req, p.p, ini.New())

		// Apply --INI-- settings after global context is created (after defaults)
		if p.iniRaw != "" {
			if err := g.IniConfig.Parse(g, strings.NewReader(p.iniRaw)); err != nil {
				return err
			}
		}
		g.SetOutput(p.output)

		// Convert to absolute path so __DIR__ and include paths work correctly
		absPath, _ := filepath.Abs(p.path)
		g.Chdir(phpv.ZString(filepath.Dir(absPath))) // chdir execution to path

		// Use .php extension for the script filename (tests expect .php, not .phpt)
		scriptPath := strings.TrimSuffix(absPath, "t")

		// Write the .php file to disk so functions like show_source(__FILE__) can re-read it
		os.WriteFile(scriptPath, b.Bytes(), 0644)
		defer os.Remove(scriptPath)

		t := tokenizer.NewLexer(b, scriptPath)
		c, err := compiler.Compile(g, t)
		if err != nil {
			// Filter exit errors from compile (e.g., E_COMPILE_ERROR already output)
			err = phpv.FilterExitError(err)
			if err != nil {
				// Handle parse errors and compile errors by outputting them
				if phpErr, ok := err.(*phpv.PhpError); ok && (phpErr.Code == phpv.E_PARSE || phpErr.Code == phpv.E_COMPILE_ERROR) {
					g.LogError(phpErr)
					g.Close()
					return nil
				}
				return err
			}
			g.Close()
			return nil
		}
		var retVal *phpv.ZVal
		retVal, err = c.Run(g)
		retVal, err = phperr.CatchReturn(retVal, err)
		_ = retVal
		err = phpv.FilterExitError(err)
		if err != nil {
			// Handle uncaught exceptions via user exception handler before closing buffers
			if ex, ok := err.(*phperr.PhpThrow); ok {
				if handler := g.GetUserExceptionHandler(); handler != nil {
					g.SetUserExceptionHandler(nil) // prevent re-entrancy
					_, handlerErr := g.CallZVal(g, handler, []*phpv.ZVal{ex.Obj.ZVal()})
					if handlerErr == nil {
						err = nil
					} else {
						err = handlerErr
					}
				}
			}
		}
		// Output fatal errors through the global output (which may be buffered)
		// so output buffer callbacks can process them (PHP behavior).
		if err != nil {
			htmlErrors := bool(g.GetConfig("html_errors", phpv.ZBool(false).ZVal()).AsBool(g))
			if ex, ok := err.(*phperr.PhpThrow); ok {
				loc := ex.Loc
				if loc == nil {
					loc = &phpv.Loc{}
				}
				// Special handling for ParseError: PHP displays these as
				// "Parse error: <message> in <file> on line <line>"
				// instead of the usual "Fatal error: Uncaught ParseError: ..." format
				if ex.Obj.GetClass().InstanceOf(phpobj.ParseError) {
					message := ex.Obj.HashTable().GetString("message").String()
					fileLoc := ex.Obj.HashTable().GetString("file").String()
					lineLoc := ex.Obj.HashTable().GetString("line").AsInt(g)
					if fileLoc == "" {
						fileLoc = loc.Filename
						lineLoc = phpv.ZInt(loc.Line)
					}
					if htmlErrors {
						fmt.Fprintf(g, "<br />\n<b>Parse error</b>:  %s in <b>%s</b> on line <b>%d</b><br />\n", message, fileLoc, lineLoc)
					} else {
						fmt.Fprintf(g, "\nParse error: %s in %s on line %d\n", message, fileLoc, lineLoc)
					}
				} else if htmlErrors {
					fmt.Fprintf(g, "<br />\n<b>Fatal error</b>:  %s\n  thrown in <b>%s</b> on line <b>%d</b><br />\n", ex.ErrorTrace(g), loc.Filename, loc.Line)
				} else {
					fmt.Fprintf(g, "\nFatal error: %s\n  thrown in %s on line %d\n", ex.ErrorTrace(g), loc.Filename, loc.Line)
				}
				err = nil
			} else if timeout, ok := phperr.CatchTimeout(err).(*phperr.PhpTimeout); ok && timeout != nil {
				fmt.Fprint(g, "\n"+timeout.String())
				err = nil
			} else if phpErr, ok := err.(*phpv.PhpError); ok && phpErr.Code == phpv.E_ERROR {
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
			if phpErr, ok := closeErr.(*phpv.PhpError); ok && phpErr.Code == phpv.E_ERROR {
				if phpErr.Loc != nil {
					fmt.Fprintf(p.output, "\nFatal error: %s in %s on line %d\n", phpErr.Err.Error(), phpErr.Loc.Filename, phpErr.Loc.Line)
				} else {
					fmt.Fprintf(p.output, "\nFatal error: %s\n", phpErr.Err.Error())
				}
				closeErr = nil
			} else if ex, ok := closeErr.(*phperr.PhpThrow); ok {
				fmt.Fprintf(p.output, "\nFatal error: %s\n  thrown in %s on line %d\n", ex.ErrorTrace(g), ex.Loc.Filename, ex.Loc.Line)
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
			return fmt.Errorf("output not as expected!\n%s", diff.LineDiff(string(exp), string(out)))
		}
		return nil
	case "SKIPIF":
		t := tokenizer.NewLexer(b, p.path)
		g := phpctx.NewGlobal(context.Background(), p.p, ini.New())
		output := &bytes.Buffer{}
		g.SetOutput(output)
		c, err := compiler.Compile(g, t)
		if err != nil {
			return err
		}
		_, err = c.Run(g)
		err = phpv.FilterExitError(err)
		if err != nil {
			return err
		}
		if bytes.HasPrefix(output.Bytes(), []byte("skip ")) {
			return skipTest
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
			return fmt.Errorf("output not as expected!\n%s", diff.LineDiff(string(exp), string(out)))
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
			return fmt.Errorf("output not as expected!\n%s", diff.LineDiff(exp, string(out)))
		}
		return nil
	case "INI":
		// Parse INI settings. Only enable tests whose INI settings we support.
		supported := map[string]bool{
			"error_reporting":        true,
			"display_errors":         true,
			"display_startup_errors": true,
			"log_errors":             true,
			"html_errors":            true,
			"include_path":           true,
			"error_log":              true,
			"max_execution_time":     true,
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
			if !supported[k] {
				hasUnsupported = true
				break
			}
		}
		if hasUnsupported {
			return skipTest
		}
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
				return skipTest
			}
		}
		return nil
	case "XFAIL":
		// TODO but safe to ignore
		return nil
	case "CLEAN", "DESCRIPTION":
		// CLEAN runs after the test (cleanup temp files etc) - not needed
		// DESCRIPTION is informational only
		return nil
	case "STDIN", "ARGS", "CGI", "CAPTURE_STDIO":
		// These require special execution modes we don't support yet
		return skipTest
	case "ENV":
		// TODO: set environment variables for the test
		return skipTest
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
	case "EXPECT_EXTERNAL", "EXPECTF_EXTERNAL", "EXPECTREGEX_EXTERNAL":
		// External expect files - skip for now
		return skipTest
	default:
		return fmt.Errorf("unhandled part type %s for test", part)
	}
}

func runTest(t *testing.T, fpath string) (p *phptest, err error) {
	p = &phptest{t: t, output: &bytes.Buffer{}, name: fpath, path: fpath}

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

	var b *bytes.Buffer
	var part string

	// prepare env
	p.p = phpctx.NewProcess("test")
	p.req = httptest.NewRequest("GET", "/"+path.Base(fpath), nil)
	r := regexp.MustCompile("^--([A-Z_]+)--$")

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

			if sub := r.FindSubmatch([]byte(lin_trimmed)); sub != nil {
				thing := string(sub[1])
				// start of a new thing?
				if b != nil {
					err := p.handlePart(part, b)
					if err != nil {
						return p, err
					}
				}
				b = &bytes.Buffer{}
				part = thing
				continue
			}
		}

		if b == nil {
			return p, fmt.Errorf("malformed test file %s", fpath)
		}
		b.Write([]byte(lin))
		if atEOF {
			break
		}
	}
	if b != nil {
		err := p.handlePart(part, b)
		if err != nil {
			return p, err
		}
	}

	return p, nil
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
			case '%':
				result.WriteString(`%`)
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

func TestPhp(t *testing.T) {
	// Set TEST_PHP_EXECUTABLE so tests like bug54514 can compare with PHP_BINARY
	if exe, err := os.Executable(); err == nil {
		os.Setenv("TEST_PHP_EXECUTABLE", exe)
	}

	// run all tests in "test"
	count := 0
	pass := 0
	skip := 0
	fail := 0
	filepath.Walk(TestsPath, func(path string, info os.FileInfo, err error) error {
		if !info.Mode().IsRegular() {
			return err
		}
		if !strings.HasSuffix(path, ".phpt") {
			return err
		}

		count += 1
		p, err := runTest(t, path)
		if err != nil {
			if err == skipTest {
				skip += 1
				return nil
			}
			fail += 1
			t.Errorf("Error in %s: %s", p.name, err.Error())
		} else {
			pass += 1
		}
		return nil
	})

	t.Logf("Total of %d tests, %d passed (%01.2f%% success), %d skipped and %d failed", count, pass, float64(pass)*100/float64(count-skip), skip, fail)
}
