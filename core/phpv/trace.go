package phpv

import (
	"bytes"
	"fmt"
	"os"
	"runtime/debug"
)

type StackTraceEntry struct {
	FuncName     string
	BareFuncName string   // just the method/function name without class prefix
	Filename     string
	ClassName    string
	MethodType   string
	Line         int
	Args         []*ZVal
	Object       ZObject  // the $this object for instance method calls
	IsInternal   bool     // true when called from internal code (e.g., output buffer callbacks)
}

type StackTrace []*StackTraceEntry

func (st StackTrace) String() ZString {
	return st.formatInternal(true, TraceArgMaxLen, false)
}

// StringNoMain formats the stack trace without the trailing {main} entry,
// as used by debug_print_backtrace().
func (st StackTrace) StringNoMain() ZString {
	return st.formatInternal(false, TraceArgMaxLen, false)
}

// FormatWithMaxLen formats the stack trace with a custom string param max length.
func (st StackTrace) FormatWithMaxLen(maxLen int) ZString {
	return st.formatInternal(true, maxLen, false)
}

// FormatNoMainOpts formats without {main}, optionally ignoring args.
// Used by debug_print_backtrace() when DEBUG_BACKTRACE_IGNORE_ARGS is set.
func (st StackTrace) FormatNoMainOpts(ignoreArgs bool) ZString {
	return st.formatInternal(false, TraceArgMaxLen, ignoreArgs)
}

func (st StackTrace) formatInternal(includeMain bool, maxLen int, ignoreArgs bool) ZString {
	var buf bytes.Buffer
	var argsBuf bytes.Buffer
	level := 0
	for _, e := range st {
		argsBuf.Reset()
		if !ignoreArgs {
			// Include/require are language constructs; PHP omits their args
			// from debug_print_backtrace() output.
			isInclude := e.FuncName == "include" || e.FuncName == "require" ||
				e.FuncName == "include_once" || e.FuncName == "require_once"
			if !isInclude {
				for i, arg := range e.Args {
					argsBuf.WriteString(TraceArgStringMaxLen(arg, maxLen))
					if i < len(e.Args)-1 {
						argsBuf.WriteString(", ")
					}
				}
			}
		}
		var line string
		if e.IsInternal {
			line = fmt.Sprintf(
				"#%d [internal function]: %s(%s)\n",
				level,
				e.FuncName,
				argsBuf.String(),
			)
		} else {
			line = fmt.Sprintf(
				"#%d %s(%d): %s(%s)\n",
				level,
				e.Filename,
				e.Line,
				e.FuncName,
				argsBuf.String(),
			)
		}
		buf.WriteString(line)
		level++
	}
	if includeMain {
		buf.WriteString(fmt.Sprintf("#%d {main}", level))
	}
	return ZString(buf.String())
}

// TraceArgMaxLen is the default max length for string arguments in stack traces.
// It can be overridden via the zend.exception_string_param_max_len ini setting.
var TraceArgMaxLen = 15

func traceArgString(arg *ZVal) string {
	return TraceArgStringMaxLen(arg, TraceArgMaxLen)
}

// TraceArgStringMaxLen formats a single argument for display in a stack trace,
// truncating string values to maxLen characters.
func TraceArgStringMaxLen(arg *ZVal, maxLen int) string {
	if arg == nil {
		return ""
	}
	switch arg.GetType() {
	case ZtObject:
		if obj, ok := arg.Value().(ZObject); ok {
			// Enum cases: format as EnumName::CaseName instead of Object(EnumName)
			if obj.GetClass().GetType().Has(ZClassTypeEnum) {
				if nameVal := obj.HashTable().GetString("name"); nameVal != nil && nameVal.GetType() == ZtString {
					return fmt.Sprintf("%s::%s", obj.GetClass().GetName(), nameVal.String())
				}
			}
			return fmt.Sprintf("Object(%s)", obj.GetClass().GetName())
		}
		return "Object"
	case ZtString:
		s := arg.String()
		escaped := escapeTraceString(s, maxLen)
		return escaped
	case ZtNull:
		return "NULL"
	case ZtBool:
		if bool(arg.Value().(ZBool)) {
			return "true"
		}
		return "false"
	case ZtArray:
		return "Array"
	default:
		return arg.String()
	}
}

// escapeTraceString escapes non-printable and non-ASCII bytes as \xHH
// and truncates to maxLen characters, matching PHP's behavior.
func escapeTraceString(s string, maxLen int) string {
	var buf bytes.Buffer
	buf.WriteByte('\'')
	charCount := 0
	truncated := false
	for i := 0; i < len(s); i++ {
		if maxLen >= 0 && charCount >= maxLen {
			truncated = true
			break
		}
		b := s[i]
		if b < 0x20 || b > 0x7e {
			// Escape non-printable and non-ASCII bytes as \xHH
			buf.WriteString(fmt.Sprintf("\\x%02X", b))
		} else {
			buf.WriteByte(b)
		}
		charCount++
	}
	if truncated {
		buf.WriteString("...'")
	} else {
		buf.WriteByte('\'')
	}
	return buf.String()
}

func GetGoDebugTrace() []byte {
	if os.Getenv("DEBUG") != "" {
		return debug.Stack()
	}
	return nil
}
