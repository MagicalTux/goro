package main_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MagicalTux/gophp/core"
	"github.com/MagicalTux/gophp/core/tokenizer"
	"github.com/andreyvit/diff"
)

type phptest struct {
	f      *os.File
	reader *bufio.Reader
	output *bytes.Buffer
	name   string
	path   string

	p *core.Process
	g *core.Global

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
	case "FILE":
		// pass data to the engine
		t := tokenizer.NewLexer(b, p.path)
		ctx := p.g.Root()
		c, err := core.Compile(ctx, t)
		if err != nil {
			return err
		}
		_, err = c.Run(ctx)
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
		ctx := core.NewGlobal(context.Background(), p.p)
		output := &bytes.Buffer{}
		ctx.SetOutput(output)
		c, err := core.Compile(ctx, t)
		if err != nil {
			return err
		}
		_, err = c.Run(ctx)
		if err != nil {
			if e, ok := err.(*core.PhpError); ok {
				if !e.IsExit() {
					return err
				}
			} else {
				return err
			}
		}
		if bytes.HasPrefix(output.Bytes(), []byte("skip ")) {
			return skipTest
		}
		return nil
	case "INI", "POST", "EXPECTF", "EXTENSIONS":
		// TODO
		return skipTest
	case "XFAIL":
		// TODO but safe to ignore
		return nil
	default:
		return fmt.Errorf("unhandled part type %s for test", part)
	}
}

func runTest(t *testing.T, path string) (p *phptest, err error) {
	p = &phptest{t: t, output: &bytes.Buffer{}, name: path, path: path}

	// read & parse test file
	p.f, err = os.Open(path)
	if err != nil {
		return
	}
	defer p.f.Close()
	p.reader = bufio.NewReader(p.f)

	var b *bytes.Buffer
	var part string

	// prepare env
	p.p = core.NewProcess()
	p.p.SetConstant("PHP_SAPI", "test")
	p.g = core.NewGlobal(context.Background(), p.p)
	p.g.SetOutput(p.output)
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
			return p, fmt.Errorf("malformed test file %s", path)
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
	filepath.Walk("test", func(path string, info os.FileInfo, err error) error {
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
