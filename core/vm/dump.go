package vm

import (
	"fmt"
	"io"

	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

// DumpFunction writes a human-readable disassembly of fn to w.
func DumpFunction(w io.Writer, fn *Function) error {
	if _, err := fmt.Fprintf(w, "; function %q (params=%d, maxstack=%d, code=%d insts)\n",
		string(fn.Name), fn.NumParams, fn.MaxStack, len(fn.Code)); err != nil {
		return err
	}
	for pc, ins := range fn.Code {
		if err := writeInstr(w, fn, uint32(pc), ins); err != nil {
			return err
		}
	}
	return nil
}

func writeInstr(w io.Writer, fn *Function, pc uint32, ins Instruction) error {
	op := ins.Op()
	a, b, c := ins.A(), ins.B(), ins.C()

	var note string
	switch op {
	case OpLoadConst:
		if int(a) < len(fn.Consts) {
			note = describeVal(fn.Consts[a])
		}
	case OpLoadLocal, OpLoadLocalOrWarn, OpStoreLocal, OpStoreLocalKeep,
		OpIncLocal, OpDecLocal, OpPostIncLocal, OpPostDecLocal:
		if int(a) < len(fn.Locals) {
			note = "$" + string(fn.Locals[a])
		}
	case OpOpAssignLocal:
		name := ""
		if int(a) < len(fn.Locals) {
			name = "$" + string(fn.Locals[a])
		}
		note = fmt.Sprintf("%s %s", name, tokenizer.ItemType(b))
	case OpJmp, OpJmpIfFalse, OpJmpIfTrue, OpJmpIfFalsePeek, OpJmpIfTruePeek:
		note = fmt.Sprintf("-> %d", int32(pc+1)+c)
	case OpCallUser:
		if int(a) < len(fn.Consts) {
			note = fmt.Sprintf("%s argc=%d", describeVal(fn.Consts[a]), b)
		}
	case OpCallDirect:
		if int(a) < len(fn.SubFns) && fn.SubFns[a] != nil {
			note = fmt.Sprintf("%s argc=%d", string(fn.SubFns[a].Name), b)
		}
	}

	_, err := fmt.Fprintf(w, "  %04d  %-18s a=%d b=%d c=%d   %s\n", pc, op.String(), a, b, c, note)
	return err
}

func describeVal(v phpv.Val) string {
	if v == nil {
		return "<nil>"
	}
	switch x := v.(type) {
	case phpv.ZString:
		return fmt.Sprintf("%q", string(x))
	case phpv.ZInt:
		return fmt.Sprintf("%d", int64(x))
	case phpv.ZFloat:
		return fmt.Sprintf("%g", float64(x))
	case phpv.ZBool:
		if bool(x) {
			return "true"
		}
		return "false"
	case phpv.ZNull:
		return "null"
	}
	return fmt.Sprintf("%T", v)
}
