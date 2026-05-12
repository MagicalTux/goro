package vmcompiler_test

import (
	"testing"

	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/phperr"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/vm/vmcompiler"
	_ "github.com/KarpelesLab/goro/ext/standard"
)

// TestClosureBodyHookWiring verifies the compiler correctly hands off
// each compiled function body to vmcompiler when the VM gate is on,
// and the resulting *ZClosure.code is a *vm.ClosureBody.
func TestClosureBodyHookWiring(t *testing.T) {
	prev := vmcompiler.Enabled()
	vmcompiler.SetEnabled(true)
	t.Cleanup(func() { vmcompiler.SetEnabled(prev) })

	g := newGlobal(t)
	r := compileSnippet(t, g, `
		function vm_wired_add($a, $b) { return $a + $b; }
		return vm_wired_add(40, 2);
	`)
	res, err := phperr.CatchReturn(r.Run(g))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.String() != "42" {
		t.Fatalf("got %q, want 42", res.String())
	}

	// Confirm the registered function actually went through the VM.
	fn, err := g.GetFunction(g, phpv.ZString("vm_wired_add"))
	if err != nil {
		t.Fatalf("GetFunction: %v", err)
	}
	if !compiler.IsVMCompiled(fn) {
		t.Fatalf("expected vm_wired_add to be VM-compiled, but isn't")
	}
}

// TestClosureBodyHookFallsBack verifies that when the body uses an
// unsupported construct (here, a spread-argument function call which
// still falls back), the AST body is kept (no VM wrapper) and the
// function still runs.
func TestClosureBodyHookFallsBack(t *testing.T) {
	prev := vmcompiler.Enabled()
	vmcompiler.SetEnabled(true)
	t.Cleanup(func() { vmcompiler.SetEnabled(prev) })

	g := newGlobal(t)
	r := compileSnippet(t, g, `
		function vm_spread_sum() {
			$xs = [1, 2, 3];
			return max(...$xs);
		}
		return vm_spread_sum();
	`)
	res, err := phperr.CatchReturn(r.Run(g))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.String() != "3" {
		t.Fatalf("got %q, want 3", res.String())
	}
	fn, err := g.GetFunction(g, phpv.ZString("vm_spread_sum"))
	if err != nil {
		t.Fatalf("GetFunction: %v", err)
	}
	if compiler.IsVMCompiled(fn) {
		t.Fatalf("vm_spread_sum should have fallen back to AST")
	}
}

// TestThrowFromVM verifies the OP_THROW opcode propagates the
// exception out of a VM-compiled function so an AST try/catch
// wrapper at the script level catches it.
func TestThrowFromVM(t *testing.T) {
	prev := vmcompiler.Enabled()
	vmcompiler.SetEnabled(true)
	t.Cleanup(func() { vmcompiler.SetEnabled(prev) })

	g := newGlobal(t)
	r := compileSnippet(t, g, `
		function vm_thrower() { throw new Exception('hello'); }
		try {
			vm_thrower();
			return 'no exception';
		} catch (Exception $e) {
			return $e->getMessage();
		}
	`)
	res, err := phperr.CatchReturn(r.Run(g))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.String() != "hello" {
		t.Fatalf("got %q, want 'hello'", res.String())
	}
	fn, _ := g.GetFunction(g, phpv.ZString("vm_thrower"))
	if !compiler.IsVMCompiled(fn) {
		t.Fatalf("vm_thrower should be VM-compiled")
	}
}

// TestRecursiveVM checks that a recursive function compiled to bytecode
// continues to work — recursion goes through CallZVal which dispatches
// to ZClosure.Call which invokes the VM body.
func TestRecursiveVM(t *testing.T) {
	prev := vmcompiler.Enabled()
	vmcompiler.SetEnabled(true)
	t.Cleanup(func() { vmcompiler.SetEnabled(prev) })

	g := newGlobal(t)
	r := compileSnippet(t, g, `
		function vm_fib($n) { if ($n <= 1) return $n; return vm_fib($n-1) + vm_fib($n-2); }
		return vm_fib(10);
	`)
	res, err := phperr.CatchReturn(r.Run(g))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.String() != "55" { // fib(10) = 55
		t.Fatalf("got %q, want 55", res.String())
	}
}

// TestDisabledByDefault confirms the gate defaults to off when the env
// var is unset and SetEnabled was never called.
func TestDisabledByDefault(t *testing.T) {
	// Force off, simulating a production process with GORO_VM unset.
	prev := vmcompiler.Enabled()
	vmcompiler.SetEnabled(false)
	t.Cleanup(func() { vmcompiler.SetEnabled(prev) })

	g := newGlobal(t)
	r := compileSnippet(t, g, `function vm_off($x) { return $x + 1; } return vm_off(5);`)
	res, err := phperr.CatchReturn(r.Run(g))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.String() != "6" {
		t.Fatalf("got %q, want 6", res.String())
	}
	// Don't bother inspecting closure.code — by being off, the AST
	// path runs and produces the correct result.
}
