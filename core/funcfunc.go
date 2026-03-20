package core

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// Function handling Functions

// > func array func_get_args ( void )
func fncFuncGetArgs(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// no params

	// go back one context
	c, ok := ctx.Parent(1).(*phpctx.FuncContext)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "func_get_args() cannot be called from the global scope")
	}

	r := phpv.NewZArray()

	// PHP 7+: func_get_args() returns the current values of parameters
	// (after any modifications in the function body), not the original values.
	// Also, references are dereferenced (no & prefix in var_dump).
	callable := c.Callable()
	if ag, ok := callable.(phpv.FuncGetArgs); ok {
		funcArgs := ag.GetArgs()
		for i := 0; i < len(c.Args); i++ {
			if i < len(funcArgs) {
				v, err := c.OffsetGet(ctx, funcArgs[i].VarName)
				if err == nil && v != nil {
					r.OffsetSet(ctx, nil, v.Dup())
					continue
				}
			}
			r.OffsetSet(ctx, nil, c.Args[i].Dup())
		}
	} else {
		for _, v := range c.Args {
			r.OffsetSet(ctx, nil, v.Dup())
		}
	}

	return r.ZVal(), nil
}

// > func int func_num_args ( void )
func fncFuncNumArgs(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// go back one context
	c, ok := ctx.Parent(1).(*phpctx.FuncContext)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "func_num_args() must be called from a function context")
	}

	return phpv.ZInt(len(c.Args)).ZVal(), nil
}

// > func mixed func_get_arg ( int $arg_num )
func fncFuncGetArg(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var argNum phpv.ZInt
	_, err := Expand(ctx, args, &argNum)
	if err != nil {
		return nil, err
	}

	// go back one context
	c, ok := ctx.Parent(1).(*phpctx.FuncContext)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "func_get_arg() cannot be called from the global scope")
	}

	if argNum < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "func_get_arg(): Argument #1 ($position) must be greater than or equal to 0")
	}
	if argNum >= phpv.ZInt(len(c.Args)) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "func_get_arg(): Argument #1 ($position) must be less than the number of the arguments passed to the currently executed function")
	}

	// PHP 7+: func_get_arg returns the CURRENT value of the parameter
	// (after any modifications in the function body), not the original
	type argsGetter interface {
		GetArgs() []*phpv.FuncArg
	}
	callable := c.Callable()
	if ag, ok := callable.(argsGetter); ok {
		funcArgs := ag.GetArgs()
		if int(argNum) < len(funcArgs) {
			// Get current value from the function context's variables
			v, err := c.OffsetGet(ctx, funcArgs[argNum].VarName)
			if err == nil && v != nil {
				return v, nil
			}
		}
	}

	return c.Args[argNum], nil
}

// > func void debug_print_backtrace ([ int $options = 0 [, int $limit = 0 ]] )
func fncDebugPrintBacktrace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var options *phpv.ZInt
	var limit *phpv.ZInt
	_, err := Expand(ctx, args, &options, &limit)
	if err != nil {
		return nil, err
	}

	// Parse options to check for DEBUG_BACKTRACE_IGNORE_ARGS
	opts := int64(0)
	if options != nil {
		opts = int64(*options)
	}
	ignoreArgs := (opts & int64(DEBUG_BACKTRACE_IGNORE_ARGS)) != 0

	rawTrace := ctx.GetStackTrace(ctx)
	// Skip the first frame (debug_print_backtrace itself)
	if len(rawTrace) > 0 {
		rawTrace = rawTrace[1:]
	}
	trace := phpv.StackTrace(rawTrace)

	lim := 0
	if limit != nil && *limit > 0 {
		lim = int(*limit)
	}
	if lim > 0 && lim < len(trace) {
		trace = trace[:lim]
	}

	ctx.Write([]byte(trace.FormatNoMainOpts(ignoreArgs)))
	return nil, nil
}

// > func array debug_backtrace ([ int $options = DEBUG_BACKTRACE_PROVIDE_OBJECT [, int $limit = 0 ]] )
func fncDebugBacktrace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var options *phpv.ZInt
	var limit *phpv.ZInt
	_, err := Expand(ctx, args, &options, &limit)
	if err != nil {
		return nil, err
	}

	// Default options value is DEBUG_BACKTRACE_PROVIDE_OBJECT (1)
	opts := int64(DEBUG_BACKTRACE_PROVIDE_OBJECT)
	if options != nil {
		opts = int64(*options)
	}
	ignoreArgs := (opts & int64(DEBUG_BACKTRACE_IGNORE_ARGS)) != 0
	provideObject := (opts & int64(DEBUG_BACKTRACE_PROVIDE_OBJECT)) != 0

	rawTrace := ctx.GetStackTrace(ctx)
	// Skip the first frame (debug_backtrace itself)
	if len(rawTrace) > 0 {
		rawTrace = rawTrace[1:]
	}

	lim := 0
	if limit != nil && *limit > 0 {
		lim = int(*limit)
	}
	if lim > 0 && lim < len(rawTrace) {
		rawTrace = rawTrace[:lim]
	}

	result := phpv.NewZArray()
	trace := rawTrace
	for _, entry := range trace {
		frame := phpv.NewZArray()
		frame.OffsetSet(ctx, phpv.ZString("file"), phpv.ZString(entry.Filename).ZVal())
		frame.OffsetSet(ctx, phpv.ZString("line"), phpv.ZInt(entry.Line).ZVal())
		// PHP outputs "function" before "class" in backtrace arrays
		funcName := entry.BareFuncName
		if funcName == "" {
			funcName = entry.FuncName
		}
		frame.OffsetSet(ctx, phpv.ZString("function"), phpv.ZString(funcName).ZVal())
		if entry.ClassName != "" {
			frame.OffsetSet(ctx, phpv.ZString("class"), phpv.ZString(entry.ClassName).ZVal())
			// Include "object" key for instance method calls when PROVIDE_OBJECT is set
			if provideObject && entry.Object != nil && entry.MethodType == "->" {
				frame.OffsetSet(ctx, phpv.ZString("object"), entry.Object.ZVal())
			}
			if entry.MethodType != "" {
				frame.OffsetSet(ctx, phpv.ZString("type"), phpv.ZString(entry.MethodType).ZVal())
			}
		}
		if !ignoreArgs {
			argsArr := phpv.NewZArray()
			for _, a := range entry.Args {
				// Deref arguments so references don't show as &... in var_dump
				if a != nil {
					a = a.Dup()
				}
				argsArr.OffsetSet(ctx, nil, a)
			}
			frame.OffsetSet(ctx, phpv.ZString("args"), argsArr.ZVal())
		}
		result.OffsetSet(ctx, nil, frame.ZVal())
	}

	return result.ZVal(), nil
}

// > func void debug_zval_dump ( mixed $value, mixed ...$values )
func fncDebugZvalDump(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	for _, z := range args {
		doDebugZvalDump(ctx, z, "", true)
	}
	return nil, nil
}

func doDebugZvalDump(ctx phpv.Context, z *phpv.ZVal, linePfx string, topLevel bool) {
	switch z.GetType() {
	case phpv.ZtNull:
		fmt.Fprintf(ctx, "%sNULL\n", linePfx)
	case phpv.ZtBool:
		if z.Value().(phpv.ZBool) {
			fmt.Fprintf(ctx, "%sbool(true)\n", linePfx)
		} else {
			fmt.Fprintf(ctx, "%sbool(false)\n", linePfx)
		}
	case phpv.ZtInt:
		fmt.Fprintf(ctx, "%sint(%d)\n", linePfx, z.Value())
	case phpv.ZtFloat:
		p := phpv.GetSerializePrecision(ctx)
		s := phpv.FormatFloatPrecision(float64(z.Value().(phpv.ZFloat)), p)
		fmt.Fprintf(ctx, "%sfloat(%s)\n", linePfx, s)
	case phpv.ZtString:
		s := z.Value().(phpv.ZString)
		// In PHP, constant/property strings are "interned"
		fmt.Fprintf(ctx, "%sstring(%d) \"%s\" interned\n", linePfx, len(s), s)
	case phpv.ZtArray:
		c := z.Value().(phpv.ZCountable).Count(ctx)
		fmt.Fprintf(ctx, "%sarray(%d) refcount(%d){\n", linePfx, c, 2)
		localPfx := linePfx + "  "
		it := z.NewIterator()
		for it.Valid(ctx) {
			k, _ := it.Key(ctx)
			if k.GetType() == phpv.ZtInt {
				fmt.Fprintf(ctx, "%s[%s]=>\n", localPfx, k)
			} else {
				fmt.Fprintf(ctx, "%s[\"%s\"]=>\n", localPfx, k)
			}
			v, _ := it.Current(ctx)
			doDebugZvalDump(ctx, v, localPfx, false)
			it.Next(ctx)
		}
		fmt.Fprintf(ctx, "%s}\n", linePfx)
	case phpv.ZtObject:
		v := z.Value()
		if obj, ok := v.(*phpobj.ZObject); ok {
			count := obj.Count(ctx)
			// refcount(2) is typical: 1 for the variable + 1 for the function argument
			fmt.Fprintf(ctx, "%sobject(%s)#%d (%d) refcount(%d){\n", linePfx, obj.Class.GetName(), obj.ID, count, 2)
			localPfx := linePfx + "  "
			for prop := range obj.IterProps(ctx) {
				suffix := ""
				switch {
				case prop.Modifiers.IsPrivate():
					className := string(obj.GetDeclClassName(prop))
					suffix = `:"` + className + `":private`
				case prop.Modifiers.IsProtected():
					suffix = ":protected"
				}
				fmt.Fprintf(ctx, "%s[\"%s\"%s]=>\n", localPfx, prop.VarName, suffix)
				pv := obj.GetPropValue(prop)
				doDebugZvalDump(ctx, pv, localPfx, false)
			}
			fmt.Fprintf(ctx, "%s}\n", linePfx)
		}
	}
}

// > const
const (
	DEBUG_BACKTRACE_PROVIDE_OBJECT = phpv.ZInt(1)
	DEBUG_BACKTRACE_IGNORE_ARGS    = phpv.ZInt(2)
)
