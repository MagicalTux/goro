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
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
	_ "github.com/MagicalTux/goro/ext/bz2"
	_ "github.com/MagicalTux/goro/ext/curl"
	_ "github.com/MagicalTux/goro/ext/ctype"
	_ "github.com/MagicalTux/goro/ext/date"
	_ "github.com/MagicalTux/goro/ext/gmp"
	_ "github.com/MagicalTux/goro/ext/hash"
	_ "github.com/MagicalTux/goro/ext/json"
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
	f       *os.File
	reader  *bufio.Reader
	output  *bytes.Buffer
	name    string
	path    string
	req     *http.Request
	skipped bool // set to true when SKIPIF triggers a skip

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
	case "CREDITS", "DESCRIPTION":
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
		// Set the script filename so get_included_files() / $_SERVER['SCRIPT_FILENAME'] work correctly
		p.p.ScriptFilename = p.path
		// Set argv/argc for CLI-like behavior (PHP test runner passes the script as argv[0])
		p.p.Argv = []string{p.path}
		// Force register_argc_argv=1 so $argc/$argv are available in global scope (CLI behavior)
		p.p.Options.IniEntries["register_argc_argv"] = "1"
		g := phpctx.NewGlobalReq(p.req, p.p, ini.New())
		g.SetOutput(p.output)
		g.Chdir(phpv.ZString(path.Dir(p.path))) // chdir execution to path

		t := tokenizer.NewLexer(b, p.path)
		c, compileErr := compiler.Compile(g, t)
		if compileErr != nil {
			if phpErr, ok := compileErr.(*phpv.PhpError); ok {
				loc := phpErr.Loc
				file, line := p.path, 0
				if loc != nil {
					file, line = loc.Filename, loc.Line
				}
				switch phpErr.Code {
				case phpv.E_PARSE:
					// Handle parse errors (E_PARSE) by writing them to output as PHP would
					fmt.Fprintf(p.output, "\nParse error: %s in %s on line %d\n", phpErr.Err.Error(), file, line)
					g.Close()
					return nil
				case phpv.E_COMPILE_ERROR:
					// Handle compile errors (E_COMPILE_ERROR) as "Fatal error: ..."
					fmt.Fprintf(p.output, "\nFatal error: %s in %s on line %d\n", phpErr.Err.Error(), file, line)
					g.Close()
					return nil
				}
			}
			// If the compile returned an exit error (e.g. from LogError + ExitError
			// for E_COMPILE_ERROR that was already written to output), treat it as success.
			if phpv.FilterExitError(compileErr) == nil {
				g.Close()
				return nil
			}
			return compileErr
		}
		_, err := c.Run(g)
		// Handle uncaught exceptions and fatal errors: in PHP, these produce
		// "Fatal error: ..." output and terminate the script BEFORE shutdown
		// functions run. Write the fatal error to output first, then Close()
		// (which runs registered shutdown functions and then destructors).
		// First, give user-registered exception handlers a chance to handle it.
		if err != nil {
			err = g.HandleUncaughtException(err)
		}
		if ex, ok := err.(*phperr.PhpThrow); ok {
			// Write "Fatal error: Uncaught ..." to output (matches PHP CLI behavior)
			// For ParseError, PHP uses "Parse error: MESSAGE in FILE on line N" format.
			thrownFile := ex.ThrownFile()
			if thrownFile == "" {
				thrownFile = "Unknown"
			}
			className := ""
			if ex.Obj != nil {
				className = string(ex.Obj.GetClass().GetName())
			}
			if className == "ParseError" {
				// PHP formats ParseError as "Parse error: MESSAGE in FILE on line N"
				msg := ""
				if ex.Obj != nil {
					if m := ex.Obj.HashTable().GetString("message"); m != nil {
						msg = m.String()
					}
				}
				fmt.Fprintf(p.output, "\nParse error: %s in %s on line %d\n",
					msg, thrownFile, ex.ThrownLine())
			} else if className == "CompileError" {
				// PHP formats CompileError as "Fatal error: MESSAGE in FILE on line N"
				msg := ""
				if ex.Obj != nil {
					if m := ex.Obj.HashTable().GetString("message"); m != nil {
						msg = m.String()
					}
				}
				fmt.Fprintf(p.output, "\nFatal error: %s in %s on line %d\n",
					msg, thrownFile, ex.ThrownLine())
			} else {
				trace, replacement := ex.ErrorTrace(g)
				src := ex
				if replacement != nil {
					src = replacement
					thrownFile = src.ThrownFile()
					if thrownFile == "" {
						thrownFile = "Unknown"
					}
				}
				fmt.Fprintf(p.output, "\nFatal error: %s\n  thrown in %s on line %d\n",
					trace, thrownFile, src.ThrownLine())
			}
			g.Close()
			return nil
		}
		if phpErr, ok := err.(*phpv.PhpError); ok && (phpErr.Code == phpv.E_ERROR || phpErr.Code == phpv.E_COMPILE_ERROR) {
			// Fatal PHP errors (E_ERROR, E_COMPILE_ERROR) terminate the script.
			// For test purposes, write the error to output and return nil.
			loc := phpErr.Loc
			file, line := "[unknown]", 0
			if loc != nil {
				file, line = loc.Filename, loc.Line
			} else if l := g.Loc(); l != nil {
				file, line = l.Filename, l.Line
			}
			fmt.Fprintf(p.output, "\nFatal error: %s in %s on line %d\n", phpErr.Err.Error(), file, line)
			g.Close()
			return nil
		}
		// Also handle PhpError wrapped inside a PhpError
		if outer, ok := err.(*phpv.PhpError); ok {
			if phpErr, ok := outer.Err.(*phpv.PhpError); ok && (phpErr.Code == phpv.E_ERROR || phpErr.Code == phpv.E_COMPILE_ERROR) {
				loc := phpErr.Loc
				file, line := "[unknown]", 0
				if loc != nil {
					file, line = loc.Filename, loc.Line
				} else if l := g.Loc(); l != nil {
					file, line = l.Filename, l.Line
				}
				fmt.Fprintf(p.output, "\nFatal error: %s in %s on line %d\n", phpErr.Err.Error(), file, line)
				g.Close()
				return nil
			}
		}
		g.Close()
		return phpv.FilterExitError(err)
	case "EXPECT":
		// compare p.output with b
		out := bytes.TrimSpace(p.output.Bytes())
		exp := bytes.TrimSpace(b.Bytes())

		if bytes.Compare(out, exp) != 0 {
			// Also try with .phpt replaced by .php to handle __FILE__ differences
			// between goro's test runner (uses .phpt) and PHP's run-tests.php (uses .php)
			outNormalized := bytes.ReplaceAll(out, []byte(".phpt"), []byte(".php"))
			if bytes.Compare(outNormalized, exp) != 0 {
				return fmt.Errorf("output not as expected!\n%s", diff.LineDiff(string(exp), string(out)))
			}
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
	case "INI", "EXTENSIONS":
		// TODO: these affect test environment setup, skip for now
		return skipTest
	case "XFAIL":
		// TODO but safe to ignore
		return nil
	case "CLEAN":
		// Run cleanup PHP code after the test (ignore errors)
		g := phpctx.NewGlobal(context.Background(), p.p, ini.New())
		output := &bytes.Buffer{}
		g.SetOutput(output)
		g.Chdir(phpv.ZString(path.Dir(p.path)))
		t := tokenizer.NewLexer(b, p.path)
		c, err := compiler.Compile(g, t)
		if err != nil {
			return nil // ignore cleanup errors
		}
		c.Run(g)
		g.Close()
		return nil
	case "EXPECTF":
		// Like EXPECT but with %s, %d, %f, %r, %i, %e, %u, %c, %x placeholders
		out := bytes.TrimSpace(p.output.Bytes())
		expTemplate := strings.TrimSpace(b.String())

		// Convert EXPECTF template to a regex pattern
		// Escape the template for regex, then replace placeholders
		// Pre-process %0 (null byte) BEFORE QuoteMeta so it becomes a literal \x00
		// that can be matched against output containing null bytes.
		expTemplate = strings.ReplaceAll(expTemplate, "%0", "\x00")
		pattern := regexp.QuoteMeta(expTemplate)
		pattern = strings.ReplaceAll(pattern, `%s`, `[^\r\n]+`)
		pattern = strings.ReplaceAll(pattern, `%S`, `[^\r\n]*`)
		pattern = strings.ReplaceAll(pattern, `%i`, `[+-]?[0-9]+`)
		pattern = strings.ReplaceAll(pattern, `%d`, `[0-9]+`)
		pattern = strings.ReplaceAll(pattern, `%f`, `[+-]?\.?[0-9]+\.?[0-9]*(E[+-]?[0-9]+)?`)
		pattern = strings.ReplaceAll(pattern, `%e`, `[+-]?\.?[0-9]+\.?[0-9]*(E[+-]?[0-9]+)?`)
		pattern = strings.ReplaceAll(pattern, `%x`, `[0-9a-fA-F]+`)
		pattern = strings.ReplaceAll(pattern, `%u`, `[0-9]+`)
		pattern = strings.ReplaceAll(pattern, `%c`, `[.]`)
		pattern = strings.ReplaceAll(pattern, `%A`, `.*`)
		pattern = strings.ReplaceAll(pattern, `%w`, `[ \t\n\r\v\f]*`)
		// %r ... %r is a regex literal - handle by extracting and inserting raw regex
		// This is complex; for now, do a simple pass
		rReg := regexp.MustCompile(`%r(.+?)%r`)
		pattern = rReg.ReplaceAllStringFunc(pattern, func(m string) string {
			sub := rReg.FindStringSubmatch(m)
			if len(sub) > 1 {
				return sub[1]
			}
			return m
		})
		re, err := regexp.Compile(`(?s)^` + pattern + `$`)
		if err != nil {
			// If pattern compilation fails, fall back to exact match
			if !bytes.Equal(out, bytes.TrimSpace([]byte(expTemplate))) {
				// Try with .phpt → .php normalization
				outNormalized := bytes.ReplaceAll(out, []byte(".phpt"), []byte(".php"))
				if !bytes.Equal(outNormalized, bytes.TrimSpace([]byte(expTemplate))) {
					return fmt.Errorf("output not as expected (EXPECTF)!\n%s", diff.LineDiff(expTemplate, string(out)))
				}
			}
			return nil
		}
		if !re.Match(out) {
			// Also try with .phpt replaced by .php to handle __FILE__ differences
			outNormalized := bytes.ReplaceAll(out, []byte(".phpt"), []byte(".php"))
			if !re.Match(outNormalized) {
				return fmt.Errorf("output not as expected (EXPECTF)!\n%s", diff.LineDiff(expTemplate, string(out)))
			}
		}
		return nil
	case "EXPECTREGEX":
		// Expected is a regex
		out := bytes.TrimSpace(p.output.Bytes())
		expRegex := strings.TrimSpace(b.String())
		re, err := regexp.Compile(`(?s)` + expRegex)
		if err != nil {
			return fmt.Errorf("EXPECTREGEX compilation error: %v", err)
		}
		if !re.Match(out) {
			return fmt.Errorf("output not as expected (EXPECTREGEX)!\nPattern: %s\nOutput: %s", expRegex, string(out))
		}
		return nil
	default:
		return fmt.Errorf("unhandled part type %s for test", part)
	}
}

func runTest(fpath string) (p *phptest, err error) {
	fmt.Println("running file", fpath)
	// Convert to absolute path so __DIR__ and __FILE__ resolve correctly
	if abspath, aerr := filepath.Abs(fpath); aerr == nil {
		fpath = abspath
	}
	p = &phptest{output: &bytes.Buffer{}, name: fpath, path: fpath}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("\nfailed to run: %s\n%s", r, debug.Stack())
		} else if err == nil {
			if p.skipped {
				fmt.Println(fpath, "SKIPPED")
			} else {
				fmt.Println(fpath, "ok")
			}
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
						// Test should be skipped - stop processing
						p.skipped = true
						return p, nil
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
			p.skipped = true
			return p, nil
		}
		if err != nil {
			return p, err
		}
	}

	return p, nil
}
