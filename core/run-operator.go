package core

import (
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

type runOperator struct {
	op  tokenizer.ItemType
	opD *operatorInternalDetails

	a, b Runnable
	l    *Loc
}

type operatorInternalDetails struct {
	write   bool
	numeric bool
	skipA   bool
	op      func(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error)
	pri     int
}

// ?: pri=24
var operatorList = map[tokenizer.ItemType]*operatorInternalDetails{
	tokenizer.ItemSingleChar('='):   &operatorInternalDetails{write: true, skipA: true, pri: 25},
	tokenizer.T_CONCAT_EQUAL:        &operatorInternalDetails{write: true, op: operatorAppend, pri: 25},
	tokenizer.T_DIV_EQUAL:           &operatorInternalDetails{write: true, numeric: true, op: operatorMath, pri: 25},
	tokenizer.T_MUL_EQUAL:           &operatorInternalDetails{write: true, numeric: true, op: operatorMath, pri: 25},
	tokenizer.T_POW_EQUAL:           &operatorInternalDetails{write: true, numeric: true, op: operatorMath, pri: 25},
	tokenizer.T_MINUS_EQUAL:         &operatorInternalDetails{write: true, numeric: true, op: operatorMath, pri: 25},
	tokenizer.T_PLUS_EQUAL:          &operatorInternalDetails{write: true, numeric: true, op: operatorMath, pri: 25},
	tokenizer.ItemSingleChar('.'):   &operatorInternalDetails{op: operatorAppend, pri: 14},
	tokenizer.ItemSingleChar('+'):   &operatorInternalDetails{numeric: true, op: operatorMath, pri: 14},
	tokenizer.ItemSingleChar('-'):   &operatorInternalDetails{numeric: true, op: operatorMath, pri: 14},
	tokenizer.ItemSingleChar('/'):   &operatorInternalDetails{numeric: true, op: operatorMath, pri: 13},
	tokenizer.ItemSingleChar('*'):   &operatorInternalDetails{numeric: true, op: operatorMath, pri: 13},
	tokenizer.T_POW:                 &operatorInternalDetails{numeric: true, op: operatorMath, pri: 10},
	tokenizer.T_OR_EQUAL:            &operatorInternalDetails{write: true, numeric: true, op: operatorMathLogic, pri: 25},
	tokenizer.T_XOR_EQUAL:           &operatorInternalDetails{write: true, numeric: true, op: operatorMathLogic, pri: 25},
	tokenizer.T_AND_EQUAL:           &operatorInternalDetails{write: true, numeric: true, op: operatorMathLogic, pri: 25},
	tokenizer.T_MOD_EQUAL:           &operatorInternalDetails{write: true, numeric: true, op: operatorMathLogic, pri: 25},
	tokenizer.ItemSingleChar('|'):   &operatorInternalDetails{op: operatorMathLogic, pri: 20},
	tokenizer.ItemSingleChar('^'):   &operatorInternalDetails{op: operatorMathLogic, pri: 19},
	tokenizer.ItemSingleChar('&'):   &operatorInternalDetails{op: operatorMathLogic, pri: 18},
	tokenizer.ItemSingleChar('%'):   &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 13},
	tokenizer.ItemSingleChar('~'):   &operatorInternalDetails{op: operatorMathLogic, pri: 11},
	tokenizer.T_SL:                  &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 15},
	tokenizer.T_SR:                  &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 15},
	tokenizer.T_LOGICAL_AND:         &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 26},
	tokenizer.T_LOGICAL_XOR:         &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 27},
	tokenizer.T_LOGICAL_OR:          &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 28},
	tokenizer.T_SL_EQUAL:            &operatorInternalDetails{write: true, numeric: true, op: operatorMathLogic, pri: 25},
	tokenizer.T_SR_EQUAL:            &operatorInternalDetails{write: true, numeric: true, op: operatorMathLogic, pri: 25},
	tokenizer.ItemSingleChar('<'):   &operatorInternalDetails{op: operatorCompare, pri: 16},
	tokenizer.ItemSingleChar('>'):   &operatorInternalDetails{op: operatorCompare, pri: 16},
	tokenizer.T_IS_SMALLER_OR_EQUAL: &operatorInternalDetails{op: operatorCompare, pri: 16},
	tokenizer.T_IS_GREATER_OR_EQUAL: &operatorInternalDetails{op: operatorCompare, pri: 16},
	tokenizer.T_IS_EQUAL:            &operatorInternalDetails{op: operatorCompare, pri: 17},
	tokenizer.T_IS_IDENTICAL:        &operatorInternalDetails{op: operatorCompareStrict, pri: 17},
	tokenizer.T_IS_NOT_EQUAL:        &operatorInternalDetails{op: operatorCompare, pri: 17},
	tokenizer.T_SPACESHIP:           &operatorInternalDetails{op: operatorCompare, pri: 17},
	tokenizer.T_IS_NOT_IDENTICAL:    &operatorInternalDetails{op: operatorCompareStrict, pri: 17},
	tokenizer.ItemSingleChar('!'):   &operatorInternalDetails{op: operatorNot, pri: 12},
	tokenizer.T_BOOLEAN_AND:         &operatorInternalDetails{op: operatorBoolLogic, pri: 21},
	tokenizer.T_BOOLEAN_OR:          &operatorInternalDetails{op: operatorBoolLogic, pri: 22},
	tokenizer.T_COALESCE:            &operatorInternalDetails{pri: 23}, // TODO
	tokenizer.T_INC:                 &operatorInternalDetails{op: operatorIncDec, pri: 11},
	tokenizer.T_DEC:                 &operatorInternalDetails{op: operatorIncDec, pri: 11},
	tokenizer.ItemSingleChar('@'):   &operatorInternalDetails{pri: 11}, // TODO

	// cast operators
	tokenizer.T_BOOL_CAST:   &operatorInternalDetails{op: func(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) { return b.As(ctx, ZtBool) }, pri: 11},
	tokenizer.T_INT_CAST:    &operatorInternalDetails{op: func(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) { return b.As(ctx, ZtInt) }, pri: 11},
	tokenizer.T_DOUBLE_CAST: &operatorInternalDetails{op: func(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) { return b.As(ctx, ZtFloat) }, pri: 11},
	tokenizer.T_ARRAY_CAST:  &operatorInternalDetails{op: func(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) { return b.As(ctx, ZtArray) }, pri: 11},
	tokenizer.T_OBJECT_CAST: &operatorInternalDetails{op: func(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) { return b.As(ctx, ZtObject) }, pri: 11},
	tokenizer.T_STRING_CAST: &operatorInternalDetails{op: func(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) { return b.As(ctx, ZtString) }, pri: 11},
}

func (r *runOperator) Loc() *Loc {
	return r.l
}

func isOperator(t tokenizer.ItemType) bool {
	_, ok := operatorList[t]
	return ok
}

func (r *runOperator) Dump(w io.Writer) error {
	_, err := w.Write([]byte{'('})
	if err != nil {
		return err
	}
	if r.a != nil {
		err = r.a.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte(r.op.Name())) // TODO fixme
	if err != nil {
		return err
	}
	if r.b != nil {
		err = r.b.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte{')'})
	return err
}

func spawnOperator(op tokenizer.ItemType, a, b Runnable, l *Loc) (Runnable, error) {
	var err error
	opD, ok := operatorList[op]
	if !ok {
		return nil, l.Errorf(nil, E_COMPILE_ERROR, "invalid operator %s", op)
	}

	//log.Printf("spawn operator %s %s %s", debugDump(a), op.Name(), debugDump(b))
	if rop, isop := a.(*runOperator); isop {
		if opD.pri < rop.opD.pri {
			// need to go down one level values
			rop.b, err = spawnOperator(op, rop.b, b, l)
			if err != nil {
				return nil, err
			}
			//rop.b = &runOperator{op: op, opD: opD, a: rop.b, b: b, l: l}
			//log.Printf("did swap(a), res = %s", debugDump(rop))
			return rop, nil
		}
	}
	final := &runOperator{op: op, opD: opD, a: a, b: b, l: l}
	//log.Printf("spawn operator: %s", debugDump(final))
	return final, nil
}

func (r *runOperator) Run(ctx Context) (*ZVal, error) {
	var a, b, res *ZVal
	var err error

	op := r.opD

	if r.op == tokenizer.ItemSingleChar('@') {
		// silence errors
		ctx = WithConfig(ctx, "error_reporting", ZInt(0).ZVal())
	}

	// read a and b
	if r.a != nil {
		a, err = r.a.Run(ctx)
		if err != nil {
			return nil, err
		}
	}

	if r.b != nil {
		b, err = r.b.Run(ctx)
		if err != nil {
			return nil, err
		}
	}

	if op.numeric {
		a, _ = a.AsNumeric(ctx)
		b, _ = b.AsNumeric(ctx)

		// normalize types
		if a.GetType() == ZtFloat || b.GetType() == ZtFloat {
			a, _ = a.As(ctx, ZtFloat)
			b, _ = b.As(ctx, ZtFloat)
		} else {
			a, _ = a.As(ctx, ZtInt)
			b, _ = b.As(ctx, ZtInt)
		}
	}

	if op.op != nil {
		res, err = op.op(ctx, r.op, a, b)
		if err != nil {
			return nil, err
		}
	} else {
		res = b
	}

	if op.write {
		w, ok := r.a.(Writable)
		if !ok {
			return nil, fmt.Errorf("Can't use %#v value in write context", r.a)
		}
		return res, w.WriteValue(ctx, res)
	}

	return res, nil
}

func operatorAppend(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) {
	a, _ = a.As(ctx, ZtString)
	b, _ = b.As(ctx, ZtString)

	return &ZVal{a.AsString(ctx) + b.AsString(ctx)}, nil
}

func operatorNot(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) {
	b, _ = b.As(ctx, ZtBool)

	return &ZVal{!b.Value().(ZBool)}, nil
}

func doInc(v *ZVal, inc bool) error {
	switch v.GetType() {
	case ZtNull:
		if inc {
			v.Set(ZInt(1).ZVal())
		}
		return nil
	case ZtBool:
		return nil
	case ZtInt:
		n := v.Value().(ZInt)
		if inc {
			if n == math.MaxInt64 {
				v.Set((ZFloat(n) + 1).ZVal())
				return nil
			}
			n += 1
		} else {
			if n == math.MinInt64 {
				v.Set((ZFloat(n) - 1).ZVal())
				return nil
			}
			n -= 1
		}
		v.Set(n.ZVal())
		return nil
	case ZtFloat:
		n := v.Value().(ZFloat)
		if inc {
			n += 1
		} else {
			n -= 1
		}
		v.Set(n.ZVal())
		return nil
	case ZtString:
		s := v.Value().(ZString)
		// first, check if potentially numeric
		if s.IsNumeric() {
			if x, err := s.AsNumeric(); err == nil {
				v.Set(x.ZVal())
				return doInc(v, inc)
			}
		}

		if !inc {
			// strings can only be incremented
			return nil
		}

		// do string increment...
		var c byte
		n := []byte(s)

		for i := len(n) - 1; i >= 0; i-- {
			c = n[i]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
				if c == 'z' {
					n[i] = 'a'
					continue
				}
				if c == 'Z' {
					n[i] = 'A'
					continue
				}
				if c == '9' {
					n[i] = '0'
					continue
				}
				n[i] = c + 1
				v.Set(ZString(n).ZVal())
				return nil
			}
			v.Set(ZString(n).ZVal())
			return nil
		}

		switch c {
		case '9':
			v.Set(("1" + ZString(n)).ZVal())
			return nil
		case 'z':
			v.Set(("a" + ZString(n)).ZVal())
			return nil
		case 'Z':
			v.Set(("A" + ZString(n)).ZVal())
			return nil
		}
	}
	return fmt.Errorf("unsupported type for increment operator %s", v.GetType())
}

func operatorIncDec(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) {
	inc := op == tokenizer.T_INC

	if a != nil {
		// post mode
		orig := a.Dup()
		return orig, doInc(a, inc)
	} else {
		// pre mode
		return b, doInc(b, inc)
	}
}

func operatorMath(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) {
	switch a.Value().GetType() {
	case ZtInt:
		var res Val
		a := a.Value().(ZInt)
		b := b.Value().(ZInt)

		switch op {
		case tokenizer.T_PLUS_EQUAL, tokenizer.ItemSingleChar('+'):
			c := a + b
			if (c > a) == (b > 0) {
				res = c
			} else {
				// overflow
				res = ZFloat(a) + ZFloat(b)
			}
		case tokenizer.T_MINUS_EQUAL, tokenizer.ItemSingleChar('-'):
			c := a - b
			if (c < a) == (b > 0) {
				res = c
			} else {
				// overflow
				res = ZFloat(a) - ZFloat(b)
			}
		case tokenizer.T_DIV_EQUAL, tokenizer.ItemSingleChar('/'):
			if b == 0 {
				return nil, errors.New("Division by zero")
			}
			if a%b != 0 {
				// this is not goign to be a int result
				res = ZFloat(a) / ZFloat(b)
			} else {
				res = a / b
			}
		case tokenizer.T_MUL_EQUAL, tokenizer.ItemSingleChar('*'):
			if a == 0 || b == 0 {
				res = ZInt(0)
				break
			}
			c := a * b
			// do overflow check (golang has no good way to perform this, so checking if c/b=a will have to do)
			if ((c < 0) == ((a < 0) != (b < 0))) && (c/b == a) {
				res = c
			} else {
				// do this as float
				res = ZFloat(a) * ZFloat(b)
			}
		case tokenizer.T_POW, tokenizer.T_POW_EQUAL:
			res = ZFloat(math.Pow(float64(a), float64(b)))
		}
		return &ZVal{res}, nil
	case ZtFloat:
		var res ZFloat
		switch op {
		case tokenizer.T_PLUS_EQUAL, tokenizer.ItemSingleChar('+'):
			res = a.Value().(ZFloat) + b.Value().(ZFloat)
		case tokenizer.T_MINUS_EQUAL, tokenizer.ItemSingleChar('-'):
			res = a.Value().(ZFloat) - b.Value().(ZFloat)
		case tokenizer.T_DIV_EQUAL, tokenizer.ItemSingleChar('/'):
			res = a.Value().(ZFloat) / b.Value().(ZFloat)
		case tokenizer.T_MUL_EQUAL, tokenizer.ItemSingleChar('*'):
			res = a.Value().(ZFloat) * b.Value().(ZFloat)
		case tokenizer.T_POW, tokenizer.T_POW_EQUAL:
			res = ZFloat(math.Pow(float64(a.Value().(ZFloat)), float64(b.Value().(ZFloat))))
		}
		return &ZVal{res}, nil
	default:
		return nil, fmt.Errorf("todo operator type unsupported %s", a.GetType())
	}
}

func operatorBoolLogic(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) {
	switch op {
	case tokenizer.T_BOOLEAN_AND:
		return (a.AsBool(ctx) && b.AsBool(ctx)).ZVal(), nil
	case tokenizer.T_BOOLEAN_OR:
		return (a.AsBool(ctx) || b.AsBool(ctx)).ZVal(), nil
	default:
		return nil, fmt.Errorf("todo operator unsupported %s", op)
	}
}

func operatorMathLogic(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) {
	if a == nil {
		a = b
	}

	switch a.Value().GetType() {
	case ZtInt:
		b, _ = b.As(ctx, ZtInt)
		var res ZInt
		switch op {
		case tokenizer.ItemSingleChar('|'), tokenizer.T_OR_EQUAL:
			res = a.Value().(ZInt) | b.Value().(ZInt)
		case tokenizer.ItemSingleChar('^'), tokenizer.T_XOR_EQUAL:
			res = a.Value().(ZInt) ^ b.Value().(ZInt)
		case tokenizer.ItemSingleChar('&'), tokenizer.T_AND_EQUAL:
			res = a.Value().(ZInt) & b.Value().(ZInt)
		case tokenizer.ItemSingleChar('%'), tokenizer.T_MOD_EQUAL:
			res = a.Value().(ZInt) % b.Value().(ZInt)
		case tokenizer.ItemSingleChar('~'):
			res = ^b.Value().(ZInt)
		case tokenizer.T_SL, tokenizer.T_SL_EQUAL:
			// TODO error check on negative b
			res = a.Value().(ZInt) << uint(b.Value().(ZInt))
		case tokenizer.T_SR, tokenizer.T_SR_EQUAL:
			// TODO error check on negative b
			res = a.Value().(ZInt) >> uint(b.Value().(ZInt))
		}
		return &ZVal{res}, nil
	case ZtFloat:
		// need to convert to int
		a, _ = a.As(ctx, ZtInt)
		b, _ = b.As(ctx, ZtInt)
		return operatorMathLogic(ctx, op, a, b)
	case ZtString:
		a := []byte(a.Value().(ZString))
		b := []byte(b.Value().(ZString))
		if len(a) != len(b) {
			if len(a) < len(b) {
				a, b = b, a
			}
			// a is longer than b
			switch op {
			case tokenizer.ItemSingleChar('|'), tokenizer.T_OR_EQUAL: // make b longer in this case
				newb := make([]byte, len(a))
				copy(newb, b)
				b = newb
			default:
				a = a[:len(b)]
			}
		}

		switch op {
		case tokenizer.ItemSingleChar('|'), tokenizer.T_OR_EQUAL:
			for i := 0; i < len(a); i++ {
				a[i] |= b[i]
			}
		case tokenizer.ItemSingleChar('^'), tokenizer.T_XOR_EQUAL:
			for i := 0; i < len(a); i++ {
				a[i] ^= b[i]
			}
		case tokenizer.ItemSingleChar('&'), tokenizer.T_AND_EQUAL:
			for i := 0; i < len(a); i++ {
				a[i] &= b[i]
			}
		case tokenizer.ItemSingleChar('~'):
			for i := 0; i < len(a); i++ {
				b[i] = ^b[i]
			}
			a = b
		default:
			return nil, errors.New("todo operator unsupported on strings")
		}
		return &ZVal{ZString(a)}, nil
	default:
		return nil, fmt.Errorf("todo operator type unsupported: %s", a.GetType())
	}
}

func operatorCompareStrict(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) {
	if a.GetType() != b.GetType() {
		// not same type → false
		return &ZVal{ZBool(op != tokenizer.T_IS_IDENTICAL)}, nil
	}

	var res bool

	switch a.GetType() {
	case ZtNull:
		res = true
	case ZtBool:
		res = a.Value().(ZBool) == b.Value().(ZBool)
	case ZtInt:
		res = a.Value().(ZInt) == b.Value().(ZInt)
	case ZtFloat:
		res = a.Value().(ZFloat) == b.Value().(ZFloat)
	case ZtString:
		res = a.Value().(ZString) == b.Value().(ZString)
	default:
		return nil, fmt.Errorf("unsupported compare type %s", a.GetType())
	}

	if op == tokenizer.T_IS_NOT_IDENTICAL {
		res = !res
	}

	return &ZVal{ZBool(res)}, nil
}

func operatorCompare(ctx Context, op tokenizer.ItemType, a, b *ZVal) (*ZVal, error) {
	// operator compare (< > <= >= == === != !== <=>) involve a lot of dark magic in php, unless both values are of the same type (and even so)
	// loose comparison will convert number-y looking strings into numbers, etc
	var ia, ib *ZVal

	switch a.GetType() {
	case ZtInt, ZtFloat:
		ia = a
	case ZtString:
		if a.Value().(ZString).LooksInt() {
			ia, _ = a.As(ctx, ZtInt)
		} else if a.Value().(ZString).IsNumeric() {
			ia, _ = a.As(ctx, ZtFloat)
		}
	}

	switch b.GetType() {
	case ZtInt, ZtFloat:
		ib = b
	case ZtString:
		if b.Value().(ZString).LooksInt() {
			ib, _ = b.As(ctx, ZtInt)
		} else if b.Value().(ZString).IsNumeric() {
			ib, _ = b.As(ctx, ZtFloat)
		}
	}

	if ia != nil || ib != nil {
		// if either part is a numeric, force the other one as numeric too and go through comparison
		if ia == nil {
			ia, _ = a.AsNumeric(ctx)
		}
		if ib == nil {
			ib, _ = b.AsNumeric(ctx)
		}

		// perform numeric comparison
		if ia.GetType() != ib.GetType() {
			// normalize type - at this point as both are numeric, it means either is a float. Make them both float
			ia, _ = ia.As(ctx, ZtFloat)
			ib, _ = ib.As(ctx, ZtFloat)
		}

		var res Val
		switch ia.GetType() {
		case ZtInt:
			ia := ia.Value().(ZInt)
			ib := ib.Value().(ZInt)
			switch op {
			case tokenizer.ItemSingleChar('<'):
				res = ZBool(ia < ib)
			case tokenizer.ItemSingleChar('>'):
				res = ZBool(ia > ib)
			case tokenizer.T_IS_SMALLER_OR_EQUAL:
				res = ZBool(ia <= ib)
			case tokenizer.T_IS_GREATER_OR_EQUAL:
				res = ZBool(ia >= ib)
			case tokenizer.T_IS_EQUAL:
				res = ZBool(ia == ib)
			case tokenizer.T_IS_NOT_EQUAL:
				res = ZBool(ia != ib)
			case tokenizer.T_SPACESHIP:
				if ia < ib {
					res = ZInt(-1)
				} else if ia > ib {
					res = ZInt(1)
				} else {
					res = ZInt(0)
				}
			default:
				return nil, fmt.Errorf("unsupported operator %s", op)
			}
		case ZtFloat:
			switch op {
			case tokenizer.ItemSingleChar('<'):
				res = ZBool(ia.Value().(ZFloat) < ib.Value().(ZFloat))
			case tokenizer.ItemSingleChar('>'):
				res = ZBool(ia.Value().(ZFloat) > ib.Value().(ZFloat))
			case tokenizer.T_IS_SMALLER_OR_EQUAL:
				res = ZBool(ia.Value().(ZFloat) <= ib.Value().(ZFloat))
			case tokenizer.T_IS_GREATER_OR_EQUAL:
				res = ZBool(ia.Value().(ZFloat) >= ib.Value().(ZFloat))
			case tokenizer.T_IS_EQUAL:
				res = ZBool(ia.Value().(ZFloat) == ib.Value().(ZFloat))
			case tokenizer.T_IS_NOT_EQUAL:
				res = ZBool(ia.Value().(ZFloat) != ib.Value().(ZFloat))
			default:
				return nil, fmt.Errorf("unsupported operator %s", op)
			}
		}

		return res.ZVal(), nil
	}

	if a.GetType() == ZtNull || b.GetType() == ZtNull {
		return ZBool(true).ZVal(), nil
	}

	if a.GetType() == ZtBool || b.GetType() == ZtBool {
		// comparing any value to bool will cause a cast to bool
		a, _ = a.As(ctx, ZtBool)
		b, _ = b.As(ctx, ZtBool)
		var res bool
		var ab, bb int
		if a.Value().(ZBool) {
			ab = 1
		} else {
			ab = 0
		}
		if b.Value().(ZBool) {
			bb = 1
		} else {
			bb = 0
		}

		switch op {
		case tokenizer.ItemSingleChar('<'):
			res = ab < bb
		case tokenizer.ItemSingleChar('>'):
			res = ab > bb
		case tokenizer.T_IS_SMALLER_OR_EQUAL:
			res = ab <= bb
		case tokenizer.T_IS_GREATER_OR_EQUAL:
			res = ab >= bb
		case tokenizer.T_IS_EQUAL:
			res = ab == bb
		case tokenizer.T_IS_NOT_EQUAL:
			res = ab != bb
		default:
			return nil, fmt.Errorf("unsupported operator %s", op)
		}

		return &ZVal{ZBool(res)}, nil
	}

	// non numeric comparison
	if a.GetType() != b.GetType() {
		return &ZVal{ZBool(false)}, nil
	}

	var res bool

	switch a.Value().GetType() {
	case ZtString:
		av := a.Value().(ZString)
		bv := b.Value().(ZString)
		switch op {
		case tokenizer.ItemSingleChar('<'):
			res = av < bv
		case tokenizer.ItemSingleChar('>'):
			res = av > bv
		case tokenizer.T_IS_SMALLER_OR_EQUAL:
			res = av <= bv
		case tokenizer.T_IS_GREATER_OR_EQUAL:
			res = av >= bv
		case tokenizer.T_IS_EQUAL:
			res = av == bv
		case tokenizer.T_IS_NOT_EQUAL:
			res = av != bv
		default:
			return nil, fmt.Errorf("unsupported operator %s", op)
		}
	default:
		return nil, fmt.Errorf("todo operator type unsupported %s", a.GetType())
	}

	return &ZVal{ZBool(res)}, nil
}
