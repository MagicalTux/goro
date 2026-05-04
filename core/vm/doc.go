// Package vm implements a stack-based bytecode virtual machine for Goro
// that runs in parallel to the AST tree-walking executor in core/compiler.
//
// The VM is opt-in. AST execution remains the default backend; the
// vmcompiler sub-package emits bytecode from compiled AST trees and a
// thin wrapper (Wrap) makes a *Function look like a phpv.Runnable so it
// plugs into the existing compile/run pipeline without changes to the
// Runnable contract.
//
// Operator semantics (numeric coercion, type juggling, GMP overload,
// deprecation warnings, …) are NOT reimplemented. The VM's opcode
// handlers call the same exported helpers in core/compiler that the AST
// uses (compiler.OperatorMath, OperatorCompare, DoInc, etc.), so any
// future bug fix or PHP-version update to those helpers is automatically
// inherited by the VM.
//
// Instruction format is a fixed 64-bit word with operand fields wide
// enough to support both stack-style ops today and register-style ops
// later without changing the encoding. The Slot type alias for *ZVal
// is the migration seam to unboxed values; opcodes don't change.
package vm
