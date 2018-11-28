package core

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

type ZClassProp struct {
	VarName   phpv.ZString
	Default   phpv.Val
	Modifiers ZObjectAttr
}

type ZClassMethod struct {
	Name      phpv.ZString
	Modifiers ZObjectAttr
	Method    phpv.Callable
}

type ZClass struct {
	Name phpv.ZString
	l    *phpv.Loc
	Type ZClassType
	attr ZClassAttr

	// string value of extend & implement (used previous to lookup)
	ExtendsStr    phpv.ZString
	ImplementsStr []phpv.ZString

	parents     map[*ZClass]*ZClass // all parents, extends & implements
	Extends     *ZClass
	Implements  []*ZClass
	Const       map[phpv.ZString]phpv.Val // class constants
	Props       []*ZClassProp
	Methods     map[phpv.ZString]*ZClassMethod
	StaticProps *phpv.ZHashTable

	// class specific handlers
	Constructor  *ZClassMethod
	HandleInvoke func(ctx phpv.Context, o *ZObject, args []phpv.Runnable) (*phpv.ZVal, error)
}

func (c *ZClass) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	err := ctx.Global().(*Global).RegisterClass(c.Name, c)
	if err != nil {
		return nil, err
	}
	return nil, c.compile(ctx)
}

func (c *ZClass) compile(ctx phpv.Context) error {
	c.parents = make(map[*ZClass]*ZClass)

	if c.ExtendsStr != "" {
		// need to lookup extend
		subc, err := ctx.Global().(*Global).GetClass(ctx, c.ExtendsStr)
		if err != nil {
			return err
		}
		if _, found := c.parents[subc]; found {
			return errors.New("class extends loop found")
		}
		c.Extends = subc
		c.parents[subc] = subc

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

func (c *ZClass) InstanceOf(subc *ZClass) bool {
	_, r := c.parents[subc]
	return r
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

func (c *ZClass) BaseName() phpv.ZString {
	// rturn class name without namespaces/etc
	pos := strings.LastIndexByte(string(c.Name), '\\')
	if pos == -1 {
		return c.Name
	}
	return c.Name[pos+1:]
}

func (c *ZClass) getStaticProps(ctx phpv.Context) (*phpv.ZHashTable, error) {
	if c.StaticProps == nil {
		c.StaticProps = phpv.NewHashTable()
		for _, p := range c.Props {
			if !p.Modifiers.IsStatic() {
				continue
			}
			if p.Default == nil {
				c.StaticProps.SetString(p.VarName, phpv.ZNULL.ZVal())
				continue
			}
			c.StaticProps.SetString(p.VarName, p.Default.ZVal())
		}
	}
	return c.StaticProps, nil
}
