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
	var buf bytes.Buffer
	var argsBuf bytes.Buffer
	level := 0
	for _, e := range st {
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
	return ZString(buf.String())
}

func GetGoDebugTrace() []byte {
	if os.Getenv("DEBUG") != "" {
		return debug.Stack()
	}
	return nil
}
