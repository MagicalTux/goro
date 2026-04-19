package compiler

import (
	"fmt"
	"io"
	"strings"

	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

// runYield represents a yield expression in a generator function.
// yield $value -- yields a value with an auto-incrementing key
// yield $key => $value -- yields with an explicit key
// yield (no value) -- yields null
type runYield struct {
	key        phpv.Runnable // nil if no explicit key
	value      phpv.Runnable // nil means yield null
	warnNotRef bool          // true when in &-generator and value is not reference-eligible
	yieldsRef  bool          // true when this is a &-generator (value should be yielded by reference)
	l          *phpv.Loc
}

// isYieldValueReferenceEligible returns true if the expression can be yielded
// by reference without a Notice. Variables, array offsets, object properties,
// and by-reference function calls are eligible; literals and regular calls are not.
func isYieldValueReferenceEligible(r phpv.Runnable) bool {
	if r == nil {
		return false
	}
	switch r.(type) {
	case *runVariable, *runVariableRef,
		*runArrayAccess,
		*runObjectVar, *runObjectDynVar,
		*runClassStaticVarRef, *runClassStaticDynVarRef,
		*runnableFunctionCallRef:
		return true
	}
	return false
}

func (r *runYield) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	if err := ctx.Tick(ctx, r.l); err != nil {
		return nil, err
	}

	var key *phpv.ZVal
	var value *phpv.ZVal

	// PHP evaluates the key expression before the value expression.
	// Evaluate key first so that object IDs and side effects are consistent with PHP.
	if r.key != nil {
		var err error
		key, err = r.key.Run(ctx)
		if err != nil {
			return nil, err
		}
	}

	if r.value != nil {
		var err error
		value, err = r.value.Run(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		value = phpv.ZNULL.ZVal()
	}

	// In a &-generator, warn if yielding a non-reference-eligible value.
	if r.warnNotRef {
		if value == nil || !value.IsRef() {
			ctx.Tick(ctx, r.l)
			ctx.Notice("Only variable references should be yielded by reference",
				logopt.NoFuncName(true))
		}
	}

	// In a &-generator with a reference-eligible value, make the value a reference
	// so that the caller can modify it through foreach &$val or $gen->current().
	if r.yieldsRef && !r.warnNotRef && value != nil {
		value.MakeRef()
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
		// Value follows directly after " => " (no extra space needed)
		if r.value != nil {
			err = r.value.Dump(w)
			if err != nil {
				return err
			}
		}
	} else if r.value != nil {
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
// yield from $iterable -- delegates to a sub-generator/iterator.
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
	if f == nil {
		// yield outside of a function is a compile error
		return nil, &phpv.PhpError{
			Err:  fmt.Errorf("The \"yield\" expression can only be used inside a function"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  l,
		}
	}
	f.isGenerator = true

	// Check return type constraints for generators.
	// Generator return type must be Generator, Iterator, Traversable, iterable, or mixed.
	if f.returnType != nil {
		if !isValidGeneratorReturnType(f.returnType) {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Generator return type must be a supertype of Generator, %s given", f.returnType.String()),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
		}
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
		// yield from is not allowed in a by-reference generator
		if f.rref {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use \"yield from\" inside a by-reference generator"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
		}
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

	// If the token is a binary-only operator (cannot start an expression),
	// yield null and leave the operator for the outer expression context.
	// PHP parses "yield * -1" as "(yield null) * (-1)" — Bug #69160.
	if isBinaryOnlyOperator(next) {
		c.backup()
		return &runYield{l: l}, nil
	}

	// yield has a value. Parse it.
	c.backup()
	value, err := compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	// In a return-by-reference generator, yield values are yielded by reference.
	// Cannot take reference of a nullsafe chain.
	if f.rref && containsNullSafe(value) {
		return nil, &phpv.PhpError{
			Err:  fmt.Errorf("Cannot take reference of a nullsafe chain"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  l,
		}
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
		// Check nullsafe in value of key=>value yield in ref generator
		if f.rref && containsNullSafe(value) {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot take reference of a nullsafe chain"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
		}
		warnNotRef := f.rref && !isYieldValueReferenceEligible(value)
		return &runYield{key: key, value: value, warnNotRef: warnNotRef, yieldsRef: f.rref, l: l}, nil
	}

	c.backup()
	warnNotRef := f.rref && !isYieldValueReferenceEligible(value)
	return &runYield{value: value, warnNotRef: warnNotRef, yieldsRef: f.rref, l: l}, nil
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
	// Pass $this so that method generators and closures can access $this.
	name := g.ZClosure.Name()
	// For generator methods, build the full qualified name including class name.
	// For anonymous classes, include the full internal name (path/line info).
	if g.ZClosure.class != nil && name != "" {
		var className string
		if zc, ok := g.ZClosure.class.(*phpobj.ZClass); ok {
			// Replace null byte to get "class@anonymous/path:line$0" format
			className = strings.Replace(string(zc.Name), "\x00", "", 1)
		} else {
			className = string(g.ZClosure.class.GetName())
		}
		name = className + "::" + name
	}
	opts := phpobj.SpawnGeneratorOptions{
		FuncName:  name,
		YieldsRef: g.ZClosure.ReturnsByRef(),
	}
	if g.ZClosure.this != nil {
		opts.This = g.ZClosure.this
	} else if ctx.This() != nil {
		opts.This = ctx.This()
	}
	if g.ZClosure.start != nil {
		opts.StartLine = int(g.ZClosure.start.Line)
		opts.StartFile = string(g.ZClosure.start.Filename)
	}
	return phpobj.SpawnGeneratorWithOptions(ctx, g.ZClosure.callBody, args, opts)
}

func (g *generatorClosure) Name() string {
	return g.ZClosure.Name()
}

func (g *generatorClosure) IsGenerator() bool {
	return true
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

// isBinaryOnlyOperator returns true if the token is a binary operator that
// cannot start an expression (i.e., cannot be used as a unary operator).
// When yield is followed by such a token, yield should yield null (PHP bug #69160).
func isBinaryOnlyOperator(item *tokenizer.Item) bool {
	// Single-char binary-only operators
	if item.IsSingle('*') || item.IsSingle('/') || item.IsSingle('%') ||
		item.IsSingle('.') {
		return true
	}
	// Multi-char binary-only operators
	switch item.Type {
	case tokenizer.T_POW,       // **
		tokenizer.T_SL,         // <<
		tokenizer.T_SR,         // >>
		tokenizer.T_IS_EQUAL,   // ==
		tokenizer.T_IS_IDENTICAL, // ===
		tokenizer.T_IS_NOT_EQUAL, // !=, <>
		tokenizer.T_IS_NOT_IDENTICAL, // !==
		tokenizer.T_IS_GREATER_OR_EQUAL, // >=
		tokenizer.T_IS_SMALLER_OR_EQUAL, // <=
		tokenizer.T_SPACESHIP,  // <=>
		tokenizer.T_BOOLEAN_AND, // &&
		tokenizer.T_BOOLEAN_OR,  // ||
		tokenizer.T_COALESCE:    // ??
		return true
	}
	return false
}

// isValidGeneratorReturnType checks if a type hint is valid as a generator return type.
// Valid types are: Generator, Iterator, Traversable, iterable, mixed, object, callable,
// or any union containing at least one of these.
func isValidGeneratorReturnType(th *phpv.TypeHint) bool {
	if th == nil {
		return true
	}
	// Union types: at least one member must be a valid generator supertype
	if len(th.Union) > 0 {
		for _, alt := range th.Union {
			if isValidGeneratorReturnType(alt) {
				return true
			}
		}
		return false
	}
	// mixed accepts anything
	if th.Type() == phpv.ZtMixed {
		return true
	}
	// Check for specific class names that are supertypes of Generator
	if th.Type() == phpv.ZtObject {
		cn := th.ClassName().ToLower()
		switch cn {
		case "generator", "iterator", "traversable", "iterable", "callable":
			return true
		case "":
			return true // bare "object" type accepts Generator
		}
	}
	return false
}
