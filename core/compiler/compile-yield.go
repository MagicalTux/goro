package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// runYield represents a yield expression in a generator function.
// yield $value — yields a value with an auto-incrementing key
// yield $key => $value — yields with an explicit key
// yield (no value) — yields null
type runYield struct {
	key   phpv.Runnable // nil if no explicit key
	value phpv.Runnable // nil means yield null
	l     *phpv.Loc
}

func (r *runYield) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	if err := ctx.Tick(ctx, r.l); err != nil {
		return nil, err
	}

	var key *phpv.ZVal
	var value *phpv.ZVal

	if r.value != nil {
		var err error
		value, err = r.value.Run(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		value = phpv.ZNULL.ZVal()
	}

	if r.key != nil {
		var err error
		key, err = r.key.Run(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Call into the generator runtime to yield the value and suspend
	return phpobj.GeneratorYieldValue(ctx, key, value)
}

func (r *runYield) Dump(w io.Writer) error {
	_, err := w.Write([]byte("yield"))
	if err != nil {
		return err
	}
	if r.key != nil {
		_, err = w.Write([]byte{' '})
		if err != nil {
			return err
		}
		err = r.key.Dump(w)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(" => "))
		if err != nil {
			return err
		}
	}
	if r.value != nil {
		_, err = w.Write([]byte{' '})
		if err != nil {
			return err
		}
		err = r.value.Dump(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// runYieldFrom represents a yield from expression.
// yield from $iterable — delegates to a sub-generator/iterator.
type runYieldFrom struct {
	expr phpv.Runnable
	l    *phpv.Loc
}

func (r *runYieldFrom) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	if err := ctx.Tick(ctx, r.l); err != nil {
		return nil, err
	}

	val, err := r.expr.Run(ctx)
	if err != nil {
		return nil, err
	}

	// Delegate to the generator runtime
	return phpobj.GeneratorYieldFrom(ctx, val)
}

func (r *runYieldFrom) Dump(w io.Writer) error {
	_, err := w.Write([]byte("yield from "))
	if err != nil {
		return err
	}
	return r.expr.Dump(w)
}

// compileYield compiles a yield expression.
// Called when T_YIELD or T_YIELD_FROM is encountered.
func compileYield(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	l := i.Loc()

	// Mark the enclosing function as a generator
	f := c.getFunc()
	if f != nil {
		f.isGenerator = true
	}

	isYieldFrom := i.Type == tokenizer.T_YIELD_FROM

	// The tokenizer doesn't emit T_YIELD_FROM; it emits T_YIELD followed by
	// T_STRING "from". Detect this pattern and treat it as yield from.
	if !isYieldFrom && i.Type == tokenizer.T_YIELD {
		next, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		if next.Type == tokenizer.T_STRING && next.Data == "from" {
			isYieldFrom = true
		} else {
			c.backup()
		}
	}

	if isYieldFrom {
		// yield from <expr>
		expr, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}
		return &runYieldFrom{expr: expr, l: l}, nil
	}

	// T_YIELD

	// Peek at the next token to determine which form of yield we have
	next, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	// yield; (no value - standalone statement or in expression context returning null)
	if next.IsSingle(';') || next.IsSingle(')') || next.IsSingle(']') || next.IsSingle(',') || next.IsSingle('}') {
		c.backup()
		return &runYield{l: l}, nil
	}

	// yield has a value. Parse it.
	c.backup()
	value, err := compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	// Check if this is yield $key => $value
	next, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if next.Type == tokenizer.T_DOUBLE_ARROW {
		// yield $key => $value
		key := value
		value, err = compileExpr(nil, c)
		if err != nil {
			return nil, err
		}
		return &runYield{key: key, value: value, l: l}, nil
	}

	c.backup()
	return &runYield{value: value, l: l}, nil
}

// compileYieldExpr compiles yield as an expression (used in compileOneExpr).
// This is the same as compileYield but is called from expression context.
func compileYieldExpr(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	return compileYield(i, c)
}

// isYieldExpression returns true if the yield is used as an expression
// (e.g., $value = yield $key => $val)
func isYieldExpression(r phpv.Runnable) bool {
	switch r.(type) {
	case *runYield, *runYieldFrom:
		return true
	}
	return false
}

// containsYield recursively checks if a Runnable tree contains any yield nodes.
// This is used during compilation to determine if a function is a generator.
func containsYield(r phpv.Runnable) bool {
	if r == nil {
		return false
	}
	switch r.(type) {
	case *runYield, *runYieldFrom:
		return true
	}

	// Check children
	children := GetChildren(r)
	for _, child := range children {
		if containsYield(child) {
			return true
		}
	}
	return false
}

// wrapGeneratorClosure wraps a ZClosure's Call method to return a Generator.
// Instead of executing the function body directly, it creates a Generator object
// that will execute the body lazily when iterated.
type generatorClosure struct {
	*ZClosure
}

func (g *generatorClosure) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Spawn a generator that runs the function body in a goroutine.
	// Use callBody to bypass the generator check in ZClosure.Call.
	return phpobj.SpawnGenerator(ctx, g.ZClosure.callBody, args)
}

func (g *generatorClosure) Name() string {
	return g.ZClosure.Name()
}

func (g *generatorClosure) GetType() phpv.ZType {
	return phpv.ZtCallable
}

func (g *generatorClosure) ZVal() *phpv.ZVal {
	return phpv.NewZVal(g)
}

func (g *generatorClosure) Value() phpv.Val {
	return g
}

func (g *generatorClosure) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	switch t {
	case phpv.ZtString:
		return phpv.ZStr(fmt.Sprintf("{generator:%s}", g.Name())), nil
	}
	return phpv.CallableVal{}.AsVal(ctx, t)
}

func (g *generatorClosure) String() string {
	return "Callable"
}

func (g *generatorClosure) GetArgs() []*phpv.FuncArg {
	return g.ZClosure.GetArgs()
}

func (g *generatorClosure) GetClass() phpv.ZClass {
	return g.ZClosure.GetClass()
}

func (g *generatorClosure) Loc() *phpv.Loc {
	return g.ZClosure.Loc()
}

func (g *generatorClosure) ReturnsByRef() bool {
	return g.ZClosure.ReturnsByRef()
}
