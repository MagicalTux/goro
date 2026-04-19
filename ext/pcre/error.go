package pcre

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/KarpelesLab/gopcre2"
	"github.com/KarpelesLab/goro/core/phpv"
)

type pcreErrKind int

const (
	pcreErrEmpty pcreErrKind = iota
	pcreErrAlphanumeric
	pcreErrNoEndDelim
	pcreErrNoEndDelimMatch
	pcreErrNulModifier
	pcreErrUnknownModifier
	pcreErrCompile
)

type pcreError struct {
	kind       pcreErrKind
	delimiter  rune
	modifier   rune
	compileErr error
}

func (e *pcreError) Error() string {
	return e.Warning("")
}

func (e *pcreError) Warning(funcName string) string {
	switch e.kind {
	case pcreErrEmpty:
		return "Empty regular expression"
	case pcreErrAlphanumeric:
		return "Delimiter must not be alphanumeric, backslash, or NUL byte"
	case pcreErrNoEndDelim:
		return fmt.Sprintf("No ending delimiter '%c' found", e.delimiter)
	case pcreErrNoEndDelimMatch:
		return fmt.Sprintf("No ending matching delimiter '%c' found", e.delimiter)
	case pcreErrNulModifier:
		return "NUL byte is not a valid modifier"
	case pcreErrUnknownModifier:
		return fmt.Sprintf("Unknown modifier '%c'", e.modifier)
	case pcreErrCompile:
		return fmt.Sprintf("Compilation failed: %s", formatCompileError(e.compileErr))
	}
	return "Unknown PCRE error"
}

// formatCompileError converts a gopcre2 CompileError to PHP's format:
// "message at offset N" instead of "gopcre2: compile error at offset N: message".
func formatCompileError(err error) string {
	if ce, ok := err.(*gopcre2.CompileError); ok {
		// PHP format: "<message> at offset <N>"
		msg := ce.Message
		// Capitalize first letter to match PHP style if needed
		if len(msg) > 0 {
			msg = strings.ToUpper(msg[:1]) + msg[1:]
		}
		return fmt.Sprintf("%s at offset %d", msg, ce.Offset)
	}
	return err.Error()
}

// PCRE error codes (PHP constants).
type pcreLastErrCode int

const (
	pcreNoError              pcreLastErrCode = 0
	pcreInternalError        pcreLastErrCode = 1
	pcreBacktrackLimitError  pcreLastErrCode = 2
	pcreRecursionLimitError  pcreLastErrCode = 3
	pcreBadUtf8Error         pcreLastErrCode = 4
	pcreBadUtf8OffsetError   pcreLastErrCode = 5
	pcreBadJitStackError     pcreLastErrCode = 6
)

// lastErrMap stores the last PCRE error per global context (one entry per PHP request).
var lastErrMap sync.Map

// setLastPCREError stores the last error code for this context.
func setLastPCREError(ctx phpv.Context, code pcreLastErrCode) {
	lastErrMap.Store(ctx.Global(), code)
}

// getLastPCREError retrieves the last error code for this context.
func getLastPCREError(ctx phpv.Context) pcreLastErrCode {
	if v, ok := lastErrMap.Load(ctx.Global()); ok {
		return v.(pcreLastErrCode)
	}
	return pcreNoError
}

// pcreErrCodeToMsg maps an error code to PHP's error message string.
func pcreErrCodeToMsg(code pcreLastErrCode) string {
	switch code {
	case pcreNoError:
		return "No error"
	case pcreInternalError:
		return "Internal error"
	case pcreBacktrackLimitError:
		return "Backtrack limit exhausted"
	case pcreRecursionLimitError:
		return "Recursion limit exhausted"
	case pcreBadUtf8Error:
		return "Malformed UTF-8 characters, possibly incorrectly encoded"
	case pcreBadUtf8OffsetError:
		return "The offset did not correspond to the beginning of a valid UTF-8 code point"
	case pcreBadJitStackError:
		return "JIT stack limit exhausted"
	}
	return "Unknown error"
}

// classifyMatchError determines the pcreLastErrCode from a gopcre2 match error.
func classifyMatchError(err error) pcreLastErrCode {
	if errors.Is(err, gopcre2.ErrMatchLimit) {
		return pcreBacktrackLimitError
	}
	if errors.Is(err, gopcre2.ErrDepthLimit) {
		return pcreRecursionLimitError
	}
	if errors.Is(err, gopcre2.ErrHeapLimit) {
		return pcreBacktrackLimitError
	}
	return pcreInternalError
}

// pregLastError returns the last PREG error code.
func pregLastError(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	code := getLastPCREError(ctx)
	return phpv.ZInt(code).ZVal(), nil
}

// pregLastErrorMsg returns the last PREG error message.
func pregLastErrorMsg(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	code := getLastPCREError(ctx)
	return phpv.ZString(pcreErrCodeToMsg(code)).ZVal(), nil
}
