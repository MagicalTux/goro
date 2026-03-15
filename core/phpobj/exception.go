package phpobj

import (
	"bytes"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

// > class Exception
var Exception *ZClass

func init() {
	Exception = &ZClass{
		Name:            "Exception",
		Implementations: []*ZClass{Throwable},
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
				return o.HashTable().GetString("message"), nil
			})},
			"getprevious": {Name: "getPrevious", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return o.HashTable().GetString("previous"), nil
			})},
			"getcode": {Name: "getCode", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return o.HashTable().GetString("code"), nil
			})},
			"getfile": {Name: "getFile", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return o.HashTable().GetString("file"), nil
			})},
			"getline": {Name: "getLine", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return o.HashTable().GetString("line"), nil
			})},
			"gettrace": {Name: "getTrace", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				opaque := o.GetOpaque(Exception)
				if opaque == nil {
					return phpv.NewZArray().ZVal(), nil
				}
				trace, ok := opaque.([]*phpv.StackTraceEntry)
				if !ok {
					return phpv.NewZArray().ZVal(), nil
				}
				return getExceptionTrace(ctx, trace).ZVal(), nil
			})},
			"gettraceasstring": {Name: "getTraceAsString", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				opaque := o.GetOpaque(Exception)
				if opaque == nil {
					return phpv.ZString("").ZVal(), nil
				}
				trace, ok := opaque.([]*phpv.StackTraceEntry)
				if !ok {
					return phpv.ZString("").ZVal(), nil
				}
				return phpv.StackTrace(trace).String().ZVal(), nil
			})},
			"__tostring": {Name: "__toString", Method: NativeMethod(exceptionToString)},

			// TODO: final private __clone ( void ) : void
		},
	}
}

func exceptionToString(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var trace []*phpv.StackTraceEntry
	if opaque := o.GetOpaque(Exception); opaque != nil {
		trace, _ = opaque.([]*phpv.StackTraceEntry)
	}
	className := o.GetClass().GetName()
	// Access properties directly to bypass visibility checks (we're internal)
	message := o.HashTable().GetString("message")
	file := o.HashTable().GetString("file")
	line := o.HashTable().GetString("line")

	var buf bytes.Buffer
	msg := message.String()
	if msg != "" {
		buf.WriteString(string(className))
		buf.WriteString(": ")
		buf.WriteString(msg)
		buf.WriteString(" in ")
		buf.WriteString(file.String())
		buf.WriteString(":")
		buf.WriteString(line.String())
		buf.WriteString("\n")
	} else {
		buf.WriteString(string(className))
		buf.WriteString(" in ")
		buf.WriteString(file.String())
		buf.WriteString(":")
		buf.WriteString(line.String())
		buf.WriteString("\n")
	}
	buf.WriteString("Stack trace:\n")
	buf.WriteString(string(phpv.StackTrace(trace).String()))
	return phpv.ZStr(buf.String()), nil
}

func SpawnException(ctx phpv.Context, l *phpv.Loc, msg phpv.ZString, code phpv.ZInt, prev *ZObject) (*ZObject, error) {
	o, err := NewZObject(ctx, Exception)
	if err != nil {
		return nil, err
	}

	if prev != nil {
		o.HashTable().SetString("previous", prev.ZVal())
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
	// Use direct hash table access to bypass visibility checks (internal method)
	switch len(args) {
	case 3:
		o.HashTable().SetString("previous", args[2])
		fallthrough
	case 2:
		o.HashTable().SetString("code", args[1])
		fallthrough
	case 1:
		o.HashTable().SetString("message", args[0])
	}

	for {
		// traverse parent contexts so that Exception/Error
		// constructors aren't included in the trace
		if ctx.This() == nil {
			break
		}
		if !ctx.This().GetClass().InstanceOf(Exception) && !ctx.This().GetClass().InstanceOf(Error) {
			break
		}
		parent := ctx.Parent(1)
		if parent == nil {
			break
		}
		ctx = parent
	}

	// Set file and line to the location where the exception was created
	loc := ctx.Loc()
	if loc != nil {
		o.HashTable().SetString("file", phpv.ZString(loc.Filename).ZVal())
		o.HashTable().SetString("line", phpv.ZInt(loc.Line).ZVal())
	}

	trace := ctx.GetStackTrace(ctx)
	o.SetOpaque(Exception, trace)
	// Also store under the actual class so ErrorTrace can find it
	// (Error doesn't extend Exception, so walking the hierarchy won't find it)
	if o.GetClass() != Exception {
		o.SetOpaque(o.GetClass(), trace)
	}

	return phpv.ZNULL.ZVal(), nil
}

func getExceptionTrace(ctx phpv.Context, stackTrace phpv.StackTrace) *phpv.ZArray {
	trace := phpv.NewZArray()
	for _, e := range stackTrace {
		args := phpv.NewZArray()
		for _, arg := range e.Args {
			args.OffsetSet(ctx, nil, arg)
		}
		item := phpv.NewZArray()
		item.OffsetSet(ctx, phpv.ZStr("file"), phpv.ZStr(e.Filename))
		item.OffsetSet(ctx, phpv.ZStr("line"), phpv.ZInt(e.Line).ZVal())
		// Use bare function name (without class prefix) for the "function" key
		funcName := e.BareFuncName
		if funcName == "" {
			funcName = e.FuncName
		}
		item.OffsetSet(ctx, phpv.ZStr("function"), phpv.ZStr(funcName))

		if e.ClassName != "" {
			item.OffsetSet(ctx, phpv.ZStr("class"), phpv.ZStr(e.ClassName))
			item.OffsetSet(ctx, phpv.ZStr("type"), phpv.ZStr(e.MethodType))
		}

		item.OffsetSet(ctx, phpv.ZStr("args"), args.ZVal())
		trace.OffsetSet(ctx, nil, item.ZVal())
	}
	return trace
}
