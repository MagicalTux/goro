package vm

// Op is a single-byte opcode identifier.
type Op uint8

// Opcodes for the MVP scope (literals, vars, arith, compare, jumps, ret).
// Function-call opcodes are added in a follow-up commit.
const (
	OpNop Op = iota

	// --- load / store ----------------------------------------------------
	OpLoadConst       // push CachedZ[A]  (scalar literal, A = const-pool index)
	OpLoadNull        // push ZNULL.ZVal()
	OpLoadTrue        // push ZTrue.ZVal()
	OpLoadFalse       // push ZFalse.ZVal()
	OpLoadLocal       // push ctx.OffsetGet(Locals[A])
	OpLoadLocalOrWarn // same but emit "Undefined variable" notice if missing
	OpStoreLocal      // pop -> ctx.OffsetSet(Locals[A], _)
	OpStoreLocalKeep  // top -> ctx.OffsetSet(Locals[A], _) (leaves value on stack)
	OpDup             // duplicate top of stack
	OpPop             // discard top of stack

	// --- arithmetic / bitwise (call OperatorMath / OperatorMathLogic) ----
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpPow
	OpBitAnd
	OpBitOr
	OpBitXor
	OpShl
	OpShr
	OpNeg    // unary -
	OpBitNot // unary ~
	OpConcat // .

	// --- compare ---------------------------------------------------------
	OpCmpEq
	OpCmpNe
	OpCmpId  // ===
	OpCmpNid // !==
	OpCmpLt
	OpCmpLe
	OpCmpGt
	OpCmpGe
	OpCmpSpaceship // <=>

	// --- logical ---------------------------------------------------------
	OpNot  // unary !
	OpBool // cast top to bool

	// --- inc/dec on locals (call DoInc) ---------------------------------
	OpIncLocal     // pre-increment local A; pushes new value
	OpDecLocal     // pre-decrement local A; pushes new value
	OpPostIncLocal // post-increment local A; pushes OLD value
	OpPostDecLocal // post-decrement local A; pushes OLD value

	// --- compound assign (B = tokenizer.ItemType, A = local idx) --------
	// Stack: pops RHS; reads-modifies-writes Locals[A] using the op named by B.
	OpOpAssignLocal

	// --- control flow (signed C is a relative jump in instructions) -----
	OpJmp             // pc += C
	OpJmpIfFalse      // pop; if !v.AsBool() pc += C
	OpJmpIfTrue       // pop; if v.AsBool() pc += C
	OpJmpIfFalsePeek  // peek; if !v.AsBool() pc += C  (used by ||)
	OpJmpIfTruePeek   // peek; if v.AsBool() pc += C   (used by &&)

	// --- call / return ---------------------------------------------------
	// (CALL opcodes are reserved here so emitter constants don't shift
	// when they're implemented; the dispatcher returns ErrUnknownOp until
	// then so accidental emission fails loudly.)
	OpCallUser   // A = const-pool ZString name, B = argc
	OpCallDirect // A = SubFns[A], B = argc
	OpRet        // pop, exit frame with value
	OpRetNull    // exit frame with null

	// --- diagnostics -----------------------------------------------------
	OpTick // call ctx.Tick(ctx, LocAt(pc-1)) and DrainTempObjects

	// Sentinel — keep last.
	opLast
)

// String returns the symbolic name of the opcode (for disassembly).
func (o Op) String() string {
	if int(o) < len(opNames) {
		if s := opNames[o]; s != "" {
			return s
		}
	}
	return "OP?"
}

var opNames = [...]string{
	OpNop:             "NOP",
	OpLoadConst:       "LOAD_CONST",
	OpLoadNull:        "LOAD_NULL",
	OpLoadTrue:        "LOAD_TRUE",
	OpLoadFalse:       "LOAD_FALSE",
	OpLoadLocal:       "LOAD_LOCAL",
	OpLoadLocalOrWarn: "LOAD_LOCAL_OR_WARN",
	OpStoreLocal:      "STORE_LOCAL",
	OpStoreLocalKeep:  "STORE_LOCAL_KEEP",
	OpDup:             "DUP",
	OpPop:             "POP",
	OpAdd:             "ADD",
	OpSub:             "SUB",
	OpMul:             "MUL",
	OpDiv:             "DIV",
	OpMod:             "MOD",
	OpPow:             "POW",
	OpBitAnd:          "BITAND",
	OpBitOr:           "BITOR",
	OpBitXor:          "BITXOR",
	OpShl:             "SHL",
	OpShr:             "SHR",
	OpNeg:             "NEG",
	OpBitNot:          "BITNOT",
	OpConcat:          "CONCAT",
	OpCmpEq:           "CMP_EQ",
	OpCmpNe:           "CMP_NE",
	OpCmpId:           "CMP_ID",
	OpCmpNid:          "CMP_NID",
	OpCmpLt:           "CMP_LT",
	OpCmpLe:           "CMP_LE",
	OpCmpGt:           "CMP_GT",
	OpCmpGe:           "CMP_GE",
	OpCmpSpaceship:    "CMP_SPACESHIP",
	OpNot:             "NOT",
	OpBool:            "BOOL",
	OpIncLocal:        "INC_LOCAL",
	OpDecLocal:        "DEC_LOCAL",
	OpPostIncLocal:    "POSTINC_LOCAL",
	OpPostDecLocal:    "POSTDEC_LOCAL",
	OpOpAssignLocal:   "OP_ASSIGN_LOCAL",
	OpJmp:             "JMP",
	OpJmpIfFalse:      "JMP_IF_FALSE",
	OpJmpIfTrue:       "JMP_IF_TRUE",
	OpJmpIfFalsePeek:  "JMP_IF_FALSE_PEEK",
	OpJmpIfTruePeek:   "JMP_IF_TRUE_PEEK",
	OpCallUser:        "CALL_USER",
	OpCallDirect:      "CALL_DIRECT",
	OpRet:             "RET",
	OpRetNull:         "RET_NULL",
	OpTick:            "TICK",
}
