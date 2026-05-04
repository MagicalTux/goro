package vm

// Instruction is a fixed-width 64-bit bytecode word.
//
// Layout (high to low):
//
//	bits 63..56 (8)  Op
//	bits 55..40 (16) A
//	bits 39..24 (16) B
//	bits 23..0  (24) C  (signed, sign-extended on read)
//
// 16-bit A/B operands cover both stack-style ops (mostly zero) and
// register-style ops up to 65535 distinct slots. The 24-bit signed C
// field gives ±8M jump range (more than any plausible function will
// need) and doubles as an immediate for small int constants.
//
// When an opcode needs a wider single index (e.g. const-pool index
// up to 4G), use ABx() / EncodeABx() to read/write A and B as one
// 32-bit unsigned field.
type Instruction uint64

// Encode builds an Instruction from its fields. c is sign-clipped to
// 24 bits; values outside [-(1<<23), (1<<23)-1] cause a panic so emitter
// bugs surface immediately rather than corrupting jump targets.
func Encode(op Op, a, b uint16, c int32) Instruction {
	if c < -(1<<23) || c >= (1<<23) {
		panic("vm: instruction C field out of range")
	}
	return Instruction(uint64(op))<<56 |
		Instruction(a)<<40 |
		Instruction(b)<<24 |
		Instruction(uint32(c)&0xFFFFFF)
}

// EncodeABx builds an Instruction packing a 32-bit unsigned value into
// the A and B fields together (A holds the high 16 bits, B the low 16).
// Used when an opcode needs a single wide index.
func EncodeABx(op Op, abx uint32, c int32) Instruction {
	a := uint16(abx >> 16)
	b := uint16(abx & 0xFFFF)
	return Encode(op, a, b, c)
}

// Op returns the opcode.
func (i Instruction) Op() Op { return Op(uint64(i) >> 56) }

// A returns the A operand (16 bits unsigned).
func (i Instruction) A() uint16 { return uint16(uint64(i) >> 40) }

// B returns the B operand (16 bits unsigned).
func (i Instruction) B() uint16 { return uint16(uint64(i) >> 24) }

// C returns the C operand sign-extended from 24 bits.
func (i Instruction) C() int32 {
	c := int32(uint64(i) & 0xFFFFFF)
	if c&(1<<23) != 0 {
		c |= ^int32(0xFFFFFF) // sign extend
	}
	return c
}

// ABx returns A and B combined as one 32-bit unsigned value (A high,
// B low). Use for opcodes that needed a wider single index.
func (i Instruction) ABx() uint32 {
	return uint32(uint64(i)>>24) & 0xFFFFFFFF
}
