package vmcompiler

import (
	"errors"
	"os"

	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/vm"
)

// Enabled reports whether the VM is enabled for the current process.
// Read once at init from the GORO_VM environment variable; "1", "true",
// or "yes" enable it. Anything else (including unset) leaves the AST
// runner as the only backend.
//
// Tests can override the gate with SetEnabled.
var enabled bool

// Enabled returns the current gate state.
func Enabled() bool { return enabled }

// SetEnabled toggles the VM compile hooks at runtime. Used by tests
// and benchmarks; production code should rely on the env var.
func SetEnabled(v bool) { enabled = v }

func init() {
	switch os.Getenv("GORO_VM") {
	case "1", "true", "yes", "on":
		enabled = true
	}

	// Install hooks unconditionally — the gate logic lives in the
	// hook bodies so SetEnabled takes effect immediately.
	compiler.TryBuildVMClosureBody = tryBuildClosureBody
	compiler.TryBuildVMScript = tryBuildScript
}

// tryBuildClosureBody attempts to bytecode-compile a closure body.
// Returns nil if the gate is off or the body uses unsupported
// constructs. The compiler keeps the AST body in either case.
func tryBuildClosureBody(name phpv.ZString, src *phpv.Loc, body phpv.Runnable) phpv.Runnable {
	if !enabled {
		return nil
	}
	fn, err := Compile(name, src, body)
	if err != nil {
		if errors.Is(err, ErrUnsupported) {
			return nil
		}
		// Any other error: don't crash the compile, just decline.
		// Surface a debug print? For now stay silent — the AST path
		// will run the same code.
		return nil
	}
	return vm.WrapClosureBody(fn, body)
}


// tryBuildScript wraps a top-level script's Runnables. Same gating
// logic; the wrapper goes through vm.Wrap because top-level scripts
// don't use the closure return-via-error convention. SlotOnly is set
// based on the body's IsSlotSafe analysis (which already rejects
// scripts that declare functions or use $GLOBALS / global / static).
func tryBuildScript(src *phpv.Loc, body phpv.Runnable) phpv.Runnable {
	if !enabled {
		return nil
	}
	fn, err := Compile("<main>", src, body)
	if err != nil {
		if errors.Is(err, ErrUnsupported) {
			return nil
		}
		return nil
	}
	return vm.Wrap(fn, body)
}
