package pcre

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpv"
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
		return fmt.Sprintf("Compilation failed: %s", e.compileErr)
	}
	return "Unknown PCRE error"
}

// preg_last_error returns PREG_NO_ERROR since Go's regexp engine
// does not have the same failure modes as PCRE.
func pregLastError(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return PREG_NO_ERROR.ZVal(), nil
}

// preg_last_error_msg returns "" (no error) for the same reason.
func pregLastErrorMsg(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZString("No error").ZVal(), nil
}
