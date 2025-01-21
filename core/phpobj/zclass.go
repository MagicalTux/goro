package phpobj

import (
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

type ZClass struct {
	Name phpv.ZString
	L    *phpv.Loc
	Type phpv.ZClassType
	Attr phpv.ZClassAttr

	// string value of extend & implement (used previous to lookup)
	ExtendsStr    phpv.ZString
	ImplementsStr []phpv.ZString

	parents     map[phpv.ZClass]phpv.ZClass // all parents, extends & implements
	Extends     *ZClass
	Implements  []*ZClass
	Const       map[phpv.ZString]phpv.Val // class constants
	Props       []*phpv.ZClassProp
	Methods     map[phpv.ZString]*phpv.ZClassMethod
	StaticProps *phpv.ZHashTable

	nextIntanceID int

	// class specific handlers
	H *phpv.ZClassHandlers
}

func (c *ZClass) GetName() phpv.ZString {
	return c.Name
}

func (c *ZClass) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	err := ctx.Global().RegisterClass(c.Name, c)
	if err != nil {
		return nil, err
	}
	return nil, c.Compile(ctx)
}

func (c *ZClass) Compile(ctx phpv.Context) error {
	c.parents = make(map[phpv.ZClass]phpv.ZClass)

	if c.ExtendsStr != "" {
		// need to lookup extend
		subc, err := ctx.Global().GetClass(ctx, c.ExtendsStr, true)
		if err != nil {
			return err
		}
		if _, found := c.parents[subc]; found {
			return ctx.Errorf("class extends loop found")
		}
		c.Extends = subc.(*ZClass)
		c.parents[subc] = subc

		// need to import methods
		for n, m := range c.Extends.Methods {
			if _, gotit := c.Methods[n]; !gotit {
				c.Methods[n] = m
			}
		}
	}

	for k, v := range c.Const {
		if r, ok := v.(*phpv.CompileDelayed); ok {
			z, err := r.Run(ctx)
			if err != nil {
				return err
			}
			c.Const[k] = z.Value()
		}
	}
	for _, p := range c.Props {
		if r, ok := p.Default.(*phpv.CompileDelayed); ok {
			z, err := r.Run(ctx)
			if err != nil {
				return err
			}
			p.Default = z.Value()
		}
	}
	for _, m := range c.Methods {
		if c, ok := m.Method.(phpv.Compilable); ok {
			err := c.Compile(ctx)
			if err != nil {
				return err
			}
		}
	}
	// TODO resolve extendstr/implementsstr
	return nil
}

func (c *ZClass) InstanceOf(subc phpv.ZClass) bool {
	if c == nil || subc == nil {
		return false
	}
	if subc == c {
		return true
	}
	_, ok := c.parents[subc]
	if ok {
		return true
	}
	parent := c.GetParent()
	if parent == nil {
		return false
	}
	return parent.InstanceOf(subc)
}

func (c *ZClass) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%sclass %s {", c.Attr, c.Name)
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

func (c *ZClass) GetStaticProps(ctx phpv.Context) (*phpv.ZHashTable, error) {
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

func (c *ZClass) GetProp(name phpv.ZString) (*phpv.ZClassProp, bool) {
	for _, prop := range c.Props {
		if prop.VarName == name {
			return prop, true
		}
	}
	return nil, false
}

func (c *ZClass) GetMethod(name phpv.ZString) (*phpv.ZClassMethod, bool) {
	name = name.ToLower()
	r, ok := c.Methods[name]
	return r, ok
}

func (c *ZClass) Handlers() *phpv.ZClassHandlers {
	return c.H
}

func (c *ZClass) GetParent() phpv.ZClass {
	return c.Extends
}

func (c *ZClass) NextInstanceID() int {
	c.nextIntanceID++
	id := c.nextIntanceID
	return id
}
