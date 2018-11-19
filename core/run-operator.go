package core

import (
	"errors"
	"fmt"
	"io"
	"math"
)

type runOperator struct {
	op  string
	opD *operatorInternalDetails

	a, b Runnable
	l    *Loc
}

type operatorInternalDetails struct {
	write   bool
	numeric bool
	skipA   bool
	op      func(ctx Context, op string, a, b *ZVal) (*ZVal, error)
	pri     int
}

// cast @ pri=11
// ?: pri=24
var operatorList = map[string]*operatorInternalDetails{
	"=":   &operatorInternalDetails{write: true, skipA: true, pri: 25},
	".=":  &operatorInternalDetails{write: true, op: operatorAppend, pri: 25},
	"/=":  &operatorInternalDetails{write: true, numeric: true, op: operatorMath, pri: 25},
	"*=":  &operatorInternalDetails{write: true, numeric: true, op: operatorMath, pri: 25},
	"**=": &operatorInternalDetails{write: true, numeric: true, op: operatorMath, pri: 25},
	"-=":  &operatorInternalDetails{write: true, numeric: true, op: operatorMath, pri: 25},
	"+=":  &operatorInternalDetails{write: true, numeric: true, op: operatorMath, pri: 25},
	".":   &operatorInternalDetails{op: operatorAppend, pri: 14},
	"+":   &operatorInternalDetails{numeric: true, op: operatorMath, pri: 14},
	"-":   &operatorInternalDetails{numeric: true, op: operatorMath, pri: 14},
	"/":   &operatorInternalDetails{numeric: true, op: operatorMath, pri: 13},
	"*":   &operatorInternalDetails{numeric: true, op: operatorMath, pri: 13},
	"**":  &operatorInternalDetails{numeric: true, op: operatorMath, pri: 10},
	"|=":  &operatorInternalDetails{write: true, numeric: true, op: operatorMathLogic, pri: 25},
	"^=":  &operatorInternalDetails{write: true, numeric: true, op: operatorMathLogic, pri: 25},
	"&=":  &operatorInternalDetails{write: true, numeric: true, op: operatorMathLogic, pri: 25},
	"%=":  &operatorInternalDetails{write: true, numeric: true, op: operatorMathLogic, pri: 25},
	"|":   &operatorInternalDetails{op: operatorMathLogic, pri: 20},
	"^":   &operatorInternalDetails{op: operatorMathLogic, pri: 19},
	"&":   &operatorInternalDetails{op: operatorMathLogic, pri: 18},
	"%":   &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 13},
	"~":   &operatorInternalDetails{op: operatorMathLogic, pri: 11},
	"<<":  &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 15},
	">>":  &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 15},
	"and": &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 26},
	"xor": &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 27},
	"ro":  &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 28},
	"<<=": &operatorInternalDetails{write: true, numeric: true, op: operatorMathLogic, pri: 25},
	">>=": &operatorInternalDetails{write: true, numeric: true, op: operatorMathLogic, pri: 25},
	"<":   &operatorInternalDetails{op: operatorCompare, pri: 16},
	">":   &operatorInternalDetails{op: operatorCompare, pri: 16},
	"<=":  &operatorInternalDetails{op: operatorCompare, pri: 16},
	">=":  &operatorInternalDetails{op: operatorCompare, pri: 16},
	"==":  &operatorInternalDetails{op: operatorCompare, pri: 17},
	"===": &operatorInternalDetails{op: operatorCompareStrict, pri: 17},
	"!=":  &operatorInternalDetails{op: operatorCompare, pri: 17},
	"<>":  &operatorInternalDetails{op: operatorCompare, pri: 17},
	"<=>": &operatorInternalDetails{op: operatorCompare, pri: 17},
	"!==": &operatorInternalDetails{op: operatorCompareStrict, pri: 17},
	"!":   &operatorInternalDetails{op: operatorNot, pri: 12},
	"&&":  &operatorInternalDetails{op: operatorBoolLogic, pri: 21},
	"||":  &operatorInternalDetails{op: operatorBoolLogic, pri: 22},
	"??":  &operatorInternalDetails{pri: 23},
	"++":  &operatorInternalDetails{op: operatorIncDec, pri: 11},
	"--":  &operatorInternalDetails{op: operatorIncDec, pri: 11},

	// cast operators
	"(bool)":   &operatorInternalDetails{op: func(ctx Context, op string, a, b *ZVal) (*ZVal, error) { return b.As(ctx, ZtBool) }, pri: 11},
	"(int)":    &operatorInternalDetails{op: func(ctx Context, op string, a, b *ZVal) (*ZVal, error) { return b.As(ctx, ZtInt) }, pri: 11},
	"(double)": &operatorInternalDetails{op: func(ctx Context, op string, a, b *ZVal) (*ZVal, error) { return b.As(ctx, ZtFloat) }, pri: 11},
	"(array)":  &operatorInternalDetails{op: func(ctx Context, op string, a, b *ZVal) (*ZVal, error) { return b.As(ctx, ZtArray) }, pri: 11},
	"(object)": &operatorInternalDetails{op: func(ctx Context, op string, a, b *ZVal) (*ZVal, error) { return b.As(ctx, ZtObject) }, pri: 11},
	"(string)": &operatorInternalDetails{op: func(ctx Context, op string, a, b *ZVal) (*ZVal, error) { return b.As(ctx, ZtString) }, pri: 11},
}

func (r *runOperator) Loc() *Loc {
	return r.l
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
	_, err = w.Write([]byte(r.op))
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

func spawnOperator(op string, a, b Runnable, l *Loc) (Runnable, error) {
	opD, ok := operatorList[op]
	if !ok {
		return nil, l.Errorf(nil, E_COMPILE_ERROR, "invalid operator %s", op)
	}
	if rop, isop := a.(*runOperator); isop {
		if opD.pri < rop.opD.pri {
			// need to swap values
			rop.b = &runOperator{op: op, opD: opD, a: rop.b, b: b, l: l}
			return rop, nil
		}
	}
	return &runOperator{op: op, opD: opD, a: a, b: b, l: l}, nil
}

func (r *runOperator) Run(ctx Context) (*ZVal, error) {
	var a, b, res *ZVal
	var err error

	op := r.opD

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

func operatorAppend(ctx Context, op string, a, b *ZVal) (*ZVal, error) {
	a, _ = a.As(ctx, ZtString)
	b, _ = b.As(ctx, ZtString)

	return &ZVal{a.AsString(ctx) + b.AsString(ctx)}, nil
}

func operatorNot(ctx Context, op string, a, b *ZVal) (*ZVal, error) {
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
			n += 1
		} else {
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
	return errors.New("unsupported type for increment operator")
}

func operatorIncDec(ctx Context, op string, a, b *ZVal) (*ZVal, error) {
	inc := op == "++"

	if a != nil {
		// post mode
		orig := a.Dup()
		return orig, doInc(a, inc)
	} else {
		// pre mode
		return b, doInc(b, inc)
	}
}

func operatorMath(ctx Context, op string, a, b *ZVal) (*ZVal, error) {
	if op[len(op)-1] == '=' {
		op = op[:len(op)-1]
	}

	switch a.Value().GetType() {
	case ZtInt:
		var res Val
		a := a.Value().(ZInt)
		b := b.Value().(ZInt)

		switch op {
		case "+":
			c := a + b
			if (c > a) == (b > 0) {
				res = c
			} else {
				// overflow
				res = ZFloat(a) + ZFloat(b)
			}
		case "-":
			c := a - b
			if (c < a) == (b > 0) {
				res = c
			} else {
				// overflow
				res = ZFloat(a) - ZFloat(b)
			}
		case "/":
			if b == 0 {
				return nil, errors.New("Division by zero")
			}
			if a%b != 0 {
				// this is not goign to be a int result
				res = ZFloat(a) / ZFloat(b)
			} else {
				res = a / b
			}
		case "*":
			res = a * b
		case "**":
			res = ZFloat(math.Pow(float64(a), float64(b)))
		}
		return &ZVal{res}, nil
	case ZtFloat:
		var res ZFloat
		switch op {
		case "+":
			res = a.Value().(ZFloat) + b.Value().(ZFloat)
		case "-":
			res = a.Value().(ZFloat) - b.Value().(ZFloat)
		case "/":
			res = a.Value().(ZFloat) / b.Value().(ZFloat)
		case "*":
			res = a.Value().(ZFloat) * b.Value().(ZFloat)
		case "**":
			res = ZFloat(math.Pow(float64(a.Value().(ZFloat)), float64(b.Value().(ZFloat))))
		}
		return &ZVal{res}, nil
	default:
		return nil, errors.New("todo operator type unsupported")
	}
}

func operatorBoolLogic(ctx Context, op string, a, b *ZVal) (*ZVal, error) {
	switch op {
	case "&&":
		return (a.AsBool(ctx) && b.AsBool(ctx)).ZVal(), nil
	case "||":
		return (a.AsBool(ctx) || b.AsBool(ctx)).ZVal(), nil
	default:
		return nil, errors.New("todo operator unsupported")
	}
}

func operatorMathLogic(ctx Context, op string, a, b *ZVal) (*ZVal, error) {
	if op[len(op)-1] == '=' {
		op = op[:len(op)-1]
	}
	if a == nil {
		a = b
	}

	switch a.Value().GetType() {
	case ZtInt:
		b, _ = b.As(ctx, ZtInt)
		var res ZInt
		switch op {
		case "|":
			res = a.Value().(ZInt) | b.Value().(ZInt)
		case "^":
			res = a.Value().(ZInt) ^ b.Value().(ZInt)
		case "&":
			res = a.Value().(ZInt) & b.Value().(ZInt)
		case "%":
			res = a.Value().(ZInt) % b.Value().(ZInt)
		case "~":
			res = ^b.Value().(ZInt)
		case "<<":
			// TODO error check on negative b
			res = a.Value().(ZInt) << uint(b.Value().(ZInt))
		case ">>":
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
			case "|": // make b longer in this case
				newb := make([]byte, len(a))
				copy(newb, b)
				b = newb
			default:
				a = a[:len(b)]
			}
		}

		switch op {
		case "|":
			for i := 0; i < len(a); i++ {
				a[i] |= b[i]
			}
		case "^":
			for i := 0; i < len(a); i++ {
				a[i] ^= b[i]
			}
		case "&":
			for i := 0; i < len(a); i++ {
				a[i] &= b[i]
			}
		case "~":
			for i := 0; i < len(a); i++ {
				b[i] = ^b[i]
			}
			a = b
		default:
			return nil, errors.New("todo operator unsupported on strings")
		}
		return &ZVal{ZString(a)}, nil
	default:
		return nil, errors.New("todo operator type unsupported")
	}
}

func operatorCompareStrict(ctx Context, op string, a, b *ZVal) (*ZVal, error) {
	if a.GetType() != b.GetType() {
		// not same type → false
		return &ZVal{ZBool(false)}, nil
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
		return nil, errors.New("unsupported compare type")
	}

	if op == "!==" {
		res = !res
	}

	return &ZVal{ZBool(res)}, nil
}

func operatorCompare(ctx Context, op string, a, b *ZVal) (*ZVal, error) {
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

		var res bool
		switch ia.GetType() {
		case ZtInt:
			switch op {
			case "<":
				res = ia.Value().(ZInt) < ib.Value().(ZInt)
			case ">":
				res = ia.Value().(ZInt) > ib.Value().(ZInt)
			case "<=":
				res = ia.Value().(ZInt) <= ib.Value().(ZInt)
			case ">=":
				res = ia.Value().(ZInt) >= ib.Value().(ZInt)
			case "==":
				res = ia.Value().(ZInt) == ib.Value().(ZInt)
			case "!=":
				res = ia.Value().(ZInt) != ib.Value().(ZInt)
			default:
				return nil, fmt.Errorf("unsupported operator %s", op)
			}
		case ZtFloat:
			switch op {
			case "<":
				res = ia.Value().(ZFloat) < ib.Value().(ZFloat)
			case ">":
				res = ia.Value().(ZFloat) > ib.Value().(ZFloat)
			case "<=":
				res = ia.Value().(ZFloat) <= ib.Value().(ZFloat)
			case ">=":
				res = ia.Value().(ZFloat) >= ib.Value().(ZFloat)
			case "==":
				res = ia.Value().(ZFloat) == ib.Value().(ZFloat)
			case "!=":
				res = ia.Value().(ZFloat) != ib.Value().(ZFloat)
			default:
				return nil, fmt.Errorf("unsupported operator %s", op)
			}
		}

		return &ZVal{ZBool(res)}, nil
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
		case "<":
			res = ab < bb
		case ">":
			res = ab > bb
		case "<=":
			res = ab <= bb
		case ">=":
			res = ab >= bb
		case "==":
			res = ab == bb
		case "!=":
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
		case "<":
			res = av < bv
		case ">":
			res = av > bv
		case "<=":
			res = av <= bv
		case ">=":
			res = av >= bv
		case "==":
			res = av == bv
		case "!=":
			res = av != bv
		default:
			return nil, fmt.Errorf("unsupported operator %s", op)
		}
	default:
		return nil, errors.New("todo operator type unsupported")
	}

	return &ZVal{ZBool(res)}, nil
}
