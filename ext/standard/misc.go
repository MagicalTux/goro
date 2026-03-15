package standard

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/compiler"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// > func mixed constant ( string $name )
func constant(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name phpv.ZString
	_, err := core.Expand(ctx, args, &name)
	if err != nil {
		return nil, err
	}

	// Strip leading backslash (global namespace prefix)
	if len(name) > 0 && name[0] == '\\' {
		name = name[1:]
	}

	// Check for class constant (ClassName::CONST_NAME)
	if idx := strings.Index(string(name), "::"); idx != -1 {
		className := phpv.ZString(name[:idx])
		constName := phpv.ZString(name[idx+2:])

		class, err := ctx.Global().GetClass(ctx, className, true)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Class \"%s\" not found", className))
		}

		if zc, ok := class.(*phpobj.ZClass); ok {
			cc, ok := zc.Const[constName]
			if !ok {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Undefined constant %s::%s", className, constName))
			}

			// Check visibility
			if cc.Modifiers.IsPrivate() {
				callerClass := ctx.Class()
				if callerClass == nil || callerClass.GetName() != class.GetName() {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot access private constant %s::%s", class.GetName(), constName))
				}
			} else if cc.Modifiers.IsProtected() {
				callerClass := ctx.Class()
				if callerClass == nil || !callerClass.InstanceOf(class) && !class.InstanceOf(callerClass) {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot access protected constant %s::%s", class.GetName(), constName))
				}
			}

			// Check #[\Deprecated] attribute on the class constant
			for _, attr := range cc.Attributes {
				if attr.ClassName == "Deprecated" {
					label := "Constant"
					if zc.Type == phpv.ZClassTypeEnum {
						for _, caseName := range zc.EnumCases {
							if caseName == constName {
								label = "Enum case"
								break
							}
						}
					}
					cname := string(class.GetName()) + "::" + string(constName)
					msg := compiler.FormatDeprecatedMsg(label, cname, attr)
					if err := ctx.UserDeprecated("%s", msg, logopt.NoFuncName(true)); err != nil {
						return nil, err
					}
					break
				}
			}

			v := cc.Value
			if cd, isCD := v.(*phpv.CompileDelayed); isCD {
				resolved, err := cd.Run(ctx)
				if err != nil {
					return nil, err
				}
				cc.Value = resolved.Value()
				return resolved, nil
			}
			return v.ZVal(), nil
		}

		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Undefined constant %s::%s", className, constName))
	}

	k, ok := ctx.Global().ConstantGet(name)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Undefined constant \"%s\"", name))
	}
	return k.ZVal(), nil
}

// > func mixed eval ( string $code )
func stdFuncEval(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) != 1 {
		return nil, errors.New("eval() requires 1 argument")
	}

	// parse code in args[0]
	z, err := args[0].As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	// Build eval filename in PHP format: "file.php(line) : eval()'d code"
	evalFilename := "- : eval()'d code"
	if loc := ctx.Loc(); loc != nil {
		evalFilename = fmt.Sprintf("%s(%d) : eval()'d code", loc.Filename, loc.Line)
	}

	// tokenize
	t := tokenizer.NewLexerPhp(bytes.NewReader([]byte(z.Value().(phpv.ZString))), evalFilename)

	c, err := compiler.Compile(ctx, t)
	if err != nil {
		err = phpv.FilterExitError(err)
		if err == nil {
			return phpv.ZBool(false).ZVal(), nil
		}
		if phpErr, ok := err.(*phpv.PhpError); ok && phpErr.Code == phpv.E_PARSE {
			// PHP 8: eval() parse errors throw ParseError instead of logging
			msg := phpErr.Err.Error()
			loc := phpErr.Loc
			if loc == nil {
				loc = ctx.Loc()
			}
			return nil, phpobj.ThrowErrorAt(ctx, phpobj.ParseError, msg, loc)
		}
		return nil, err
	}

	return c.Run(ctx.Parent(1))
}

// > func mixed hrtime ([ bool $get_as_number = FALSE ] )
func stdFuncHrTime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var getAsNum *bool
	_, err := core.Expand(ctx, args, &getAsNum)
	if err != nil {
		return nil, err
	}

	// TODO find a better time source

	if getAsNum != nil && *getAsNum {
		// do get as num
		return phpv.ZInt(time.Now().UnixNano()).ZVal(), nil
	}

	t := time.Now()
	r := phpv.NewZArray()
	r.OffsetSet(ctx, nil, phpv.ZInt(t.Unix()).ZVal())
	r.OffsetSet(ctx, nil, phpv.ZInt(t.Nanosecond()).ZVal())
	return r.ZVal(), nil
}

// > func int sleep ( int $seconds )
func stdFuncSleep(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var t phpv.ZInt
	_, err := core.Expand(ctx, args, &t)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Duration(t) * time.Second)

	return phpv.ZInt(0).ZVal(), nil
}

// > func int usleep ( int $seconds )
func stdFuncUsleep(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var t phpv.ZInt
	_, err := core.Expand(ctx, args, &t)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Duration(t) * time.Microsecond)

	return nil, nil
}

// > func void die ([ string|int $status ] )
func die(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return exit(ctx, args)
}

// > func void exit ([ string|int $status ] )
func exit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var ext **phpv.ZVal
	_, err := core.Expand(ctx, args, &ext)
	if err != nil {
		return nil, err
	}

	if ext == nil {
		return nil, phpv.ExitError(0)
	}

	z := *ext

	if z.GetType() == phpv.ZtInt {
		return nil, phpv.ExitError(z.AsInt(ctx))
	}

	z, err = z.As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	ctx.Write([]byte(z.String()))
	return nil, phpv.ExitError(0)
}

// > func bool phpcredits ([ int $flag = CREDITS_ALL ] )
func fncPhpCredits(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Output a minimal credits string; PHP's real phpcredits() outputs HTML.
	ctx.Write([]byte("Goro PHP Engine\n"))
	return phpv.ZBool(true).ZVal(), nil
}

// > func void register_shutdown_function ( callable $callback [, mixed $... ]  )
func registerShutdownFunction(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var callback phpv.Callable
	_, err := core.Expand(ctx, args, &callback)
	if err != nil {
		return phpv.ZFalse.ZVal(), err
	}

	var callbackArgs []*phpv.ZVal
	for _, arg := range args[1:] {
		var cbArg *phpv.ZVal
		_, err := core.Expand(ctx, []*phpv.ZVal{arg}, &cbArg)
		if err != nil {
			return nil, err
		}
		callbackArgs = append(callbackArgs, cbArg)
	}

	fn := phpv.Bind(callback, nil, callbackArgs...)
	ctx.Global().RegisterShutdownFunction(fn)

	return nil, nil
}
