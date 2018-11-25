package core

import (
	"fmt"
	"io"
	"strings"
)

type ZClassProp struct {
	VarName   ZString
	Default   Val
	Modifiers ZObjectAttr
}

type ZClassMethod struct {
	Name      ZString
	Modifiers ZObjectAttr
	Method    Callable
}

type ZClass struct {
	Name ZString
	l    *Loc
	attr ZClassAttr

	// string value of extend & implement (used previous to lookup)
	ExtendsStr    ZString
	ImplementsStr []ZString

	Constructor *ZClassMethod

	Extends     *ZClass
	Implements  []*ZClass
	Const       map[ZString]Val // class constants
	Props       []*ZClassProp
	Methods     map[ZString]*ZClassMethod
	StaticProps *ZHashTable

	// class specific handlers
	Callable
}

func (c *ZClass) Run(ctx Context) (*ZVal, error) {
	err := ctx.Global().RegisterClass(c.Name, c)
	if err != nil {
		return nil, err
	}
	return nil, c.compile(ctx)
}

func (c *ZClass) compile(ctx Context) error {
	if c.ExtendsStr != "" {
		// need to lookup extend
		subc, err := ctx.Global().GetClass(ctx, c.ExtendsStr)
		if err != nil {
			return err
		}
		c.Extends = subc
		// TODO check recursivity

		// need to import methods
		for n, m := range c.Extends.Methods {
			if _, gotit := c.Methods[n]; !gotit {
				c.Methods[n] = m
			}
		}
	}

	for k, v := range c.Const {
		if r, ok := v.(*compileDelayed); ok {
			z, err := r.Run(ctx)
			if err != nil {
				return err
			}
			c.Const[k] = z.Value()
		}
	}
	for _, p := range c.Props {
		if r, ok := p.Default.(*compileDelayed); ok {
			z, err := r.Run(ctx)
			if err != nil {
				return err
			}
			p.Default = z.Value()
		}
	}
	for _, m := range c.Methods {
		if c, ok := m.Method.(compilable); ok {
			err := c.compile(ctx)
			if err != nil {
				return err
			}
		}
	}
	// TODO resolve extendstr/implementsstr
	return nil
}

func (c *ZClass) Loc() *Loc {
	return c.l
}

func (c *ZClass) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%sclass %s {", c.attr, c.Name)
	if err != nil {
		return err
	}
	// TODO
	_, err = fmt.Fprintf(w, "TODO }")
	return err
}

func (c *ZClass) BaseName() ZString {
	// rturn class name without namespaces/etc
	pos := strings.LastIndexByte(string(c.Name), '\\')
	if pos == -1 {
		return c.Name
	}
	return c.Name[pos+1:]
}

func (c *ZClass) getStaticProps(ctx Context) (*ZHashTable, error) {
	if c.StaticProps == nil {
		c.StaticProps = NewHashTable()
		for _, p := range c.Props {
			if !p.Modifiers.IsStatic() {
				continue
			}
			if p.Default == nil {
				c.StaticProps.SetString(p.VarName, ZNULL.ZVal())
				continue
			}
			c.StaticProps.SetString(p.VarName, p.Default.ZVal())
		}
	}
	return c.StaticProps, nil
}
