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
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpctx"
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
		g.SetOutput(p.output)
		g.Chdir(phpv.ZString(path.Dir(p.path))) // chdir execution to path

		// Use .php extension for the script filename (tests expect .php, not .phpt)
		scriptPath := strings.TrimSuffix(p.path, "t")
		t := tokenizer.NewLexer(b, scriptPath)
		c, err := compiler.Compile(g, t)
		if err != nil {
			return err
		}
		var retVal *phpv.ZVal
		retVal, err = c.Run(g)
		g.Close()
		retVal, err = phperr.CatchReturn(retVal, err)
		_ = retVal
		err = phpv.FilterExitError(err)
		if err != nil {
			// Format uncaught exceptions/errors to output like PHP does
			if ex, ok := err.(*phperr.PhpThrow); ok {
				fmt.Fprintf(p.output, "\nFatal error: %s\n  thrown in %s on line %d\n", ex.ErrorTrace(g), ex.Loc.Filename, ex.Loc.Line)
				return nil
			}
			if phpErr, ok := err.(*phpv.PhpError); ok && phpErr.Code == phpv.E_ERROR {
				fmt.Fprintf(p.output, "\nFatal error: Uncaught Error: %s\n", phpErr.Error())
				return nil
			}
		}
		return err
	case "EXPECT":
		// compare p.output with b
		out := bytes.TrimSpace(p.output.Bytes())
		exp := bytes.TrimSpace(b.Bytes())

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

		// Convert EXPECTF pattern to a regex
		re := expectfToRegex(string(exp))
		matched, err := regexp.MatchString("(?s)^"+re+"$", string(out))
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
	case "INI", "EXTENSIONS":
		// TODO
		return skipTest
	case "XFAIL":
		// TODO but safe to ignore
		return nil
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
	r := regexp.MustCompile("^--([A-Z]+)--$")

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
				result.WriteString(regexp.QuoteMeta(string(pattern[i])))
				i++
				continue
			}
			i += 2
		} else {
			result.WriteString(regexp.QuoteMeta(string(pattern[i])))
			i++
		}
	}
	return result.String()
}

func TestPhp(t *testing.T) {
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
