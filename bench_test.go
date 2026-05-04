package main_test

import (
	"context"
	"testing"

	"github.com/KarpelesLab/goro/core/ini"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/vm/vmcompiler"
	_ "github.com/KarpelesLab/goro/ext/standard"
)

func benchPHP(b *testing.B, code string) {
	p := phpctx.NewProcess("cli")
	for b.Loop() {
		cfg := ini.New()
		ctx := phpctx.NewGlobal(context.Background(), p, cfg)
		ctx.DoString(ctx, phpv.ZString(code))
	}
}

// runBoth runs the same workload under both backends as sub-benchmarks
// (one with the VM gate off, one with it on). The sub-benchmark names
// ("ast" / "vm") show in `go test -bench=...` output.
func runBoth(b *testing.B, code string) {
	prev := vmcompiler.Enabled()
	b.Cleanup(func() { vmcompiler.SetEnabled(prev) })

	b.Run("ast", func(b *testing.B) {
		vmcompiler.SetEnabled(false)
		benchPHP(b, code)
	})
	b.Run("vm", func(b *testing.B) {
		vmcompiler.SetEnabled(true)
		benchPHP(b, code)
	})
}

func BenchmarkArithmetic(b *testing.B) {
	runBoth(b, `$sum = 0; for ($i = 0; $i < 100000; $i++) { $sum = $sum + $i; }`)
}

func BenchmarkFunctionCalls(b *testing.B) {
	runBoth(b, `function bench_add($a, $b) { return $a + $b; } $sum = 0; for ($i = 0; $i < 10000; $i++) { $sum = bench_add($sum, $i); }`)
}

func BenchmarkFibonacci(b *testing.B) {
	runBoth(b, `function bench_fib($n) { if ($n <= 1) return $n; return bench_fib($n - 1) + bench_fib($n - 2); } bench_fib(20);`)
}

func BenchmarkArrayOps(b *testing.B) {
	runBoth(b, `$arr = []; for ($i = 0; $i < 10000; $i++) { $arr[] = $i; } $sum = 0; foreach ($arr as $v) { $sum += $v; }`)
}

func BenchmarkStringConcat(b *testing.B) {
	runBoth(b, `$str = ""; for ($i = 0; $i < 10000; $i++) { $str .= "x"; }`)
}
