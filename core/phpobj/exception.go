package phpobj

import (
	"bytes"
	"fmt"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

// > class Exception
var Exception *ZClass

func init() {
	Exception = &ZClass{
		Name:       "Exception",
		Implements: []*ZClass{Throwable},
		Props: []*phpv.ZClassProp{
			{VarName: phpv.ZString("message"), Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrProtected},
			{VarName: phpv.ZString("string"), Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPrivate},
			{VarName: phpv.ZString("code"), Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrProtected},
			{VarName: phpv.ZString("file"), Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrProtected},
			{VarName: phpv.ZString("line"), Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrProtected},
			{VarName: phpv.ZString("trace"), Default: phpv.NewZArray().ZVal(), Modifiers: phpv.ZAttrPrivate},
			{VarName: phpv.ZString("previous"), Default: phpv.ZNULL.ZVal(), Modifiers: phpv.ZAttrPrivate},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: NativeMethod(exceptionConstruct)},

			"getmessage": {Name: "getMessage", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return o.ObjectGet(ctx, phpv.ZStr("message"))
			})},
			"getprevious": {Name: "getPrevious", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return o.ObjectGet(ctx, phpv.ZStr("previous"))
			})},
			"getcode": {Name: "getCode", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return o.ObjectGet(ctx, phpv.ZStr("code"))
			})},
			"getfile": {Name: "getFile", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return o.ObjectGet(ctx, phpv.ZStr("file"))
			})},
			"getline": {Name: "getLine", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return o.ObjectGet(ctx, phpv.ZStr("line"))
			})},
			"gettrace": {Name: "getTrace", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				trace := o.GetOpaque(Exception).([]*phpv.StackTraceEntry)
				return getExceptionTrace(ctx, trace).ZVal(), nil
			})},
			"gettraceasstring": {Name: "getTraceAsString", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				trace := o.GetOpaque(Exception).([]*phpv.StackTraceEntry)
				return getExceptionString(ctx, trace).ZVal(), nil
			})},
			"__tostring": {Name: "__toString", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				trace := o.GetOpaque(Exception).([]*phpv.StackTraceEntry)
				filename := ctx.Loc().Filename
				line := ctx.Loc().Line
				if len(trace) > 0 {
					filename = trace[0].Filename
					line = trace[0].Line
				}

				var buf bytes.Buffer
				buf.WriteString(fmt.Sprintf("Exception in %s:%d\n", filename, line))
				buf.WriteString("Stack trace:\n")
				buf.WriteString(string(getExceptionString(ctx, trace)))
				return phpv.ZStr(buf.String()), nil
			})},

			// TODO: final private __clone ( void ) : void
		},
	}
}

func SpawnException(ctx phpv.Context, l *phpv.Loc, msg phpv.ZString, code phpv.ZInt, prev *ZObject) (*ZObject, error) {
	o, err := NewZObject(ctx, Exception)
	if err != nil {
		return nil, err
	}

	if prev != nil {
		o.ObjectSet(ctx, phpv.ZString("previous").ZVal(), prev.ZVal())
	}
	return o, nil
}

func ThrowException(ctx phpv.Context, l *phpv.Loc, msg phpv.ZString, code phpv.ZInt) error {
	o, err := SpawnException(ctx, l, msg, code, nil)
	if err != nil {
		return err
	}
	return &phperr.PhpThrow{Obj: o}
}

// public __construct ([ string $message = "" [, int $code = 0 [, Throwable $previous = NULL ]]] )
func exceptionConstruct(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	switch len(args) {
	case 3:
		o.ObjectSet(ctx, phpv.ZString("previous").ZVal(), args[2])
		fallthrough
	case 2:
		o.ObjectSet(ctx, phpv.ZString("code").ZVal(), args[1])
		fallthrough
	case 1:
		o.ObjectSet(ctx, phpv.ZString("message").ZVal(), args[0])
	}

	for {
		// traverse parent contexts so that Exception
		// constructors aren't included in the trace
		if ctx.This() == nil {
			break
		}
		if !ctx.This().GetClass().InstanceOf(Exception) {
			break
		}
		parent := ctx.Parent(1)
		if parent == nil {
			break
		}
		ctx = parent
	}

	trace := ctx.GetStackTrace(ctx)
	o.SetOpaque(Exception, trace)

	return phpv.ZNULL.ZVal(), nil
}

func getExceptionTrace(ctx phpv.Context, stackTrace []*phpv.StackTraceEntry) *phpv.ZArray {
	trace := phpv.NewZArray()
	for _, e := range stackTrace {
		args := phpv.NewZArray()
		for _, arg := range e.Args {
			args.OffsetSet(ctx, nil, arg)
		}
		item := phpv.NewZArray()
		item.OffsetSet(ctx, phpv.ZStr("file"), phpv.ZStr(e.Filename))
		item.OffsetSet(ctx, phpv.ZStr("line"), phpv.ZInt(e.Line).ZVal())
		item.OffsetSet(ctx, phpv.ZStr("function"), phpv.ZStr(e.FuncName))

		if e.ClassName != "" {
			item.OffsetSet(ctx, phpv.ZStr("class"), phpv.ZStr(e.ClassName))
			item.OffsetSet(ctx, phpv.ZStr("type"), phpv.ZStr(e.MethodType))
		}

		item.OffsetSet(ctx, phpv.ZStr("args"), args.ZVal())
		trace.OffsetSet(ctx, nil, item.ZVal())
	}
	return trace
}

func getExceptionString(ctx phpv.Context, stackTrace []*phpv.StackTraceEntry) phpv.ZString {
	var buf bytes.Buffer
	var argsBuf bytes.Buffer
	level := 0
	for _, e := range stackTrace {
		argsBuf.Reset()
		for i, arg := range e.Args {
			argsBuf.WriteString(arg.String())
			if i < len(e.Args)-1 {
				argsBuf.WriteString(", ")
			}
		}
		line := fmt.Sprintf(
			"#%d %s(%d): %s(%s)\n",
			level,
			e.Filename,
			e.Line,
			e.FuncName,
			argsBuf.String(),
		)
		buf.WriteString(line)
		level++
	}
	buf.WriteString(fmt.Sprintf("#%d {main}", level))
	return phpv.ZString(buf.String())
}
