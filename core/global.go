package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

type Global struct {
	context.Context

	p     *Process
	start time.Time

	globalFuncs   map[ZString]Callable
	globalClasses map[ZString]*ZClass // TODO replace *ZClass with a nice interface
	constant      map[ZString]*ZVal

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
		constant:      make(map[ZString]*ZVal),
	}

	for k, v := range p.defaultConstants {
		res.constant[k] = v
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
	ctx := NewContext(g)
	_, err := ctx.Include(ZString(fn))

	return err
}

func (g *Global) Write(v []byte) (int, error) {
	return g.out.Write(v)
}

func (g *Global) GetGlobal() *Global {
	return g
}

func (g *Global) GetConfig(name ZString, def *ZVal) *ZVal {
	// TODO
	return def
}

func (g *Global) GetVariable(name ZString) (*ZVal, error) {
	// TODO
	return nil, nil
}

func (g *Global) SetVariable(name ZString, v *ZVal) error {
	// TODO
	return nil
}

func (g *Global) Include(name ZString) (*ZVal, error) {
	return nil, errors.New("Include can only be called within a context")
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
	if v, ok := g.constant[name]; ok {
		return v, nil
	}
	return nil, nil
}

func (g *Global) GetClass(name ZString) (*ZClass, error) {
	if c, ok := g.globalClasses[name.ToLower()]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("Class '%s' not found", name)
}

func (g *Global) RegisterClass(name ZString, c *ZClass) error {
	name = name.ToLower()
	if _, ok := g.globalClasses[name]; ok {
		return fmt.Errorf("Cannot declare class %s, because the name is already in use", name)
	}
	g.globalClasses[name] = c
	return nil
}
