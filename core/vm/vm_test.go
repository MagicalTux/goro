package vm_test

import (
	"context"
	"strings"
	"testing"

	"github.com/KarpelesLab/goro/core/ini"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
	"github.com/KarpelesLab/goro/core/vm"
	_ "github.com/KarpelesLab/goro/ext/standard"
)

// makeCtx builds a fresh phpv.Context suitable for executing a vm.Function.
// Uses the cli SAPI so we don't depend on any specific extension.
func makeCtx(t *testing.T) phpv.Context {
	t.Helper()
	p := phpctx.NewProcess("cli")
	cfg := ini.New()
	g := phpctx.NewGlobal(context.Background(), p, cfg)
	return g
}

// internLoc is a placeholder used by tests where a real *phpv.Loc isn't
// otherwise available — the dispatcher only needs Source to be non-nil
// for error wrapping, never reads its contents on the happy path.
var internLoc = &phpv.Loc{Filename: "<vm-test>", Line: 1, Char: 1}

// makeFunction creates a *vm.Function from raw fields with sensible
// defaults (Source set, MaxStack defaulted to len(code)*2).
func makeFunction(name string, code []vm.Instruction, consts []phpv.Val, locals []phpv.ZString) *vm.Function {
	cached := make([]*phpv.ZVal, len(consts))
	for i, c := range consts {
		cached[i] = phpv.MakeCachedZVal(c)
	}
	maxStack := len(code) * 2
	if maxStack < 4 {
		maxStack = 4
	}
	return &vm.Function{
		Code:     code,
		Consts:   consts,
		CachedZ:  cached,
		Locals:   locals,
		Source:   internLoc,
		Name:     phpv.ZString(name),
		MaxStack: maxStack,
	}
}

func TestArithmetic(t *testing.T) {
	cases := []struct {
		name string
		op   vm.Op
		a, b phpv.Val
		want string // string repr of expected ZVal
	}{
		{"int+int", vm.OpAdd, phpv.ZInt(40), phpv.ZInt(2), "42"},
		{"int-int", vm.OpSub, phpv.ZInt(50), phpv.ZInt(8), "42"},
		{"int*int", vm.OpMul, phpv.ZInt(6), phpv.ZInt(7), "42"},
		{"int/int", vm.OpDiv, phpv.ZInt(84), phpv.ZInt(2), "42"},
		{"int%int", vm.OpMod, phpv.ZInt(85), phpv.ZInt(43), "42"},
		{"concat", vm.OpConcat, phpv.ZString("hello "), phpv.ZString("world"), "hello world"},
		{"bitand", vm.OpBitAnd, phpv.ZInt(0xff), phpv.ZInt(0x0f), "15"},
		{"bitor", vm.OpBitOr, phpv.ZInt(0xf0), phpv.ZInt(0x0f), "255"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn := makeFunction("test", []vm.Instruction{
				vm.Encode(vm.OpLoadConst, 0, 0, 0),
				vm.Encode(vm.OpLoadConst, 1, 0, 0),
				vm.Encode(tc.op, 0, 0, 0),
				vm.Encode(vm.OpRet, 0, 0, 0),
			}, []phpv.Val{tc.a, tc.b}, nil)

			ctx := makeCtx(t)
			got, err := vm.Run(ctx, fn)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if s := got.String(); s != tc.want {
				t.Errorf("got %q, want %q", s, tc.want)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		name string
		op   vm.Op
		a, b phpv.Val
		want bool
	}{
		{"eq-true", vm.OpCmpEq, phpv.ZInt(42), phpv.ZInt(42), true},
		{"eq-loose", vm.OpCmpEq, phpv.ZInt(1), phpv.ZString("1"), true},
		{"id-strict", vm.OpCmpId, phpv.ZInt(1), phpv.ZString("1"), false},
		{"lt", vm.OpCmpLt, phpv.ZInt(1), phpv.ZInt(2), true},
		{"gt", vm.OpCmpGt, phpv.ZInt(1), phpv.ZInt(2), false},
		{"ne", vm.OpCmpNe, phpv.ZInt(1), phpv.ZInt(2), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn := makeFunction("test", []vm.Instruction{
				vm.Encode(vm.OpLoadConst, 0, 0, 0),
				vm.Encode(vm.OpLoadConst, 1, 0, 0),
				vm.Encode(tc.op, 0, 0, 0),
				vm.Encode(vm.OpRet, 0, 0, 0),
			}, []phpv.Val{tc.a, tc.b}, nil)

			ctx := makeCtx(t)
			got, err := vm.Run(ctx, fn)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if bool(got.AsBool(ctx)) != tc.want {
				t.Errorf("got %v, want %v", bool(got.AsBool(ctx)), tc.want)
			}
		})
	}
}

func TestLocalStoreAndLoad(t *testing.T) {
	// $x = 7; $y = 35; return $x + $y;
	code := []vm.Instruction{
		vm.Encode(vm.OpLoadConst, 0, 0, 0),
		vm.Encode(vm.OpStoreLocal, 0, 0, 0), // $x = 7
		vm.Encode(vm.OpLoadConst, 1, 0, 0),
		vm.Encode(vm.OpStoreLocal, 1, 0, 0), // $y = 35
		vm.Encode(vm.OpLoadLocal, 0, 0, 0),  // push $x
		vm.Encode(vm.OpLoadLocal, 1, 0, 0),  // push $y
		vm.Encode(vm.OpAdd, 0, 0, 0),
		vm.Encode(vm.OpRet, 0, 0, 0),
	}
	fn := makeFunction("test", code,
		[]phpv.Val{phpv.ZInt(7), phpv.ZInt(35)},
		[]phpv.ZString{"x", "y"},
	)

	ctx := makeCtx(t)
	got, err := vm.Run(ctx, fn)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.String() != "42" {
		t.Errorf("got %q, want 42", got.String())
	}
}

func TestIncLocalLoop(t *testing.T) {
	// $sum = 0; $i = 0; while ($i < 5) { $sum = $sum + $i; ++$i; } return $sum;
	//
	// Layout — relative jump C semantics: after fetch f.pc has already
	// been incremented past the jump, so pc += C lands at target where
	// C = target - (jumpPC + 1).
	//
	//   pc=0: LOAD_CONST  0  (push 0)
	//   pc=1: STORE_LOCAL $sum
	//   pc=2: LOAD_CONST  0  (push 0)
	//   pc=3: STORE_LOCAL $i
	//   pc=4: LOAD_LOCAL  $i        ; loop head
	//   pc=5: LOAD_CONST  1  (push 5)
	//   pc=6: CMP_LT
	//   pc=7: JMP_IF_FALSE -> pc=14 (C = 14 - 8 = 6)
	//   pc=8: LOAD_LOCAL  $sum
	//   pc=9: LOAD_LOCAL  $i
	//   pc=10: ADD
	//   pc=11: STORE_LOCAL $sum
	//   pc=12: INC_LOCAL $i (pushes new)
	//   pc=13: POP
	//   pc=14? NO — JMP back to pc=4: but there's no slot 14 yet. Re-layout.
	//
	// Re-layout:
	//   pc=0..3: init
	//   pc=4: LOAD_LOCAL $i
	//   pc=5: LOAD_CONST 5
	//   pc=6: CMP_LT
	//   pc=7: JMP_IF_FALSE -> pc=14   (C = 14 - 8 = 6)
	//   pc=8: LOAD_LOCAL $sum
	//   pc=9: LOAD_LOCAL $i
	//   pc=10: ADD
	//   pc=11: STORE_LOCAL $sum
	//   pc=12: INC_LOCAL $i
	//   pc=13: POP
	//   pc=14: ... but we want to jump back to pc=4 first, so reorder
	//
	// Final layout:
	//   pc=0..3: init  (LOAD 0 / STORE sum / LOAD 0 / STORE i)
	//   pc=4..7: cond  (LOAD i / LOAD 5 / CMP_LT / JMPF to exit)
	//   pc=8..13: body (LOAD sum / LOAD i / ADD / STORE sum / INC i / POP)
	//   pc=14: JMP back to pc=4   (C = 4 - 15 = -11)
	//   pc=15: LOAD_LOCAL $sum
	//   pc=16: RET
	// Exit target for JMPF at pc=7: pc=15  (C = 15 - 8 = 7)
	zeroIdx := uint16(2) // const-pool index for ZInt(0)
	code := []vm.Instruction{
		vm.Encode(vm.OpLoadConst, zeroIdx, 0, 0), // 0
		vm.Encode(vm.OpStoreLocal, 0, 0, 0),      // 1
		vm.Encode(vm.OpLoadConst, zeroIdx, 0, 0), // 2
		vm.Encode(vm.OpStoreLocal, 1, 0, 0),      // 3
		vm.Encode(vm.OpLoadLocal, 1, 0, 0),       // 4
		vm.Encode(vm.OpLoadConst, 0, 0, 0),       // 5  (5)
		vm.Encode(vm.OpCmpLt, 0, 0, 0),           // 6
		vm.Encode(vm.OpJmpIfFalse, 0, 0, 7),      // 7  -> 15
		vm.Encode(vm.OpLoadLocal, 0, 0, 0),       // 8
		vm.Encode(vm.OpLoadLocal, 1, 0, 0),       // 9
		vm.Encode(vm.OpAdd, 0, 0, 0),             // 10
		vm.Encode(vm.OpStoreLocal, 0, 0, 0),      // 11
		vm.Encode(vm.OpIncLocal, 1, 0, 0),        // 12
		vm.Encode(vm.OpPop, 0, 0, 0),             // 13
		vm.Encode(vm.OpJmp, 0, 0, -11),           // 14 -> 4
		vm.Encode(vm.OpLoadLocal, 0, 0, 0),       // 15
		vm.Encode(vm.OpRet, 0, 0, 0),             // 16
	}
	fn := makeFunction("loop", code,
		[]phpv.Val{phpv.ZInt(5), phpv.ZInt(1) /*unused*/, phpv.ZInt(0)},
		[]phpv.ZString{"sum", "i"},
	)
	fn.MaxStack = 4

	ctx := makeCtx(t)
	got, err := vm.Run(ctx, fn)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.String() != "10" {
		t.Errorf("expected sum=10 (0+1+2+3+4), got %q", got.String())
	}
}

func TestCompoundAssign(t *testing.T) {
	// $x = 10; $x += 32; return $x;
	code := []vm.Instruction{
		vm.Encode(vm.OpLoadConst, 0, 0, 0),                                       // push 10
		vm.Encode(vm.OpStoreLocal, 0, 0, 0),                                      // $x = 10
		vm.Encode(vm.OpLoadConst, 1, 0, 0),                                       // push 32
		vm.Encode(vm.OpOpAssignLocal, 0, uint16(tokenizer.T_PLUS_EQUAL), 0),      // $x += pop
		vm.Encode(vm.OpLoadLocal, 0, 0, 0),                                       // push $x
		vm.Encode(vm.OpRet, 0, 0, 0),
	}
	fn := makeFunction("compound", code,
		[]phpv.Val{phpv.ZInt(10), phpv.ZInt(32)},
		[]phpv.ZString{"x"},
	)

	ctx := makeCtx(t)
	got, err := vm.Run(ctx, fn)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.String() != "42" {
		t.Errorf("got %q, want 42", got.String())
	}
}

func TestRetNullByDefault(t *testing.T) {
	fn := makeFunction("retnull", []vm.Instruction{
		vm.Encode(vm.OpRetNull, 0, 0, 0),
	}, nil, nil)

	ctx := makeCtx(t)
	got, err := vm.Run(ctx, fn)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.GetType() != phpv.ZtNull {
		t.Errorf("expected null, got %v", got.GetType())
	}
}

func TestInstructionEncoding(t *testing.T) {
	// Round-trip every field to catch sign-ext / packing bugs.
	cases := []struct {
		op   vm.Op
		a, b uint16
		c    int32
	}{
		{vm.OpLoadConst, 0, 0, 0},
		{vm.OpJmp, 0, 0, -11},
		{vm.OpJmp, 0, 0, 0x7FFFFF},  // max
		{vm.OpJmp, 0, 0, -0x800000}, // min
		{vm.OpAdd, 0xFFFF, 0xFFFE, 1},
	}
	for _, tc := range cases {
		ins := vm.Encode(tc.op, tc.a, tc.b, tc.c)
		if ins.Op() != tc.op {
			t.Errorf("op: got %v, want %v", ins.Op(), tc.op)
		}
		if ins.A() != tc.a {
			t.Errorf("A: got %d, want %d", ins.A(), tc.a)
		}
		if ins.B() != tc.b {
			t.Errorf("B: got %d, want %d", ins.B(), tc.b)
		}
		if ins.C() != tc.c {
			t.Errorf("C: got %d, want %d", ins.C(), tc.c)
		}
	}
}

func TestDumpFunction(t *testing.T) {
	fn := makeFunction("test", []vm.Instruction{
		vm.Encode(vm.OpLoadConst, 0, 0, 0),
		vm.Encode(vm.OpRet, 0, 0, 0),
	}, []phpv.Val{phpv.ZInt(7)}, nil)

	var sb strings.Builder
	if err := vm.DumpFunction(&sb, fn); err != nil {
		t.Fatalf("DumpFunction: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "LOAD_CONST") {
		t.Errorf("disasm missing LOAD_CONST: %s", out)
	}
	if !strings.Contains(out, "RET") {
		t.Errorf("disasm missing RET: %s", out)
	}
}
