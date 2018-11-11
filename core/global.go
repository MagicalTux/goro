package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type Global struct {
	context.Context

	p     *Process
	start time.Time

	globalFuncs   map[ZString]Callable
	globalClasses map[ZString]*ZClass // TODO replace *ZClass with a nice interface

	out io.Writer
}

func NewGlobal(ctx context.Context, p *Process) *Global {
	res := &Global{
		Context: ctx,
		p:       p,
		out:     os.Stdout,
		start:   time.Now(),

		globalFuncs:   make(map[ZString]Callable),
		globalClasses: make(map[ZString]*ZClass),
	}

	// import global funcs from ext
	for _, e := range globalExtMap {
		for k, v := range e.Functions {
			res.globalFuncs[ZString(k)] = v
		}
	}
	return res
}

func (g *Global) RunFile(fn string) error {
	u, err := url.Parse(fn)
	if err != nil {
		return err
	}

	f, err := g.p.Open(u)
	if err != nil {
		return err
	}

	defer f.Close()

	// grab full path of file if possible
	if fn2, ok := f.Attr("uri").(string); ok {
		fn = fn2
	}

	// tokenize
	t := tokenizer.NewLexer(f, fn)

	ctx := NewContext(g)
	// compile
	c := Compile(ctx, t)

	_, err = c.Run(ctx)
	return err
}

func (g *Global) Write(v []byte) (int, error) {
	return g.out.Write(v)
}

func (g *Global) GetGlobal() *Global {
	return g
}

func (g *Global) GetVariable(name ZString) (*ZVal, error) {
	// TODO
	return nil, nil
}

func (g *Global) SetVariable(name ZString, v *ZVal) error {
	// TODO
	return nil
}

func (g *Global) RegisterFunction(name ZString, f Callable) error {
	name = name.ToLower()
	if _, exists := g.globalFuncs[name]; exists {
		return errors.New("duplicate function name in declaration")
	}
	g.globalFuncs[name] = f
	return nil
}

func (g *Global) GetFunction(name ZString) (Callable, error) {
	if f, ok := g.globalFuncs[name.ToLower()]; ok {
		return f, nil
	}
	return nil, fmt.Errorf("Call to undefined function %s", name)
}

func (g *Global) GetConstant(name ZString) (*ZVal, error) {
	return nil, nil // TODO
}

func (g *Global) RegisterClass(name ZString, c *ZClass) error {
	name = name.ToLower()
	if _, ok := g.globalClasses[name]; ok {
		return fmt.Errorf("Cannot declare class %s, because the name is already in use", name)
	}
	g.globalClasses[name] = c
	return nil
}
