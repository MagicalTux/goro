package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/MagicalTux/goro/core/compiler"
	"github.com/MagicalTux/goro/core/ini"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
	_ "github.com/MagicalTux/goro/ext/bz2"
	_ "github.com/MagicalTux/goro/ext/curl"
	_ "github.com/MagicalTux/goro/ext/ctype"
	_ "github.com/MagicalTux/goro/ext/date"
	_ "github.com/MagicalTux/goro/ext/gmp"
	_ "github.com/MagicalTux/goro/ext/hash"
	_ "github.com/MagicalTux/goro/ext/json"
	_ "github.com/MagicalTux/goro/ext/mbstring"
	_ "github.com/MagicalTux/goro/ext/mysqli"
	_ "github.com/MagicalTux/goro/ext/sqlite3"
	_ "github.com/MagicalTux/goro/ext/openssl"
	_ "github.com/MagicalTux/goro/ext/pcre"
	_ "github.com/MagicalTux/goro/ext/reflection"
	_ "github.com/MagicalTux/goro/ext/session"
	_ "github.com/MagicalTux/goro/ext/sockets"
	_ "github.com/MagicalTux/goro/ext/spl"
	_ "github.com/MagicalTux/goro/ext/standard"
	_ "github.com/MagicalTux/goro/ext/xml"
	_ "github.com/MagicalTux/goro/ext/zlib"
	"github.com/andreyvit/diff"
)

func main() {
	if len(os.Args) != 2 {
		println("need .phpt filenames to run")
		os.Exit(1)
	}
	for _, fpath := range os.Args[1:] {
		stat, err := os.Stat(fpath)
		if err != nil {
			panic(err)
		}
		if stat.IsDir() {
			files, err := os.ReadDir(fpath)
			if err != nil {
				panic(err)
			}
			for _, f := range files {
				if filepath.Ext(f.Name()) != ".phpt" {
					continue
				}
				if _, err := runTest(filepath.Join(fpath, f.Name())); err != nil {
					log.Printf("failed to run file: %s", err)
					os.Exit(1)
				}
			}
		} else {
			if _, err := runTest(fpath); err != nil {
				log.Printf("failed to run file: %s", err)
				os.Exit(1)
			}
		}
	}
}

type phptest struct {
	f      *os.File
	reader *bufio.Reader
	output *bytes.Buffer
	name   string
	path   string
	req    *http.Request
	ini    map[string]string // INI settings from --INI-- section

	p *phpctx.Process
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
	case "FILE":
		// pass data to the engine
		g := phpctx.NewGlobalReq(p.req, p.p, ini.New())
		// Apply --INI-- settings after Global init (which calls LoadDefaults)
		for k, v := range p.ini {
			g.IniConfig.SetGlobal(g, phpv.ZString(k), phpv.ZString(v).ZVal())
		}
		// Re-sync memory_limit to MemMgr if --INI-- changed it
		g.ApplyMaxMemoryLimit()
		// Re-process disable_functions if set by --INI--
		g.ReinitSuperglobals()
		// Emit the "PHP Startup: Invalid date.timezone" warning if needed
		g.ValidateDateTimezone()
		g.SetOutput(p.output)
		g.Chdir(phpv.ZString(path.Dir(p.path))) // chdir execution to path

		// PHP's run-tests.php writes the FILE section to a temporary .php
		// file. We use the .phpt path with .php extension as the "script
		// name" so error messages match the expected format. The path
		// must be absolute so __DIR__ and __FILE__ work correctly.
		scriptName := strings.TrimSuffix(p.path, ".phpt") + ".php"
		if abs, err := filepath.Abs(scriptName); err == nil {
			scriptName = abs
		}
		t := tokenizer.NewLexer(b, scriptName)
		c, compileErr := compiler.Compile(g, t)
		if compileErr != nil {
			// Handle parse/compile errors: write them to output like PHP does
			compileErr = phpv.FilterExitError(compileErr)
			if compileErr != nil {
				if ex, ok := compileErr.(*phperr.PhpThrow); ok {
					// CompileError/ParseError thrown as exception
					if ex.Obj.GetClass().InstanceOf(phpobj.ParseError) {
						msg := ex.Obj.HashTable().GetString("message").String()
						file := ex.ThrownFile()
						line := ex.ThrownLine()
						fmt.Fprintf(p.output, "\nParse error: %s in %s on line %d\n", msg, file, line)
					} else {
						trace, replacement := ex.ErrorTrace(g)
						displayEx := ex
						if replacement != nil {
							displayEx = replacement
						}
						fmt.Fprintf(p.output, "\nFatal error: %s\n  thrown in %s on line %d\n",
							trace, displayEx.ThrownFile(), displayEx.ThrownLine())
					}
				} else if phpErr, ok := compileErr.(*phpv.PhpError); ok {
					g.LogError(phpErr)
				} else {
					fmt.Fprintf(p.output, "\nFatal error: %s\n", compileErr.Error())
				}
			}
			g.Close()
			return nil
		}
		_, err := c.Run(g)
		g.Close()
		// Handle uncaught exceptions and fatal errors: in PHP, these produce
		// "Fatal error: ..." output and terminate the script. For test purposes,
		// we treat this as a successful run (the fatal error text is in the output).
		if ex, ok := err.(*phperr.PhpThrow); ok {
			// Match php-cli's format: the trace already contains the full
			// "Uncaught ...: <msg> in <file>:<line>\nStack trace:\n..." text
			// ending at "#N {main}". Just append the "thrown in" footer.
			trace, replacement := ex.ErrorTrace(g)
			displayEx := ex
			if replacement != nil {
				displayEx = replacement
			}
			fmt.Fprintf(p.output, "\nFatal error: %s\n  thrown in %s on line %d\n",
				trace, displayEx.ThrownFile(), displayEx.ThrownLine())
			return nil
		}
		if phpErr, ok := err.(*phpv.PhpError); ok && phpErr.Code == phpv.E_ERROR {
			// Fatal PHP errors (E_ERROR) terminate the script and output the error message.
			// For test purposes, write the error to output and return nil.
			loc := phpErr.Loc
			file, line := "[unknown]", 0
			if loc != nil {
				file, line = loc.Filename, loc.Line
			} else if l := g.Loc(); l != nil {
				file, line = l.Filename, l.Line
			}
			fmt.Fprintf(p.output, "\nFatal error: %s in %s on line %d\n", phpErr.Err.Error(), file, line)
			return nil
		}
		// Also handle PhpError wrapped inside a PhpError
		if outer, ok := err.(*phpv.PhpError); ok {
			if phpErr, ok := outer.Err.(*phpv.PhpError); ok && phpErr.Code == phpv.E_ERROR {
				loc := phpErr.Loc
				file, line := "[unknown]", 0
				if loc != nil {
					file, line = loc.Filename, loc.Line
				} else if l := g.Loc(); l != nil {
					file, line = l.Filename, l.Line
				}
				fmt.Fprintf(p.output, "\nFatal error: %s in %s on line %d\n", phpErr.Err.Error(), file, line)
				return nil
			}
		}
		return phpv.FilterExitError(err)
	case "EXPECT":
		// compare p.output with b (normalize \r\n → \n)
		out := bytes.ReplaceAll(bytes.TrimSpace(p.output.Bytes()), []byte("\r\n"), []byte("\n"))
		exp := bytes.ReplaceAll(bytes.TrimSpace(b.Bytes()), []byte("\r\n"), []byte("\n"))

		if !bytes.Equal(out, exp) {
			return fmt.Errorf("output not as expected!\n%s", diff.LineDiff(string(exp), string(out)))
		}
		return nil
	case "EXPECTF":
		// compare p.output with b using PHP format specifiers (normalize \r\n → \n)
		out := bytes.ReplaceAll(bytes.TrimSpace(p.output.Bytes()), []byte("\r\n"), []byte("\n"))
		exp := bytes.ReplaceAll(bytes.TrimSpace(b.Bytes()), []byte("\r\n"), []byte("\n"))

		re, err := expectfToRegex(string(exp))
		if err != nil {
			return fmt.Errorf("bad EXPECTF pattern: %w", err)
		}
		if !re.Match(out) {
			return fmt.Errorf("output not as expected!\n%s", diff.LineDiff(string(exp), string(out)))
		}
		return nil
	case "EXPECTREGEX":
		// compare p.output with b using a raw regex
		out := bytes.TrimSpace(p.output.Bytes())
		exp := bytes.TrimSpace(b.Bytes())

		re, err := regexp.Compile("(?s)\\A" + string(exp) + "\\z")
		if err != nil {
			return fmt.Errorf("bad EXPECTREGEX pattern: %w", err)
		}
		if !re.Match(out) {
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
	case "INI":
		// Parse INI settings (key=value lines, with {PWD} substitution)
		dir := filepath.Dir(p.path)
		if absDir, err := filepath.Abs(dir); err == nil {
			dir = absDir
		}
		p.ini = make(map[string]string)
		for _, line := range strings.Split(b.String(), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line[0] == ';' {
				continue
			}
			if idx := strings.IndexByte(line, '='); idx >= 0 {
				key := strings.TrimSpace(line[:idx])
				val := strings.TrimSpace(line[idx+1:])
				// Strip inline comments (;) unless value is quoted
				if !strings.HasPrefix(val, "\"") && !strings.HasPrefix(val, "'") {
					if semi := strings.IndexByte(val, ';'); semi != -1 {
						val = strings.TrimSpace(val[:semi])
					}
				}
				val = strings.ReplaceAll(val, "{PWD}", dir)
				p.ini[key] = val
			}
		}
		return nil
	case "EXTENSIONS":
		// Check that all required extensions are loaded.
		// Each line is an extension name.
		for _, line := range strings.Split(b.String(), "\n") {
			ext := strings.TrimSpace(line)
			if ext == "" {
				continue
			}
			if !phpctx.HasExt(ext) {
				return skipTest
			}
		}
		return nil
	case "CLEAN", "DESCRIPTION", "WHITESPACE_SENSITIVE", "ENV":
		// CLEAN: cleanup code (temp files) — not needed, we don't create them
		// DESCRIPTION: descriptive text — informational only
		// WHITESPACE_SENSITIVE: marker — we already do exact matching
		// ENV: set environment variables — TODO but safe to ignore for now
		return nil
	case "XFAIL":
		// TODO but safe to ignore
		return nil
	default:
		return fmt.Errorf("unhandled part type %s for test", part)
	}
}

// expectfToRegex converts PHP's EXPECTF format into a Go regexp.
// See https://qa.php.net/write-test.php for the format spec.
//
// Supported specifiers:
//   %e        directory separator
//   %s        one or more characters, except newline
//   %S        zero or more characters, except newline
//   %a        one or more characters, including newlines
//   %A        zero or more characters, including newlines
//   %w        zero or more whitespace characters
//   %i        signed integer
//   %d        unsigned integer
//   %x        one or more hex digits
//   %f        floating point
//   %c        single character
//   %%        literal %
//   %r<re>%r  inline regex pattern
func expectfToRegex(pattern string) (*regexp.Regexp, error) {
	var buf bytes.Buffer
	buf.WriteString("(?s)\\A")
	i := 0
	for i < len(pattern) {
		ch := pattern[i]
		if ch == '%' && i+1 < len(pattern) {
			// Inline regex: %r...%r
			if pattern[i+1] == 'r' {
				end := strings.Index(pattern[i+2:], "%r")
				if end >= 0 {
					buf.WriteString("(?:")
					buf.WriteString(pattern[i+2 : i+2+end])
					buf.WriteString(")")
					i += 2 + end + 2
					continue
				}
			}
			switch pattern[i+1] {
			case 'e':
				buf.WriteString(regexp.QuoteMeta(string(os.PathSeparator)))
			case 's':
				buf.WriteString("[^\\r\\n]+")
			case 'S':
				buf.WriteString("[^\\r\\n]*")
			case 'a':
				buf.WriteString(".+")
			case 'A':
				buf.WriteString(".*")
			case 'w':
				buf.WriteString("\\s*")
			case 'i':
				buf.WriteString("[+-]?\\d+")
			case 'd':
				buf.WriteString("\\d+")
			case 'x':
				buf.WriteString("[0-9a-fA-F]+")
			case 'f':
				buf.WriteString("[+-]?\\d+(?:\\.\\d+)?(?:[eE][+-]?\\d+)?")
			case 'c':
				buf.WriteString(".")
			case '%':
				buf.WriteString("%")
			default:
				buf.WriteString(regexp.QuoteMeta(pattern[i : i+2]))
			}
			i += 2
			continue
		}
		buf.WriteString(regexp.QuoteMeta(string(ch)))
		i++
	}
	buf.WriteString("\\z")
	return regexp.Compile(buf.String())
}

func runTest(fpath string) (p *phptest, err error) {
	fmt.Println("running file", fpath)
	p = &phptest{output: &bytes.Buffer{}, name: fpath, path: fpath}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("\nfailed to run: %s\n%s", r, debug.Stack())
		} else {
			fmt.Println(fpath, "ok")
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
		if err != nil {
			if err == io.EOF {
				break
			}
			return p, err
		}
		if strings.HasPrefix(lin, "--") {
			lin_trimmed := strings.TrimRight(lin, "\r\n")

			if sub := r.FindSubmatch([]byte(lin_trimmed)); sub != nil {
				thing := string(sub[1])
				// start of a new thing?
				if b != nil {
					err := p.handlePart(part, b)
					if err == skipTest {
						return p, nil // test skipped
					}
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
	}
	if b != nil {
		err := p.handlePart(part, b)
		if err == skipTest {
			return p, nil
		}
		if err != nil {
			return p, err
		}
	}

	return p, nil
}
