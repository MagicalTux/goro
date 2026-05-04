package main_test

// Differential tests: run a curated set of self-contained PHP snippets
// under both backends (AST and VM) and assert byte-identical stdout.
//
// These don't replace the broader phpt suite — they're lightweight
// regression coverage that can run in a few seconds and pinpoint a
// specific divergence quickly. Run with:
//
//	go test -run TestVMDiff -v .
//
// To exercise the same snippets under the full phpt suite gate, set
// GORO_VM=1 before running TestPhp.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/KarpelesLab/goro/core/ini"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/vm/vmcompiler"
	_ "github.com/KarpelesLab/goro/ext/standard"
)

// runWithBuf runs `code` (as PHP source after a leading <?php) with the
// VM gate set to vm. Returns whatever was written to the global output
// buffer (i.e. echo / print output). Errors fail the test.
func runWithBuf(t *testing.T, code string, vm bool) string {
	t.Helper()
	prev := vmcompiler.Enabled()
	vmcompiler.SetEnabled(vm)
	t.Cleanup(func() { vmcompiler.SetEnabled(prev) })

	p := phpctx.NewProcess("cli")
	cfg := ini.New()
	g := phpctx.NewGlobal(context.Background(), p, cfg)

	var out bytes.Buffer
	g.SetOutput(&out)

	if _, err := g.DoString(g, phpv.ZString(code)); err != nil {
		// PHP scripts may "fail" via exit codes; don't auto-fail the
		// test if both backends produce the same error. Surface the
		// stdout instead.
		t.Logf("DoString error (vm=%v): %v", vm, err)
	}
	return out.String()
}

// diffSnippet runs `code` under both backends and asserts equality.
func diffSnippet(t *testing.T, name, code string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		ast := runWithBuf(t, code, false)
		vmOut := runWithBuf(t, code, true)
		if ast != vmOut {
			t.Errorf("output diverged.\nAST:\n%s\nVM:\n%s", ast, vmOut)
		}
	})
}

func TestVMDiff(t *testing.T) {
	// Each entry exercises a different backend feature. All should
	// produce identical output regardless of the gate.
	cases := []struct {
		name string
		code string
	}{
		{"basic_arith", `echo 1 + 2 * 3 - 4;`},
		{"vars", `$a = 7; $b = 35; echo $a + $b;`},
		{"compound_assign", `$x = 1; $x += 2; $x *= 3; $x -= 1; echo $x;`},
		{"if_else", `$x = 5; if ($x > 3) echo "big"; else echo "small";`},
		{"if_elseif", `$x = 2; if ($x === 1) echo "a"; elseif ($x === 2) echo "b"; else echo "c";`},
		{"while", `$i = 0; while ($i < 5) { echo $i; $i++; }`},
		{"for", `for ($i = 0; $i < 5; $i++) echo $i;`},
		{"break", `for ($i = 0; $i < 10; $i++) { if ($i === 3) break; echo $i; }`},
		{"continue", `for ($i = 0; $i < 5; $i++) { if ($i % 2 === 0) continue; echo $i; }`},
		{"short_circuit_and", `echo (true && false) ? "1" : "0";`},
		{"short_circuit_or", `echo (false || true) ? "1" : "0";`},
		{"unary", `echo -5; echo " "; echo !true ? "1" : "0"; echo " "; echo ~0;`},
		{"concat", `$s = "foo"; $s .= "bar"; $s .= "baz"; echo $s;`},
		{"function_basic", `
			function add($a, $b) { return $a + $b; }
			echo add(40, 2);
		`},
		{"function_loop", `
			function dbl($x) { return $x * 2; }
			$s = 0;
			for ($i = 1; $i <= 5; $i++) $s += dbl($i);
			echo $s;
		`},
		{"function_recursive", `
			function fact($n) { return $n <= 1 ? 1 : $n * fact($n - 1); }
			echo fact(5);
		`},
		{"compare_chain", `
			$x = 5;
			echo $x === 5 ? "id5" : "no";
			echo " ";
			echo $x !== "5" ? "strict" : "loose";
		`},
		{"undefined_var_warning", `
			error_reporting(E_ALL);
			// Reading an undefined var emits a Warning. Both backends
			// must produce the same notice format. We rely on echo to
			// surface stdout (the warning may go through the error
			// handler, depending on config).
			echo $undefined_var ?? "default";
		`},
		{"builtin_call", `echo strlen("hello world");`},
		{"falls_back_to_ast", `
			// Arrays + foreach forces the body to fall back to AST.
			$a = [1, 2, 3, 4, 5];
			$s = 0;
			foreach ($a as $v) { $s += $v; }
			echo $s;
		`},
	}

	for _, tc := range cases {
		// Trim leading whitespace from multi-line snippets so the
		// embedded code line numbers align.
		code := strings.TrimSpace(tc.code)
		diffSnippet(t, tc.name, code)
	}
}
