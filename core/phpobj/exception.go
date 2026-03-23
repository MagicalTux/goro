package phpobj

import (
	"bytes"
	"fmt"
	"strings"

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
			"gettraceasstring": {Name: "getTraceAsString", Method: NativeMethod(exceptionGetTraceAsString)},
			"__tostring": {Name: "__toString", Method: NativeMethod(exceptionToString)},

			// TODO: final private __clone ( void ) : void
		},
	}
}

// getExceptionStringParamMaxLen reads the zend.exception_string_param_max_len
// ini setting from the context, falling back to the default (15).
func getExceptionStringParamMaxLen(ctx phpv.Context) int {
	if ctx == nil {
		return phpv.TraceArgMaxLen
	}
	val := ctx.GetConfig("zend.exception_string_param_max_len", phpv.ZInt(int64(phpv.TraceArgMaxLen)).ZVal())
	if val == nil {
		return phpv.TraceArgMaxLen
	}
	n := val.AsInt(ctx)
	if n < 0 {
		return phpv.TraceArgMaxLen
	}
	return int(n)
}

// exceptionEntryToString formats a single exception entry (without the previous chain).
// Returns the formatted string and an error if the message could not be converted to string.
func exceptionEntryToString(ctx phpv.Context, o *ZObject, maxLen int) (string, error) {
	var trace []*phpv.StackTraceEntry
	if opaque := o.GetOpaque(Exception); opaque != nil {
		trace, _ = opaque.([]*phpv.StackTraceEntry)
	}
	className := o.GetClass().GetName()
	messageVal := o.HashTable().GetString("message")
	file := o.HashTable().GetString("file")
	line := o.HashTable().GetString("line")

	// Get the file string - for objects with __toString, call it via context
	var fileStr string
	if file != nil {
		if file.GetType() == phpv.ZtObject && ctx != nil {
			fileStr = string(file.AsString(ctx))
		} else {
			fileStr = file.String()
		}
	}

	// Get the line string
	var lineStr string
	if line != nil {
		lineStr = line.String()
	}

	// Get the message string - for objects, call __toString via context
	var msg string
	if messageVal != nil {
		if messageVal.GetType() == phpv.ZtObject && ctx != nil {
			// Object message - try to call __toString to get string representation
			r, err := messageVal.As(ctx, phpv.ZtString)
			if err != nil {
				// The message object could not be converted to string
				return "", err
			}
			msg = r.String()
		} else {
			msg = messageVal.String()
		}
	}

	var buf bytes.Buffer
	// PHP uses "and defined in" when message contains "called in" (e.g. type errors)
	locPrefix := " in "
	if strings.Contains(msg, "called in") {
		locPrefix = " and defined in "
	}
	if msg != "" {
		buf.WriteString(string(className))
		buf.WriteString(": ")
		buf.WriteString(msg)
		buf.WriteString(locPrefix)
		buf.WriteString(fileStr)
		buf.WriteString(":")
		buf.WriteString(lineStr)
		buf.WriteString("\n")
	} else {
		buf.WriteString(string(className))
		buf.WriteString(" in ")
		buf.WriteString(fileStr)
		buf.WriteString(":")
		buf.WriteString(lineStr)
		buf.WriteString("\n")
	}
	buf.WriteString("Stack trace:\n")
	buf.WriteString(string(phpv.StackTrace(trace).FormatWithMaxLen(maxLen)))
	return buf.String(), nil
}

func exceptionToString(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	maxLen := getExceptionStringParamMaxLen(ctx)

	// Collect the chain from innermost to outermost (current)
	var chain []*ZObject
	seen := make(map[*ZObject]bool)
	current := o
	for current != nil {
		if seen[current] {
			break // prevent infinite loops
		}
		seen[current] = true
		chain = append(chain, current)
		prev := current.HashTable().GetString("previous")
		if prev == nil || prev.GetType() == phpv.ZtNull {
			break
		}
		if prevObj, ok := prev.Value().(*ZObject); ok {
			current = prevObj
		} else {
			break
		}
	}

	var buf bytes.Buffer
	// Output from innermost (last in chain) to outermost (first = current exception)
	for i := len(chain) - 1; i >= 0; i-- {
		if i < len(chain)-1 {
			buf.WriteString("\n\nNext ")
		}
		s, err := exceptionEntryToString(ctx, chain[i], maxLen)
		if err != nil {
			return nil, err
		}
		buf.WriteString(s)
	}
	result := phpv.ZStr(buf.String())
	// Cache the result in the 'string' private property (PHP behavior).
	// Use the mangled private property key to avoid creating a duplicate entry.
	mangledKey := phpv.ZString("*" + string(Exception.GetName()) + ":string")
	o.HashTable().SetString(mangledKey, result)
	return result, nil
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

// exceptionConstructorClassName returns the name of the base class that
// defines __construct for error messages. In PHP, TypeError messages say
// "Exception::__construct()" even when called on a subclass like "Hello".
// For ErrorException subclasses, it returns "ErrorException" since ErrorException
// defines its own __construct.
func exceptionConstructorClassName(o *ZObject) phpv.ZString {
	// Walk the class hierarchy to find which class defines __construct
	if zc, ok := o.GetClass().(*ZClass); ok {
		// Check ErrorException first (it overrides Exception::__construct)
		if zc.InstanceOf(ErrorException) {
			return ErrorException.GetName()
		}
	}
	if o.GetClass().InstanceOf(Exception) {
		return Exception.GetName()
	}
	if o.GetClass().InstanceOf(Error) {
		return Error.GetName()
	}
	return o.GetClass().GetName()
}

// public __construct ([ string $message = "" [, int $code = 0 [, Throwable $previous = NULL ]]] )
func exceptionConstruct(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Determine base class name for error messages
	baseClassName := exceptionConstructorClassName(o)

	// Validate and set arguments.
	// Use ObjectSet for property writes so that subclass property hooks
	// (PHP 8.4) are triggered, matching PHP behavior.
	if len(args) >= 1 && args[0] != nil && !args[0].IsNull() {
		// $message must be string - PHP uses coercion mode for internal functions,
		// so scalar types (int, float, bool) are accepted and coerced to string.
		// Only objects and arrays cause a TypeError.
		switch args[0].GetType() {
		case phpv.ZtString:
			if err := o.ObjectSet(ctx, phpv.ZString("message"), args[0]); err != nil {
				return nil, err
			}
		case phpv.ZtInt, phpv.ZtFloat, phpv.ZtBool:
			// Coerce scalar to string
			msgVal, err := args[0].As(ctx, phpv.ZtString)
			if err != nil {
				return nil, err
			}
			if err := o.ObjectSet(ctx, phpv.ZString("message"), msgVal); err != nil {
				return nil, err
			}
		default:
			return nil, ThrowError(ctx, TypeError,
				fmt.Sprintf("%s::__construct(): Argument #1 ($message) must be of type string, %s given",
					baseClassName, phpv.ZValTypeName(args[0])))
		}
	}
	if len(args) >= 2 && args[1] != nil && !args[1].IsNull() {
		// $code must be int - also uses coercion mode
		switch args[1].GetType() {
		case phpv.ZtInt:
			if err := o.ObjectSet(ctx, phpv.ZString("code"), args[1]); err != nil {
				return nil, err
			}
		case phpv.ZtFloat, phpv.ZtString, phpv.ZtBool:
			codeVal, err := args[1].As(ctx, phpv.ZtInt)
			if err != nil {
				return nil, ThrowError(ctx, TypeError,
					fmt.Sprintf("%s::__construct(): Argument #2 ($code) must be of type int, %s given",
						baseClassName, phpv.ZValTypeName(args[1])))
			}
			if err := o.ObjectSet(ctx, phpv.ZString("code"), codeVal); err != nil {
				return nil, err
			}
		default:
			return nil, ThrowError(ctx, TypeError,
				fmt.Sprintf("%s::__construct(): Argument #2 ($code) must be of type int, %s given",
					baseClassName, phpv.ZValTypeName(args[1])))
		}
	}
	if len(args) >= 3 && args[2] != nil && !args[2].IsNull() {
		// $previous must be ?Throwable
		if args[2].GetType() != phpv.ZtObject {
			return nil, ThrowError(ctx, TypeError,
				fmt.Sprintf("%s::__construct(): Argument #3 ($previous) must be of type ?Throwable, %s given",
					baseClassName, phpv.ZValTypeName(args[2])))
		}
		prevObj, ok := args[2].Value().(*ZObject)
		if !ok || (!prevObj.GetClass().InstanceOf(Exception) && !prevObj.GetClass().InstanceOf(Error) && !prevObj.GetClass().Implements(Throwable)) {
			return nil, ThrowError(ctx, TypeError,
				fmt.Sprintf("%s::__construct(): Argument #3 ($previous) must be of type ?Throwable, %s given",
					baseClassName, phpv.ZValTypeName(args[2])))
		}
		// Use direct hash table write for $previous (private property).
		// Private properties can't have hooks in subclasses.
		o.HashTable().SetString("previous", prevObj.ZVal())
	}

	for {
		// traverse parent contexts so that the current object's own
		// constructor is not included in the trace. Only skip contexts
		// where $this is the exact same object being constructed.
		if ctx.This() != o {
			break
		}
		parent := ctx.Parent(1)
		if parent == nil {
			break
		}
		ctx = parent
	}

	// Set file and line to the location where the exception was created.
	// Use direct hash table writes for file/line since these are internal
	// properties that should not trigger hooks (PHP behavior).
	loc := ctx.Loc()
	if loc != nil {
		o.HashTable().SetString("file", phpv.ZString(loc.Filename).ZVal())
		o.HashTable().SetString("line", phpv.ZInt(loc.Line).ZVal())
	}

	trace := ctx.GetStackTrace(ctx)

	// When zend.exception_ignore_args is enabled, strip args from the trace
	if ignoreVal := ctx.GetConfig("zend.exception_ignore_args", phpv.ZBool(false).ZVal()); ignoreVal != nil {
		if ignoreVal.AsBool(ctx) {
			for _, entry := range trace {
				entry.Args = nil
			}
		}
	}

	o.SetOpaque(Exception, trace)
	// Also store under the actual class so ErrorTrace can find it
	// (Error doesn't extend Exception, so walking the hierarchy won't find it)
	if o.GetClass() != Exception {
		o.SetOpaque(o.GetClass(), trace)
	}

	// Populate the trace property in the hash table as an array, matching PHP
	// behaviour. This allows user code (e.g. ReflectionProperty::setValue) to
	// modify the trace and have getTraceAsString() / __toString() see the
	// modifications.
	traceArr := getExceptionTrace(ctx, trace)
	o.HashTable().SetString("trace", traceArr.ZVal())

	return phpv.ZNULL.ZVal(), nil
}

func exceptionGetTraceAsString(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	maxLen := getExceptionStringParamMaxLen(ctx)

	// Check if the trace property was modified by user code (e.g. via
	// ReflectionProperty::setValue). In that case, read from the hash table
	// array representation rather than the opaque structured trace.
	traceVal := o.HashTable().GetString("trace")
	if traceVal != nil && traceVal.GetType() == phpv.ZtArray {
		traceArr := traceVal.Value().(*phpv.ZArray)
		return getTraceAsStringFromArray(ctx, traceArr, maxLen).ZVal(), nil
	}

	opaque := o.GetOpaque(Exception)
	if opaque == nil {
		return phpv.ZString("#0 {main}").ZVal(), nil
	}
	trace, ok := opaque.([]*phpv.StackTraceEntry)
	if !ok {
		return phpv.ZString("#0 {main}").ZVal(), nil
	}
	return phpv.StackTrace(trace).FormatWithMaxLen(maxLen).ZVal(), nil
}

// getTraceAsStringFromArray formats a trace array (from the hash table) as a string.
// This matches PHP's behavior of reading from the trace property which may have
// been modified by user code (e.g., via ReflectionProperty::setValue).
func getTraceAsStringFromArray(ctx phpv.Context, traceArr *phpv.ZArray, maxLen int) phpv.ZString {
	var buf bytes.Buffer
	level := 0
	frameIdx := 0
	for _, frame := range traceArr.Iterate(ctx) {
		// Each frame should be an array
		if frame == nil || frame.GetType() != phpv.ZtArray {
			ctx.Warn("Expected array for frame %d", frameIdx)
			frameIdx++
			continue
		}
		frameIdx++
		frameArr := frame.Value().(*phpv.ZArray)

		// Read frame fields with validation
		fileVal, _ := frameArr.OffsetGet(ctx, phpv.ZString("file"))
		lineVal, _ := frameArr.OffsetGet(ctx, phpv.ZString("line"))
		funcVal, _ := frameArr.OffsetGet(ctx, phpv.ZString("function"))
		classVal, _ := frameArr.OffsetGet(ctx, phpv.ZString("class"))
		typeVal, _ := frameArr.OffsetGet(ctx, phpv.ZString("type"))
		argsVal, _ := frameArr.OffsetGet(ctx, phpv.ZString("args"))

		// Validate and extract values
		file := ""
		if fileVal != nil && fileVal.GetType() != phpv.ZtNull {
			if fileVal.GetType() != phpv.ZtString {
				ctx.Warn("File name is not a string")
				file = "[unknown file]"
			} else {
				file = fileVal.String()
			}
		}

		lineNum := 0
		if lineVal != nil && lineVal.GetType() != phpv.ZtNull {
			lineNum = int(lineVal.AsInt(ctx))
		}

		funcName := ""
		if funcVal != nil && funcVal.GetType() != phpv.ZtNull {
			if funcVal.GetType() != phpv.ZtString {
				ctx.Warn("Value for function is not a string")
				funcName = "[unknown]"
			} else {
				funcName = funcVal.String()
			}
		}

		className := ""
		if classVal != nil && classVal.GetType() != phpv.ZtNull {
			if classVal.GetType() != phpv.ZtString {
				ctx.Warn("Value for class is not a string")
				className = "[unknown]"
			} else {
				className = classVal.String()
			}
		}

		methodType := ""
		if typeVal != nil && typeVal.GetType() != phpv.ZtNull {
			if typeVal.GetType() != phpv.ZtString {
				ctx.Warn("Value for type is not a string")
				methodType = "[unknown]"
			} else {
				methodType = typeVal.String()
			}
		}

		// Format args
		argsStr := ""
		if argsVal != nil && argsVal.GetType() != phpv.ZtNull {
			if argsVal.GetType() != phpv.ZtArray {
				ctx.Warn("args element is not an array")
			} else {
				argsArr := argsVal.Value().(*phpv.ZArray)
				var argsBuf bytes.Buffer
				first := true
				for _, arg := range argsArr.Iterate(ctx) {
					if !first {
						argsBuf.WriteString(", ")
					}
					first = false
					argsBuf.WriteString(phpv.TraceArgStringMaxLen(arg, maxLen))
				}
				argsStr = argsBuf.String()
			}
		}

		// Build the full function name
		fullFunc := funcName
		if className != "" {
			fullFunc = className + methodType + funcName
		}

		// Format the line
		if file == "" {
			buf.WriteString(fmt.Sprintf("#%d [internal function]: %s(%s)\n", level, fullFunc, argsStr))
		} else {
			buf.WriteString(fmt.Sprintf("#%d %s(%d): %s(%s)\n", level, file, lineNum, fullFunc, argsStr))
		}
		level++
	}
	buf.WriteString(fmt.Sprintf("#%d {main}", level))
	return phpv.ZString(buf.String())
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
