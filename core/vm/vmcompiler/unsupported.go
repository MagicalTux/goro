package vmcompiler

import (
	"errors"
	"fmt"
)

// ErrUnsupported is returned by Compile and the emitter helpers when
// they encounter an AST node type not yet handled by the bytecode
// emitter. It is meant to be checked with errors.Is so callers can
// fall back to the AST runner.
var ErrUnsupported = errors.New("vmcompiler: unsupported AST node")

// unsupportedf wraps ErrUnsupported with a formatted detail message.
func unsupportedf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrUnsupported}, args...)...)
}
