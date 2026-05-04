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

// compileSnippet returns the AST root for `<?php <code>`.
func compileSnippet(t *testing.T, code string) phpv.Runnable {
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

	p := phpctx.NewProcess("cli")
	cfg := ini.New()
	g := phpctx.NewGlobal(context.Background(), p, cfg)

	r, err := compiler.Compile(g, lex)
	if err != nil {
		t.Fatalf("compiler.Compile: %v", err)
	}
	return r
}

func runAST(t *testing.T, r phpv.Runnable) *phpv.ZVal {
	t.Helper()
	p := phpctx.NewProcess("cli")
	cfg := ini.New()
	g := phpctx.NewGlobal(context.Background(), p, cfg)
	res, err := phperr.CatchReturn(r.Run(g))
	if err != nil {
		t.Fatalf("AST Run: %v", err)
	}
	return res
}

func runVM(t *testing.T, fn *vm.Function) *phpv.ZVal {
	t.Helper()
	p := phpctx.NewProcess("cli")
	cfg := ini.New()
	g := phpctx.NewGlobal(context.Background(), p, cfg)
	res, err := vm.Run(g, fn)
	if err != nil {
		t.Fatalf("VM Run: %v", err)
	}
	return res
}

func compareReturns(t *testing.T, code string) {
	t.Helper()
	r := compileSnippet(t, code)
	fn, err := vmcompiler.Compile("<test>", &phpv.Loc{Filename: "<test>"}, r)
	if err != nil {
		t.Fatalf("vmcompiler.Compile: %v", err)
	}

	ast := runAST(t, r)
	vmRes := runVM(t, fn)

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

func TestUnsupportedNodeFallsBack(t *testing.T) {
	// Arrays aren't supported yet — the emitter should report that.
	r := compileSnippet(t, "$a = [1,2,3]; return $a[0];")
	_, err := vmcompiler.Compile("<test>", &phpv.Loc{Filename: "<test>"}, r)
	if err == nil {
		t.Fatalf("expected ErrUnsupported, got nil")
	}
	if !errorsIsUnsupported(err) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
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
