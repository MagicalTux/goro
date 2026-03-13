package phpobj

import (
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

// > class Throwable
var Throwable = &ZClass{
	Name: "Throwable",
	// need abstract methods:
	// getMessage getCode getFile getLine getTrace getPrevious getTraceAsString __toString
}

// > class Error implements Throwable
var Error *ZClass

// > class TypeError extends Error
var TypeError *ZClass

// > class ValueError extends Error
var ValueError *ZClass

// > class ArgumentCountError extends TypeError
var ArgumentCountError *ZClass

// > class ArithmeticError extends Error
var ArithmeticError *ZClass

// > class DivisionByZeroError extends ArithmeticError
var DivisionByZeroError *ZClass

// > class UnhandledMatchError extends Error
var UnhandledMatchError *ZClass

// > class CompileError extends Error
var CompileError *ZClass

// > class ParseError extends CompileError
var ParseError *ZClass

// > class AssertionError extends Error
var AssertionError *ZClass

// > class ErrorException extends Exception
var ErrorException *ZClass

// > class LogicException extends Exception
var LogicException *ZClass

// > class BadFunctionCallException extends LogicException
var BadFunctionCallException *ZClass

// > class RuntimeException extends Exception
var RuntimeException *ZClass

// > class OverflowException extends RuntimeException
var OverflowException *ZClass

// > class UnexpectedValueException extends RuntimeException
var UnexpectedValueException *ZClass

// > class InvalidArgumentException extends LogicException
var InvalidArgumentException *ZClass

func init() {
	// Error hierarchy (PHP 7+)
	Error = &ZClass{
		Name:            "Error",
		Implementations: []*ZClass{Throwable},
		Props: []*phpv.ZClassProp{
			{VarName: phpv.ZString("message"), Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrProtected},
			{VarName: phpv.ZString("code"), Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrProtected},
			{VarName: phpv.ZString("file"), Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrProtected},
			{VarName: phpv.ZString("line"), Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrProtected},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: NativeMethod(exceptionConstruct)},
			"getmessage": {Name: "getMessage", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return o.HashTable().GetString("message"), nil
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
				trace := o.GetOpaque(Exception).([]*phpv.StackTraceEntry)
				return getExceptionTrace(ctx, trace).ZVal(), nil
			})},
			"gettraceasstring": {Name: "getTraceAsString", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				trace := phpv.StackTrace(o.GetOpaque(Exception).([]*phpv.StackTraceEntry))
				return trace.String().ZVal(), nil
			})},
			"__tostring": {Name: "__toString", Method: NativeMethod(exceptionToString)},
		},
	}

	TypeError = &ZClass{
		Name:    "TypeError",
		Extends: Error,
		Props:   Error.Props,
		Methods: CopyMethods(Error.Methods),
	}

	ValueError = &ZClass{
		Name:    "ValueError",
		Extends: Error,
		Props:   Error.Props,
		Methods: CopyMethods(Error.Methods),
	}

	ArgumentCountError = &ZClass{
		Name:    "ArgumentCountError",
		Extends: TypeError,
		Props:   TypeError.Props,
		Methods: CopyMethods(TypeError.Methods),
	}

	ArithmeticError = &ZClass{
		Name:    "ArithmeticError",
		Extends: Error,
		Props:   Error.Props,
		Methods: CopyMethods(Error.Methods),
	}

	DivisionByZeroError = &ZClass{
		Name:    "DivisionByZeroError",
		Extends: ArithmeticError,
		Props:   ArithmeticError.Props,
		Methods: CopyMethods(ArithmeticError.Methods),
	}

	UnhandledMatchError = &ZClass{
		Name:    "UnhandledMatchError",
		Extends: Error,
		Props:   Error.Props,
		Methods: CopyMethods(Error.Methods),
	}

	CompileError = &ZClass{
		Name:    "CompileError",
		Extends: Error,
		Props:   Error.Props,
		Methods: CopyMethods(Error.Methods),
	}

	ParseError = &ZClass{
		Name:    "ParseError",
		Extends: CompileError,
		Props:   CompileError.Props,
		Methods: CopyMethods(CompileError.Methods),
	}

	AssertionError = &ZClass{
		Name:    "AssertionError",
		Extends: Error,
		Props:   Error.Props,
		Methods: CopyMethods(Error.Methods),
	}

	// ErrorException extends Exception with severity support
	ErrorException = &ZClass{
		Name:    "ErrorException",
		Extends: Exception,
		Props: append([]*phpv.ZClassProp{
			{VarName: phpv.ZString("severity"), Default: phpv.ZInt(int64(phpv.E_ERROR)).ZVal(), Modifiers: phpv.ZAttrProtected},
		}, Exception.Props...),
		Methods: func() map[phpv.ZString]*phpv.ZClassMethod {
			m := CopyMethods(Exception.Methods)
			m["__construct"] = &phpv.ZClassMethod{Name: "__construct", Method: NativeMethod(errorExceptionConstruct)}
			m["getseverity"] = &phpv.ZClassMethod{Name: "getSeverity", Method: NativeMethod(func(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return o.HashTable().GetString("severity"), nil
			})}
			return m
		}(),
	}

	// Exception subclasses
	LogicException = &ZClass{
		Name:    "LogicException",
		Extends: Exception,
		Props:   Exception.Props,
		Methods: CopyMethods(Exception.Methods),
	}

	BadFunctionCallException = &ZClass{
		Name:    "BadFunctionCallException",
		Extends: LogicException,
		Props:   LogicException.Props,
		Methods: CopyMethods(LogicException.Methods),
	}

	InvalidArgumentException = &ZClass{
		Name:    "InvalidArgumentException",
		Extends: LogicException,
		Props:   LogicException.Props,
		Methods: CopyMethods(LogicException.Methods),
	}

	RuntimeException = &ZClass{
		Name:    "RuntimeException",
		Extends: Exception,
		Props:   Exception.Props,
		Methods: CopyMethods(Exception.Methods),
	}

	OverflowException = &ZClass{
		Name:    "OverflowException",
		Extends: RuntimeException,
		Props:   RuntimeException.Props,
		Methods: CopyMethods(RuntimeException.Methods),
	}

	UnexpectedValueException = &ZClass{
		Name:    "UnexpectedValueException",
		Extends: RuntimeException,
		Props:   RuntimeException.Props,
		Methods: CopyMethods(RuntimeException.Methods),
	}
}

// ErrorException constructor: __construct($message = "", $code = 0, $severity = E_ERROR, $filename = __FILE__, $lineno = __LINE__, $previous = null)
func errorExceptionConstruct(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// First call the parent Exception::__construct with (message, code, previous)
	parentArgs := make([]*phpv.ZVal, 0, 3)
	if len(args) > 0 {
		parentArgs = append(parentArgs, args[0]) // message
	}
	if len(args) > 1 {
		parentArgs = append(parentArgs, args[1]) // code
	} else {
		parentArgs = append(parentArgs, phpv.ZInt(0).ZVal())
	}
	if len(args) > 5 {
		parentArgs = append(parentArgs, args[5]) // previous
	}
	_, err := exceptionConstruct(ctx, o, parentArgs)
	if err != nil {
		return nil, err
	}

	// Set severity
	if len(args) > 2 {
		o.HashTable().SetString("severity", args[2])
	} else {
		o.HashTable().SetString("severity", phpv.ZInt(int64(phpv.E_ERROR)).ZVal())
	}

	// Override file/line if provided
	if len(args) > 3 && !args[3].IsNull() {
		o.HashTable().SetString("file", args[3])
	}
	if len(args) > 4 && !args[4].IsNull() {
		o.HashTable().SetString("line", args[4])
	}

	return phpv.ZNULL.ZVal(), nil
}

func CopyMethods(src map[phpv.ZString]*phpv.ZClassMethod) map[phpv.ZString]*phpv.ZClassMethod {
	dst := make(map[phpv.ZString]*phpv.ZClassMethod, len(src))
	for k, v := range src {
		// Deep copy so RegisterExt setting m.Class doesn't affect the original
		cp := *v
		dst[k] = &cp
	}
	return dst
}

// ThrowError creates an Error (or subclass) object and throws it
func ThrowError(ctx phpv.Context, class *ZClass, msg string) error {
	o, err := NewZObject(ctx, class, phpv.ZString(msg).ZVal())
	if err != nil {
		// Fallback if we can't create the error object
		return ctx.Errorf("%s", msg)
	}
	return &phperr.PhpThrow{Obj: o, Loc: ctx.Loc()}
}

func ThrowErrorAt(ctx phpv.Context, class *ZClass, msg string, loc *phpv.Loc) error {
	o, err := NewZObject(ctx, class, phpv.ZString(msg).ZVal())
	if err != nil {
		return ctx.Errorf("%s", msg)
	}
	if loc == nil {
		loc = ctx.Loc()
	}
	// Override the file/line set by the constructor to match the specified location
	if loc != nil {
		o.HashTable().SetString("file", phpv.ZString(loc.Filename).ZVal())
		o.HashTable().SetString("line", phpv.ZInt(loc.Line).ZVal())
	}
	return &phperr.PhpThrow{Obj: o, Loc: loc}
}

func ThrowObject(ctx phpv.Context, v *phpv.ZVal) error {
	o, ok := v.Value().(*ZObject)
	if !ok {
		return ctx.Errorf("Can only throw objects")
	}
	// TODO check if implements throwable or extends Exception

	err := &phperr.PhpThrow{Obj: o, Loc: ctx.Loc()}
	return err
}
