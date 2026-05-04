// Package vmcompiler walks a compiled AST tree (phpv.Runnable) and
// emits a *vm.Function that the VM can execute.
//
// On any node type the emitter doesn't yet understand, Compile returns
// a wrapped ErrUnsupported. The caller then falls back to running the
// AST directly via the original Runnable. This per-function fallback
// lets us widen VM coverage incrementally without ever risking a
// regression on unsupported features.
//
// The emitter never modifies the input Runnable. It does call
// out-of-package accessor methods on the unexported AST node types
// (defined in core/compiler/vmaccess.go) to read fields it needs.
package vmcompiler
