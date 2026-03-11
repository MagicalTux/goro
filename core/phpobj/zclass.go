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

	parents         map[*ZClass]*ZClass // all parents, extends & implements
	Extends         *ZClass
	Implementations []*ZClass
	Const           map[phpv.ZString]phpv.Val // class constants
	Props           []*phpv.ZClassProp
	Methods         map[phpv.ZString]*phpv.ZClassMethod
	StaticProps     *phpv.ZHashTable

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
	c.parents = make(map[*ZClass]*ZClass)

	if c.ExtendsStr != "" {
		// need to lookup extend
		parent, err := ctx.Global().GetClass(ctx, c.ExtendsStr, true)
		if err != nil {
			return err
		}
		if _, found := c.parents[parent.(*ZClass)]; found {
			return ctx.Errorf("class extends loop found")
		}
		c.Extends = parent.(*ZClass)
		c.parents[parent.(*ZClass)] = parent.(*ZClass)

		// need to import methods
		for n, m := range c.Extends.Methods {
			if _, gotit := c.Methods[n]; !gotit {
				c.Methods[n] = m
			}
		}
	}
	for _, impl := range c.ImplementsStr {
		intf, err := ctx.Global().GetClass(ctx, impl, true)
		if err != nil {
			return err
		}
		c.Implementations = append(c.Implementations, intf.(*ZClass))
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
		if c.Type == phpv.ZClassTypeInterface && !m.Empty {
			// TODO: why is Loc not set here, probably missing a Tick()
			return fmt.Errorf("Interface function Template::%s() cannot contain body", string(m.Name))
		}
		if m.Modifiers.Has(phpv.ZAttrAbstract) && m.Modifiers.Has(phpv.ZAttrFinal) {
			return c.fatalError(ctx, "Cannot use the final modifier on an abstract class member")
		}
		if comp, ok := m.Method.(phpv.Compilable); ok {
			err := comp.Compile(ctx)
			if err != nil {
				return err
			}
		}
	}

	// Import abstract methods from interfaces that aren't already implemented
	for _, intf := range c.Implementations {
		for n, m := range intf.Methods {
			if _, gotit := c.Methods[n]; !gotit {
				c.Methods[n] = m
			}
		}
	}

	// Validate: non-abstract, non-interface classes must implement all abstract methods
	if c.Type != phpv.ZClassTypeInterface && c.Attr&phpv.ZClassAttr(phpv.ZClassExplicitAbstract) == 0 {
		var unimplemented []string
		for _, m := range c.Methods {
			if m.Empty && (m.Modifiers.Has(phpv.ZAttrAbstract) || m.Class != nil && m.Class != c) {
				// Find the source class name for the error message
				sourceName := c.Name
				if m.Class != nil {
					sourceName = m.Class.GetName()
				}
				unimplemented = append(unimplemented, string(sourceName)+"::"+string(m.Name))
			}
		}
		if len(unimplemented) > 0 {
			msg := fmt.Sprintf("Class %s contains %d abstract method", c.Name, len(unimplemented))
			if len(unimplemented) > 1 {
				msg += "s"
			}
			msg += " and must therefore be declared abstract or implement the remaining methods ("
			for i, u := range unimplemented {
				if i > 0 {
					msg += ", "
				}
				msg += u
			}
			msg += ")"
			return c.fatalError(ctx, msg)
		}
	}

	return nil
}

// fatalError writes a fatal error to the output buffer and returns an exit error
// so execution stops but the error message is properly formatted in PHP style.
func (c *ZClass) fatalError(ctx phpv.Context, msg string) error {
	phpErr := &phpv.PhpError{
		Err:  fmt.Errorf("%s", msg),
		Code: phpv.E_ERROR,
		Loc:  c.L,
	}
	ctx.Global().LogError(phpErr)
	return phpv.ExitError(255)
}

func (c *ZClass) InstanceOf(parentClass phpv.ZClass) bool {
	if c == nil || parentClass == nil {
		return false
	}
	if parentClass == c {
		return true
	}
	_, ok := c.parents[parentClass.(*ZClass)]
	if ok {
		return true
	}
	return false
}

func (c *ZClass) Implements(class phpv.ZClass) bool {
	for _, intf := range c.Implementations {
		if class == intf {
			return true
		}
	}
	parent := c.GetParent().(*ZClass)
	if parent != nil {
		return parent.Implements(class)
	}
	return false
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
