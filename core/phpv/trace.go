package phpv

import (
	"bytes"
	"fmt"
	"os"
	"runtime/debug"
)

type StackTraceEntry struct {
	FuncName   string
	Filename   string
	ClassName  string
	MethodType string
	Line       int
	Args       []*ZVal
}

type StackTrace []*StackTraceEntry

func (st StackTrace) String() ZString {
	return st.format(true)
}

// StringNoMain formats the stack trace without the trailing {main} entry,
// as used by debug_print_backtrace().
func (st StackTrace) StringNoMain() ZString {
	return st.format(false)
}

func (st StackTrace) format(includeMain bool) ZString {
	var buf bytes.Buffer
	var argsBuf bytes.Buffer
	level := 0
	for _, e := range st {
		argsBuf.Reset()
		for i, arg := range e.Args {
			argsBuf.WriteString(traceArgString(arg))
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
	if includeMain {
		buf.WriteString(fmt.Sprintf("#%d {main}", level))
	}
	return ZString(buf.String())
}

func traceArgString(arg *ZVal) string {
	if arg == nil {
		return ""
	}
	switch arg.GetType() {
	case ZtObject:
		if obj, ok := arg.Value().(ZObject); ok {
			return fmt.Sprintf("Object(%s)", obj.GetClass().GetName())
		}
		return "Object"
	case ZtString:
		s := arg.String()
		if len(s) > 15 {
			return "'" + s[:15] + "...'"
		}
		return "'" + s + "'"
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

func GetGoDebugTrace() []byte {
	if os.Getenv("DEBUG") != "" {
		return debug.Stack()
	}
	return nil
}
