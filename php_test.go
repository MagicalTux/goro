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

	"github.com/MagicalTux/goro/core"
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

	p *core.Process

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
	case "FILE":
		// pass data to the engine
		g := core.NewGlobalReq(p.req, p.p)
		g.SetOutput(p.output)
		g.Chdir(core.ZString(path.Dir(p.path))) // chdir execution to path

		t := tokenizer.NewLexer(b, p.path)
		c, err := core.Compile(g, t)
		if err != nil {
			return err
		}
		_, err = c.Run(g)
		g.Close()
		return core.FilterError(err)
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
		g := core.NewGlobal(context.Background(), p.p)
		output := &bytes.Buffer{}
		g.SetOutput(output)
		c, err := core.Compile(g, t)
		if err != nil {
			return err
		}
		_, err = c.Run(g)
		err = core.FilterError(err)
		if err != nil {
			return err
		}
		if bytes.HasPrefix(output.Bytes(), []byte("skip ")) {
			return skipTest
		}
		return nil
	case "INI", "EXPECTF", "EXTENSIONS":
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
	p.p = core.NewProcess("test")
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
		if err != nil {
			return p, err
		}
	}

	return p, nil
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
