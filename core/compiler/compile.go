package compiler

import (
	"io"
	"reflect"

	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// a helper struct to make a Runnable parentable
type runnableChild struct {
	Parent phpv.Runnable
}

func (n *runnableChild) GetParentNode() phpv.Runnable {
	return n.Parent
}
func (n *runnableChild) SetParentNode(p phpv.Runnable) {
	n.Parent = p
}

type compileCtx interface {
	phpv.Context

	ExpectSingle(r rune) error
	NextItem() (*tokenizer.Item, error)
	backup()
	getClass() *phpobj.ZClass
	getFunc() *ZClosure
	peekType() tokenizer.ItemType
}

type compileRootCtx struct {
	phpv.Context

	t *tokenizer.Lexer

	next *tokenizer.Item
	last *tokenizer.Item
}

func (c *compileRootCtx) ExpectSingle(r rune) error {
	// read one item, check if rune, if not fallback & return error
	i, err := c.NextItem()
	if err != nil {
		return err
	}

	if !i.IsSingle(r) {
		c.backup()
		return i.Unexpected()
	}
	return nil
}

func (c *compileRootCtx) getClass() *phpobj.ZClass {
	return nil
}

func (c *compileRootCtx) getFunc() *ZClosure {
	return nil
}

func (c *compileRootCtx) peekType() tokenizer.ItemType {
	if c.next != nil {
		return c.next.Type
	}

	n, err := c.NextItem()
	if err != nil {
		return -1
	}
	c.backup()
	return n.Type
}

func (c *compileRootCtx) NextItem() (*tokenizer.Item, error) {
	if c.next != nil {
		c.last, c.next = c.next, nil
		return c.last, nil
	}
	for {
		i, err := c.t.NextItem()
		if err != nil {
			return i, err
		}

		switch i.Type {
		case tokenizer.T_WHITESPACE:
		case tokenizer.T_COMMENT:
		default:
			c.last = i
			return i, err
		}
	}
}

func (c *compileRootCtx) backup() {
	c.next, c.last = c.last, nil
}

func init() {
	phpctx.Compile = Compile
}

func Compile(parent phpv.Context, t *tokenizer.Lexer) (phpv.Runnable, error) {
	c := &compileRootCtx{
		Context: parent,
		t:       t,
	}

	r, err := compileBaseUntil(nil, c, tokenizer.T_EOF)
	if err != nil && err != io.EOF {
		return nil, err
	}

	if list, ok := r.(phpv.Runnables); ok {
		// check for any function
		for i, elem := range list {
			switch obj := elem.(type) {
			case *ZClosure:
				if obj.name != "" {
					c.Global().RegisterLazyFunc(obj.name, list, i)
				}
			case *phpobj.ZClass:
				// TODO first index classes by name, instanciate in right order
				if obj.Name != "" {
					c.Global().RegisterLazyClass(obj.Name, list, i)
				}
			}
		}
	}

	// In some cases, a node needs to know if it's a write context,
	// and one way of conveying that information is with parent nodes.
	// Doing something like ctx = WriteContext(ctx) wouldn't work
	// since the surrounding context isn't known yet and expressions
	// are parsed from left to right.
	ConnectParentNodes(r)

	return r, nil
}

func ConnectParentNodes(r phpv.Runnable) {
	if r == nil {
		return
	}
	for _, c := range GetChildren(r) {
		if c == nil {
			continue
		}
		if rc, ok := c.(phpv.RunnableChild); ok {
			rc.SetParentNode(r)
		}
		ConnectParentNodes(c)
	}
}

// TODO: probably better to go-generate instead
func GetChildren(r phpv.Runnable) []phpv.Runnable {
	type rt []phpv.Runnable
	switch t := r.(type) {
	case *runRef:
		return rt{t.v}
	case *runVariable:
		return nil
	case *runVariableRef:
		return rt{t.v}
	case *runParentheses:
		return rt{t.r}
	case *runZVal:
		if x, ok := t.v.(phpv.Runnable); ok {
			return rt{x}
		}
		return nil
	case *runOperator:
		return rt{t.a, t.b}
	case *runConstant:
		return nil
	case runConcat:
		return t
	case *runnableWhile:
		return rt{t.cond, t.code}
	case *runnableDoWhile:
		return rt{t.cond, t.code}
	case *runnableIf:
		return rt{t.cond, t.yes, t.no}
	case *runnableFor:
		return rt{t.start, t.cond, t.each, t.code}
	case *runnableForeach:
		return rt{t.src, t.code, t.k, t.v}
	case *runSwitch:
		res := rt{t.cond}
		if t.def != nil {
			res = append(res, t.def.cond)
		}
		for _, e := range t.blocks {
			res = append(res, e.cond)
			res = append(res, e.code)
		}
		return res
	case *runReturn:
		return rt{t.v}
	case *runnableTry:
		res := rt{t.try, t.finally}
		for _, e := range t.catches {
			res = append(res, e.body)
		}
		return res
	case *runnableThrow:
		return rt{t.v}
	case runInlineHtml:
		return nil
	case *runStaticVar:
		res := rt{}
		for _, e := range t.vars {
			res = append(res, e.def)
		}
		return res
	case *runObjectFunc:
		res := rt{t.ref}
		for _, e := range t.args {
			res = append(res, e)
		}
		return res
	case *runObjectVar:
		return rt{t.ref}
	case *runNewObject:
		res := rt{t.cl}
		for _, e := range t.newArg {
			res = append(res, e)
		}
		return res
	case *runnableIsset:
		res := rt{}
		for _, e := range t.args {
			res = append(res, e)
		}
		return res
	case *runInstanceOf:
		return rt{t.v}
	case *runGlobal:
		return nil
	case *runnableFunctionCall:
		res := rt{}
		for _, e := range t.args {
			res = append(res, e)
		}
		return res
	case *runnableFunctionCallRef:
		res := rt{t.name}
		for _, e := range t.args {
			res = append(res, e)
		}
		return res
	case *runDestructure:
		res := rt{}
		for _, e := range t.e {
			res = append(res, e.k)
			res = append(res, e.v)
		}
		return res
	case *runnableClone:
		return rt{t.arg}
	case *runClassStaticVarRef:
		return rt{t.className}
	case *runClassStaticObjRef:
		return rt{t.className}
	case *runArray:
		res := rt{}
		for _, e := range t.e {
			res = append(res, e.k)
			res = append(res, e.v)
		}
		return res
	case *runArrayAccess:
		return []phpv.Runnable{t.offset, t.value}
	case phpv.Runnables:
		return t
	case *ZClosure:
		return rt{t.code}
	case *phpobj.ZClass:
		res := rt{}
		for _, v := range t.Methods {
			if f, ok := v.Method.(phpv.Runnable); ok {
				res = append(res, f)
			}
		}
		return res
	case *phperr.PhpBreak:
		return nil
	case *phperr.PhpContinue:
		return nil
	default:
		panic("TODO: " + reflect.TypeOf(r).String())
	}
}
