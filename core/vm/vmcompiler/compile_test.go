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
	fn, err := vmcompiler.Compile("<test>", &phpv.Loc{Filename: "<test>"}, r2, gVM)
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

func TestClassConst(t *testing.T) {
	classDecl := `
		class Color {
			const RED = '#f00';
			const GREEN = '#0f0';
			const BLUE = '#00f';
			const ALL = [self::RED, self::GREEN, self::BLUE];
		}
	`
	cases := []string{
		`return Color::RED;`,
		`return Color::RED . '/' . Color::GREEN;`,
		// self-referential const (CompileDelayed)
		`$arr = Color::ALL; return $arr[0] . $arr[1] . $arr[2];`,
		// ::class
		`return Color::class;`,
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, classDecl) })
	}
}

func TestInlineClosures(t *testing.T) {
	cases := []string{
		// Anonymous closure called immediately
		`$f = function() { return 42; }; return $f();`,
		// Closure with arg
		`$f = function($x) { return $x * 2; }; return $f(21);`,
		// Closure captures by value
		`$x = 7; $f = function() use ($x) { return $x + 1; }; $x = 100; return $f();`,
		// Arrow function (auto-capture by value)
		`$x = 5; $f = fn($y) => $x + $y; return $f(10);`,
		// Closure passed to array_map
		`$arr = [1, 2, 3]; $sq = array_map(function($v) { return $v * $v; }, $arr); return $sq[0] + $sq[1] + $sq[2];`,
		// Nested closure
		`$outer = function($x) { return function($y) use ($x) { return $x + $y; }; }; $f = $outer(10); return $f(5);`,
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

func TestTryFinally(t *testing.T) {
	cases := []string{
		// finally runs after normal completion
		"$s = ''; try { $s .= 'try'; } finally { $s .= '/fin'; } return $s;",
		// finally runs after caught exception
		"$s = ''; try { throw new Exception('e'); } catch (Exception $e) { $s .= 'caught'; } finally { $s .= '/fin'; } return $s;",
		// finally runs after return
		"function vm_tf_a() { try { return 'try'; } finally { /* runs */ } } return vm_tf_a();",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

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

func TestStaticPropertyWrite(t *testing.T) {
	classDecl := `
		class Counter {
			public static $count = 0;
			public static function inc() { self::$count++; }
		}
	`
	cases := []string{
		"Counter::$count = 5; return Counter::$count;",
		"Counter::$count = 0; Counter::inc(); Counter::inc(); return Counter::$count;",
		"Counter::$count = 1; Counter::$count += 10; return Counter::$count;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, classDecl) })
	}
}

func TestPropertyWrite(t *testing.T) {
	classDecl := `
		class Box {
			public $x = 0;
			public $y = 0;
			function set($v) { $this->x = $v; }
			function bump() { $this->x += 10; }
			function chain() { $this->x = $this->y = 7; }
		}
	`
	cases := []string{
		// Direct property write
		"$b = new Box(); $b->x = 42; return $b->x;",
		// Method that writes $this->prop
		"$b = new Box(); $b->set(99); return $b->x;",
		// Compound assignment on property
		"$b = new Box(); $b->x = 5; $b->bump(); return $b->x;",
		// Chained property writes ($a = $b = v)
		"$b = new Box(); $b->chain(); return $b->x + $b->y;",
		// Write then read in same expression
		"$b = new Box(); $b->x = 10; return $b->x + 5;",
		// Multiple writes
		"$b = new Box(); $b->x = 3; $b->y = 4; return $b->x * $b->x + $b->y * $b->y;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, classDecl) })
	}
}

func TestByRefParam(t *testing.T) {
	setup := `
		function vm_swap(&$a, &$b) { $tmp = $a; $a = $b; $b = $tmp; }
		function vm_incr(&$x, $by) { $x += $by; return $x; }
		function vm_push(&$arr, $v) { $arr[] = $v; }
	`
	cases := []string{
		"$x = 1; $y = 2; vm_swap($x, $y); return $x * 10 + $y;",      // 21
		"$x = 5; $r = vm_incr($x, 3); return $x * 10 + $r;",           // 88
		"$a = [1,2]; vm_push($a, 3); vm_push($a, 4); return array_sum($a);", // 10
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, setup) })
	}
}

func TestGeneratorIteration(t *testing.T) {
	setup := `
		function vm_gen() {
			yield 1;
			yield 2;
			yield 3;
		}
		function vm_keyed_gen() {
			yield 'a' => 10;
			yield 'b' => 20;
		}
	`
	cases := []string{
		// Iterate generator via foreach
		"$sum = 0; foreach (vm_gen() as $v) { $sum += $v; } return $sum;",
		// Iterate with key
		"$out = ''; foreach (vm_keyed_gen() as $k => $v) { $out .= $k . '=' . $v . ';'; } return $out;",
		// Break out of generator iteration
		"$first = null; foreach (vm_gen() as $v) { $first = $v; break; } return $first;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, setup) })
	}
}

func TestCloneInstanceofDoWhile(t *testing.T) {
	classDecl := `
		class Box { public $v = 0; }
		class Animal {}
		class Dog extends Animal {}
	`
	cases := []string{
		// clone produces independent object
		"$a = new Box(); $a->v = 7; $b = clone $a; $b->v = 99; return $a->v + $b->v;",
		// instanceof check
		"$d = new Dog(); return $d instanceof Animal ? 1 : 0;",
		"$d = new Dog(); return $d instanceof Box ? 1 : 0;",
		// do-while loop runs at least once
		"$x = 0; do { $x++; } while ($x < 5); return $x;",
		"$x = 10; do { $x++; } while ($x < 5); return $x;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, classDecl) })
	}
}

func TestIncDecNonLocal(t *testing.T) {
	classDecl := `
		class Counter {
			public $n = 0;
			public static $shared = 0;
		}
	`
	cases := []string{
		// postfix on property
		"$c = new Counter(); $a = $c->n++; return $a + $c->n;",
		// prefix on property
		"$c = new Counter(); $a = ++$c->n; return $a + $c->n;",
		// postfix on array element
		"$a = [10]; $b = $a[0]++; return $a[0] + $b;",
		// postfix on static property
		"Counter::$shared = 0; $a = Counter::$shared++; return $a + Counter::$shared;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, classDecl) })
	}
}

func TestCompoundAssignAsExpr(t *testing.T) {
	cases := []string{
		"$x = 1; $y = ($x += 5); return $x + $y;",       // both 6
		"$x = 10; $y = ($x -= 3) * 2; return $y;",       // 14
		"$x = 'a'; $y = ($x .= 'b'); return $x . $y;",   // 'abab'
		"$a = 4; $b = ($a *= 2) + ($a += 1); return $b;", // (8) + (8+1) = 17
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestDestructure(t *testing.T) {
	cases := []string{
		// short-list with positional
		"[$a, $b] = [10, 20]; return $a + $b;",
		// list() keyword form
		"list($a, $b) = [3, 4]; return $a * $b;",
		// keyed destructure
		"['x' => $x, 'y' => $y] = ['x' => 1, 'y' => 2]; return $x + $y;",
		// nested
		"[$a, [$b, $c]] = [1, [2, 3]]; return $a + $b + $c;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestVariableVariables(t *testing.T) {
	cases := []string{
		// $$ read
		"$name = 'x'; $x = 7; return $$name;",
		// $$ write
		"$name = 'y'; $$name = 42; return $y;",
		// ${$expr} read
		"$name = 'foo'; $foo = 100; return ${$name};",
		// ${$expr} write
		"$key = 'count'; ${$key} = 5; return $count;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestTypedReturn(t *testing.T) {
	setup := `
		function vm_int_id(int $x): int { return $x; }
		function vm_str_cast(int $x): string { return $x; }
		function vm_nullable(?int $x): ?int { return $x; }
		function vm_void_ret(): void { return; }
	`
	cases := []string{
		"return vm_int_id(7);",
		"return vm_str_cast(123);",
		"return vm_nullable(null) ?? -1;",
		"return vm_nullable(42);",
		"vm_void_ret(); return 'ok';",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, setup) })
	}
}

func TestUserConstants(t *testing.T) {
	setup := `
		const MY_INT = 42;
		const MY_STR = 'hello';
		define('MY_RT', 99);
	`
	cases := []string{
		"return MY_INT;",
		"return MY_STR;",
		"return MY_RT + 1;",
		"return PHP_INT_MAX;",
		"return PHP_INT_SIZE;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, setup) })
	}
}

func TestGlobalStaticDecl(t *testing.T) {
	setup := `
		$g_counter = 100;
		function vm_use_global() {
			global $g_counter;
			$g_counter += 5;
			return $g_counter;
		}
		function vm_use_static() {
			static $n = 0;
			$n++;
			return $n;
		}
	`
	cases := []string{
		// global reads/writes outer
		"return vm_use_global();",
		// static persists across calls
		"vm_use_static(); vm_use_static(); return vm_use_static();",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, setup) })
	}
}

func TestMatchExpression(t *testing.T) {
	cases := []string{
		"$x = 2; return match ($x) { 1 => 'one', 2 => 'two', 3 => 'three' };",
		"$x = 4; return match ($x) { 1, 2 => 'low', 3, 4 => 'mid', default => 'high' };",
		"$x = 'go'; return match (true) { $x === 'go' => 'going', $x === 'stop' => 'stopped' };",
		"$y = 0; foreach ([1,2,3] as $v) { $y += match ($v) { 1 => 10, 2 => 20, 3 => 30 }; } return $y;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestSwitchStatement(t *testing.T) {
	cases := []string{
		// Plain switch
		"$x = 'b'; $r = ''; switch ($x) { case 'a': $r = 'A'; break; case 'b': $r = 'B'; break; default: $r = 'X'; } return $r;",
		// Fall-through
		"$x = 1; $r = 0; switch ($x) { case 1: case 2: $r = 12; break; case 3: $r = 3; } return $r;",
		// switch with return inside (propagates out)
		"function vm_sw_ret($x) { switch ($x) { case 1: return 'one'; case 2: return 'two'; } return 'other'; } return vm_sw_ret(2);",
		// nested switch with break exiting inner only
		"$x = 1; $y = 'a'; $r = ''; switch ($x) { case 1: switch ($y) { case 'a': $r = 'A1'; break; } $r .= '!'; break; case 2: $r = 'two'; } return $r;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestNewObjectShapes(t *testing.T) {
	classDecl := `
		class Pair {
			public $a;
			public $b;
			public function __construct($a, $b) { $this->a = $a; $this->b = $b; }
			public function sum() { return $this->a + $this->b; }
		}
	`
	cases := []string{
		// Spread into constructor
		"$args = [3, 4]; $p = new Pair(...$args); return $p->sum();",
		// Named args in constructor
		"$p = new Pair(b: 10, a: 5); return $p->sum();",
		// Anonymous class
		"$o = new class { public $x = 17; public function get() { return $this->x; } }; return $o->get();",
		// Dynamic class name
		"$cls = 'Pair'; $p = new $cls(1, 2); return $p->sum();",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, classDecl) })
	}
}

func TestSpecialArgs(t *testing.T) {
	addDecl := `
		function vm_spread_add($a, $b, $c) { return $a + $b + $c; }
		function vm_named_div($n, $d) { return $n / $d; }
		class Calc {
			public function sum(...$xs) { return array_sum($xs); }
			public function div($num, $den) { return $num / $den; }
		}
	`
	cases := []string{
		// Spread into user function
		"$xs = [1, 2, 3]; return vm_spread_add(...$xs);",
		// Spread builtin
		"$xs = [10, 20, 5]; return max(...$xs);",
		// Named arg
		"return vm_named_div(d: 4, n: 20);",
		// Spread into method
		"$c = new Calc(); return $c->sum(1, 2, 3, 4);",
		"$c = new Calc(); $xs = [10, 20]; return $c->sum(...$xs);",
		// Named arg in method
		"$c = new Calc(); return $c->div(den: 5, num: 30);",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, addDecl) })
	}
}

func TestNullsafe(t *testing.T) {
	classDecl := `
		class Inner {
			public $val = 42;
			public function ping() { return 'pong'; }
		}
		class Outer {
			public ?Inner $inner = null;
		}
	`
	cases := []string{
		// Nullsafe property on null
		"$o = new Outer(); return $o->inner?->val ?? -1;",
		// Nullsafe property on set
		"$o = new Outer(); $o->inner = new Inner(); return $o->inner?->val;",
		// Nullsafe method on null
		"$o = new Outer(); return $o->inner?->ping() ?? 'gone';",
		// Nullsafe method on set
		"$o = new Outer(); $o->inner = new Inner(); return $o->inner?->ping();",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, classDecl) })
	}
}

func TestTernary(t *testing.T) {
	cases := []string{
		// Plain ternary
		"$x = 5; return $x > 0 ? 'pos' : 'neg';",
		"return 1 == 1 ? 100 : 200;",
		// Short ternary `?:` — reuses cond when truthy
		"$x = 'hi'; return $x ?: 'default';",
		"$x = 0; return $x ?: 99;",
		"$x = null; return $x ?: 'fallback';",
		// Nested ternary
		"$a = 2; return $a == 1 ? 'one' : ($a == 2 ? 'two' : 'other');",
		// Ternary returning array index
		"$arr = ['a','b']; $i = 1; return $arr[$i > 0 ? 1 : 0];",
		// Ternary in arithmetic
		"$x = 5; return ($x > 0 ? $x : -$x) + 1;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestUnsetIssetEmpty(t *testing.T) {
	classDecl := `
		class Bag {
			public $cur = 0;
			public $list = [];
		}
	`
	cases := []string{
		// isset of a local
		"$a = 1; return isset($a) ? 1 : 0;",
		// isset of undeclared
		"return isset($undeclared) ? 1 : 0;",
		// empty
		"$a = ''; return empty($a) ? 1 : 0;",
		"$a = 'x'; return empty($a) ? 1 : 0;",
		// unset removes the variable
		"$a = 1; unset($a); return isset($a) ? 1 : 0;",
		// unset of multiple
		"$a = 1; $b = 2; $c = 3; unset($a, $c); return isset($a) + isset($b) + isset($c);",
		// unset of array element
		"$a = [1,2,3]; unset($a[1]); return count($a);",
		// unset of object property
		"$b = new Bag(); $b->cur = 5; unset($b->cur); return isset($b->cur) ? 1 : 0;",
		// isset of array element with missing key
		"$a = ['x' => 1]; return isset($a['y']) ? 1 : 0;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, classDecl) })
	}
}

func TestForeachByRef(t *testing.T) {
	classDecl := `
		class Bag {
			public $cur = 0;
		}
	`
	cases := []string{
		// foreach by reference mutates source
		"$a = [1,2,3]; foreach ($a as &$v) { $v *= 10; } return array_sum($a);",
		// foreach by ref with key
		"$a = ['x'=>1,'y'=>2]; foreach ($a as $k => &$v) { $v += 100; } return $a['x'] + $a['y'];",
		// non-local value target: object property
		"$b = new Bag(); $sum = 0; foreach ([10,20,30] as $b->cur) { $sum += $b->cur; } return $sum;",
		// non-local key target: object property as key
		"$b = new Bag(); $vs = 0; foreach (['a','b','c'] as $b->cur => $v) { $vs += $b->cur; } return $vs;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, classDecl) })
	}
}

func TestMultiLevelBreakContinue(t *testing.T) {
	cases := []string{
		// break 2 exits both loops
		"$hit = 0; for ($i=0;$i<3;$i++) { for ($j=0;$j<3;$j++) { $hit++; if ($j==1) break 2; } } return $hit;",
		// continue 2 skips to outer step
		"$s = 0; for ($i=0;$i<3;$i++) { for ($j=0;$j<3;$j++) { if ($j==1) continue 2; $s++; } } return $s;",
		// break N with N == loop count exits all
		"$ok = 0; for ($i=0;$i<2;$i++) { for ($j=0;$j<2;$j++) { for ($k=0;$k<2;$k++) { $ok++; break 3; } } } return $ok;",
		// continue 2 then break 2 — combined
		"$s = 0; for ($i=0;$i<3;$i++) { for ($j=0;$j<3;$j++) { if ($i==1 && $j==1) break 2; $s++; } } return $s;",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestArraySpread(t *testing.T) {
	cases := []string{
		"$a = [1, 2, 3]; $b = [0, ...$a, 4]; return array_sum($b);",
		"$a = ['x' => 1, 'y' => 2]; $b = [...$a, 'z' => 3]; return $b['x'] + $b['y'] + $b['z'];",
		"$a = [10, 20]; $b = [...$a, ...$a]; return count($b);",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

func TestNestedArrayWrite(t *testing.T) {
	classDecl := `
		class Bag {
			public $items = [];
		}
	`
	cases := []string{
		// Nested local array writes (two-level)
		"$a = []; $a['x']['y'] = 42; return $a['x']['y'];",
		// Three-level
		"$a = []; $a[0][1][2] = 7; return $a[0][1][2];",
		// Object property containing an array
		"$b = new Bag(); $b->items['hello'] = 'world'; return $b->items['hello'];",
		// Compound write on nested array
		"$a = ['k' => ['v' => 1]]; $a['k']['v'] += 9; return $a['k']['v'];",
		// Append to property array
		"$b = new Bag(); $b->items[] = 1; $b->items[] = 2; $b->items[] = 3; return $b->items[2];",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c, classDecl) })
	}
}

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
		// COW: $b = $a; modifying $b via sort doesn't affect $a
		"$a = [3, 1, 2]; $b = $a; sort($b); return $a[0] * 100 + $b[0];", // 3*100 + 1 = 301
		// COW: $b = $a; modifying $b via [] doesn't affect $a
		"$a = [1, 2]; $b = $a; $b[] = 99; return count($a) + count($b);", // 2 + 3 = 5
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) { compareReturns(t, c) })
	}
}

// TestUnsupportedNodeFallsBack used to verify that nodes the emitter
// doesn't handle return ErrUnsupported, so the function-level
// fallback restores the AST. The catch-all is no longer reachable
// from any valid script — coverage is wide enough that every
// node either compiles natively or AST-delegates. Fallback for
// closures (generators, by-ref params, by-ref return) is verified
// by integration_test.go::TestClosureBodyHookFallsBack.

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
