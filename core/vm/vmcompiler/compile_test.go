package vmcompiler_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/KarpelesLab/goro/core/compiler"
	"github.com/KarpelesLab/goro/core/ini"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phperr"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
	"github.com/KarpelesLab/goro/core/vm"
	"github.com/KarpelesLab/goro/core/vm/vmcompiler"
	_ "github.com/KarpelesLab/goro/ext/standard"
)

// Each test compiles the same snippet through the AST and through the
// VM emitter, runs both on a fresh Global, and asserts the two paths
// produce the same return value. This is the differential contract
// that holds the VM honest.

// compileSnippet returns the AST root for `<?php <code>`. The provided
// Global is used as the parent context (so any registered functions
// from prior compiles are visible).
func compileSnippet(t *testing.T, g *phpctx.Global, code string) phpv.Runnable {
	t.Helper()
	f, err := os.CreateTemp("", "vmcompiler*.php")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })

	if _, err := f.WriteString("<?php " + code); err != nil {
		t.Fatal(err)
	}
	f.Sync()
	f.Seek(0, 0)
	defer f.Close()

	lex := tokenizer.NewLexer(f, f.Name())
	defer lex.Close()

	r, err := compiler.Compile(g, lex)
	if err != nil {
		t.Fatalf("compiler.Compile: %v", err)
	}
	return r
}

// newGlobal creates a fresh Global suitable for a single test.
func newGlobal(t *testing.T) *phpctx.Global {
	t.Helper()
	p := phpctx.NewProcess("cli")
	cfg := ini.New()
	return phpctx.NewGlobal(context.Background(), p, cfg)
}

func runAST(t *testing.T, g *phpctx.Global, r phpv.Runnable) *phpv.ZVal {
	t.Helper()
	res, err := phperr.CatchReturn(r.Run(g))
	if err != nil {
		t.Fatalf("AST Run: %v", err)
	}
	return res
}

func runVM(t *testing.T, g *phpctx.Global, fn *vm.Function) *phpv.ZVal {
	t.Helper()
	res, err := vm.Run(g, fn)
	if err != nil {
		t.Fatalf("VM Run: %v", err)
	}
	return res
}

// compareReturns runs `code` under both backends and asserts the
// results match. setup, if non-empty, is run via the AST on each
// backend's Global before the test code (used to declare functions).
func compareReturns(t *testing.T, code string, setup ...string) {
	t.Helper()

	gAST := newGlobal(t)
	for _, s := range setup {
		r := compileSnippet(t, gAST, s)
		if _, err := r.Run(gAST); err != nil {
			t.Fatalf("setup AST run: %v", err)
		}
	}
	r := compileSnippet(t, gAST, code)
	ast := runAST(t, gAST, r)

	gVM := newGlobal(t)
	for _, s := range setup {
		sr := compileSnippet(t, gVM, s)
		if _, err := sr.Run(gVM); err != nil {
			t.Fatalf("setup VM run: %v", err)
		}
	}
	r2 := compileSnippet(t, gVM, code)
	fn, err := vmcompiler.Compile("<test>", &phpv.Loc{Filename: "<test>"}, r2)
	if err != nil {
		t.Fatalf("vmcompiler.Compile: %v", err)
	}
	vmRes := runVM(t, gVM, fn)

	astStr := astString(ast)
	vmStr := astString(vmRes)
	if astStr != vmStr {
		t.Errorf("snippet=%q\n  AST: %s\n  VM:  %s", code, astStr, vmStr)
	}
}

func astString(z *phpv.ZVal) string {
	if z == nil {
		return "<nil>"
	}
	return z.GetType().String() + ":" + z.String()
}

func TestArithmeticReturn(t *testing.T) {
	cases := []string{
		"return 1 + 2;",
		"return 10 - 3;",
		"return 6 * 7;",
		"return 84 / 2;",
		"return 7 % 3;",
		"return 2 ** 8;",
		"return 0xff & 0x0f;",
		"return 0xf0 | 0x0f;",
		"return 0xff ^ 0xf0;",
		"return 1 << 4;",
		"return 256 >> 2;",
		"return 'foo' . 'bar';",
		"return -42;",
		"return ~0;",
		"return !true;",
	}
	for _, c := range cases {
		t.Run(strings.ReplaceAll(c, " ", "_"), func(t *testing.T) {
			compareReturns(t, c)
		})
	}
}

func TestCompareReturn(t *testing.T) {
	cases := []string{
		"return 1 == 1;",
		"return 1 === '1';",
		"return 1 !== '1';",
		"return 1 < 2;",
		"return 2 <= 2;",
		"return 3 > 2;",
		"return 2 >= 2;",
		"return 1 <=> 2;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestVarsAndAssignments(t *testing.T) {
	cases := []string{
		"$x = 7; return $x;",
		"$x = 7; $y = 35; return $x + $y;",
		"$x = 1; $x += 41; return $x;",
		"$x = 50; $x -= 8; return $x;",
		"$x = 'foo'; $x .= 'bar'; return $x;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestIncDec(t *testing.T) {
	cases := []string{
		"$x = 5; ++$x; return $x;",
		"$x = 5; --$x; return $x;",
		"$x = 5; $y = $x++; return $x + $y * 100;", // post: $y = 5, $x = 6, total 506
		"$x = 5; $y = ++$x; return $x + $y * 100;", // pre: both = 6, total 606
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestIfElse(t *testing.T) {
	cases := []string{
		"if (true) return 1; return 2;",
		"if (false) return 1; return 2;",
		"$x = 5; if ($x > 3) return 'big'; return 'small';",
		"$x = 1; if ($x === 1) { return 'a'; } else { return 'b'; }",
		"$x = 2; if ($x === 1) return 'a'; elseif ($x === 2) return 'b'; else return 'c';",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestWhile(t *testing.T) {
	cases := []string{
		"$x = 0; while ($x < 5) { $x = $x + 1; } return $x;",
		"$x = 10; $sum = 0; while ($x > 0) { $sum += $x; $x--; } return $sum;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestFor(t *testing.T) {
	cases := []string{
		"$sum = 0; for ($i = 0; $i < 5; $i++) { $sum = $sum + $i; } return $sum;",
		"$sum = 0; for ($i = 0; $i < 100; $i++) $sum += $i; return $sum;",
		"$s = ''; for ($i = 0; $i < 4; $i++) $s .= 'x'; return $s;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestBreakContinue(t *testing.T) {
	cases := []string{
		"$s = 0; for ($i = 0; $i < 10; $i++) { if ($i === 5) break; $s += $i; } return $s;",
		"$s = 0; for ($i = 0; $i < 10; $i++) { if ($i % 2 === 0) continue; $s += $i; } return $s;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestStringInterpolation(t *testing.T) {
	cases := []string{
		// simple variable interpolation
		`$name = 'world'; return "hello $name";`,
		// brace form
		`$x = 7; return "value: {$x}";`,
		// multiple variables
		`$a = 1; $b = 2; return "a=$a b=$b";`,
		// numeric coercion
		`$n = 42; return "answer is $n!";`,
		// nested concat
		`$a = 'hi'; $b = 'world'; return "first: $a, second: $b!";`,
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestCoalesce(t *testing.T) {
	cases := []string{
		// Defined non-null
		"$x = 'val'; return $x ?? 'default';",
		// Undefined → fallback (no warning)
		"return $undef ?? 'default';",
		// Null → fallback
		"$x = null; return $x ?? 'default';",
		// Zero is set and non-null → keep it
		"$x = 0; return $x ?? 'default';",
		// Empty string is set and non-null → keep it
		"$x = ''; return $x ?? 'default';",
		// Chain
		"return $a ?? $b ?? 'last';",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestShortCircuit(t *testing.T) {
	cases := []string{
		"return true && true;",
		"return true && false;",
		"return false && true;", // short-circuit, doesn't eval RHS
		"return false || true;",
		"return true || false;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

// throw is exercised indirectly by integration_test.go — the test
// runs a script that defines a VM-compiled function that throws,
// then catches the exception in an AST-evaluated try/catch wrapper.

func TestTryCatch(t *testing.T) {
	cases := []string{
		// no exception thrown
		"$x = 0; try { $x = 1; } catch (Exception $e) { $x = 2; } return $x;",
		// caught
		"try { throw new Exception('boom'); } catch (Exception $e) { return $e->getMessage(); }",
		// type mismatch — not caught (use a class that doesn't match)
		"try { try { throw new RuntimeException('x'); } catch (LogicException $e) { return 'wrong'; } } catch (RuntimeException $e) { return 'right'; }",
		// nested try/catch
		"try { try { throw new Exception('inner'); } catch (Error $e) { return 'wrong-inner'; } } catch (Exception $e) { return $e->getMessage(); }",
		// catch without var
		"try { throw new Exception('x'); } catch (Exception) { return 'caught'; }",
		// exception in catch body propagates to outer
		"try { try { throw new Exception('a'); } catch (Exception $e) { throw new Exception('b'); } } catch (Exception $e) { return $e->getMessage(); }",
		// multiple catches, second one matches
		"try { throw new RuntimeException('rt'); } catch (LogicException $e) { return 'logic'; } catch (RuntimeException $e) { return 'runtime'; }",
		// union type catch
		"try { throw new RuntimeException('rt'); } catch (LogicException | RuntimeException $e) { return get_class($e); }",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

// Property write is deferred to AST until we extract a helper that
// mirrors runObjectVar.WriteValue exactly. The integration test
// confirms that calling a method that writes to $this->x still works
// (the method's body falls back to AST).

func TestObjectAccess(t *testing.T) {
	classDecl := `
		class Pt {
			public $x;
			public $y;
			function __construct($x, $y) { $this->x = $x; $this->y = $y; }
			function dist2() { return $this->x * $this->x + $this->y * $this->y; }
			function add($other) { return new Pt($this->x + $other->x, $this->y + $other->y); }
		}
	`
	cases := []string{
		// Property read
		"$p = new Pt(3, 4); return $p->x + $p->y;",
		// Method call
		"$p = new Pt(3, 4); return $p->dist2();",
		// Method returning an object → chained property read
		"$a = new Pt(1, 2); $b = new Pt(10, 20); return $a->add($b)->x + $a->add($b)->y;",
		// Method-call result used in arithmetic
		"$p = new Pt(3, 4); $a = new Pt(1, 1); return $p->dist2() + $a->dist2();",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, classDecl) })
	}
}

func TestForeach(t *testing.T) {
	cases := []string{
		// value form
		"$s = 0; foreach ([1,2,3] as $v) { $s += $v; } return $s;",
		// key + value
		"$s = ''; foreach (['a'=>1,'b'=>2,'c'=>3] as $k => $v) { $s .= $k . $v; } return $s;",
		// over a local var
		"$a = [10, 20, 30]; $s = 0; foreach ($a as $v) { $s += $v; } return $s;",
		// break inside
		"$s = 0; foreach ([1,2,3,4,5] as $v) { if ($v > 3) break; $s += $v; } return $s;",
		// continue inside
		"$s = 0; foreach ([1,2,3,4,5] as $v) { if ($v % 2 === 0) continue; $s += $v; } return $s;",
		// nested foreach
		"$s = 0; foreach ([[1,2],[3,4]] as $row) { foreach ($row as $v) { $s += $v; } } return $s;",
		// over null (foreach() argument warning)
		"$x = null; $s = 0; foreach ($x as $v) { $s++; } return $s;",
		// non-iterable type — emits warning, no iteration
		"$x = 5; foreach ($x as $v) {} return 'ok';",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestArrays(t *testing.T) {
	cases := []string{
		// literals
		"$a = []; return count($a);",
		"$a = [1,2,3]; return $a[0] + $a[1] + $a[2];",
		"$a = ['x' => 7, 'y' => 35]; return $a['x'] + $a['y'];",
		"$a = [10, 'k' => 20, 30]; return $a[0] + $a['k'] + $a[1];",
		// append
		"$a = []; $a[] = 1; $a[] = 2; $a[] = 3; return $a[0] + $a[1] + $a[2];",
		// indexed write
		"$a = []; $a[0] = 'foo'; $a[1] = 'bar'; return $a[0] . $a[1];",
		// auto-vivification from null
		"$a[] = 7; $a[] = 35; return $a[0] + $a[1];",
		// chained read (container is itself an array access)
		"$a = [[1,2],[3,4]]; return $a[1][0];",
		// mixed: literal in for-loop
		"$a = []; for ($i = 0; $i < 5; $i++) { $a[] = $i * 2; } return $a[0] + $a[2] + $a[4];",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestUnsupportedNodeFallsBack(t *testing.T) {
	// Spread in array literal isn't supported — the emitter should
	// report that and the function-level fallback restores the AST.
	g := newGlobal(t)
	r := compileSnippet(t, g, "$src = [1,2,3]; $a = [0, ...$src]; return $a[3];")
	_, err := vmcompiler.Compile("<test>", &phpv.Loc{Filename: "<test>"}, r)
	if err == nil {
		t.Fatalf("expected ErrUnsupported, got nil")
	}
	if !errorsIsUnsupported(err) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}

func TestFunctionCall(t *testing.T) {
	// Define a function in setup, call it via VM-compiled bytecode.
	addDecl := "function vm_test_add($a, $b) { return $a + $b; }"
	cases := []string{
		"return vm_test_add(1, 2);",
		"$s = 0; for ($i = 0; $i < 10; $i++) { $s = vm_test_add($s, $i); } return $s;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			compareReturns(t, c, addDecl)
		})
	}

	// Built-in: strlen, no setup needed.
	t.Run("builtin_strlen", func(t *testing.T) {
		compareReturns(t, "return strlen('hello');")
	})

	// Recursive user function.
	fibDecl := "function vm_test_fib($n) { if ($n <= 1) return $n; return vm_test_fib($n - 1) + vm_test_fib($n - 2); }"
	t.Run("recursive_fib", func(t *testing.T) {
		compareReturns(t, "return vm_test_fib(10);", fibDecl)
	})
}

func errorsIsUnsupported(err error) bool {
	for ; err != nil; err = unwrap(err) {
		if err == vmcompiler.ErrUnsupported {
			return true
		}
	}
	return false
}

func unwrap(err error) error {
	type wrapper interface{ Unwrap() error }
	if w, ok := err.(wrapper); ok {
		return w.Unwrap()
	}
	return nil
}
