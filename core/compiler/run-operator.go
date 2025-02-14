package compiler

import (
	"io"
	"math"

	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runOperator struct {
	op  tokenizer.ItemType
	opD *operatorInternalDetails

	a, b phpv.Runnable
	l    *phpv.Loc
}

type operatorInternalDetails struct {
	write   bool
	numeric bool
	skipA   bool
	op      func(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error)
	pri     int
}

// ?: pri=24
var operatorList = map[tokenizer.ItemType]*operatorInternalDetails{
	tokenizer.Rune('='):             &operatorInternalDetails{write: true, skipA: true, pri: 25},
	tokenizer.T_CONCAT_EQUAL:        &operatorInternalDetails{write: true, op: operatorAppend, pri: 25},
	tokenizer.T_DIV_EQUAL:           &operatorInternalDetails{write: true, skipA: true, numeric: true, op: operatorMath, pri: 25},
	tokenizer.T_MUL_EQUAL:           &operatorInternalDetails{write: true, skipA: true, numeric: true, op: operatorMath, pri: 25},
	tokenizer.T_POW_EQUAL:           &operatorInternalDetails{write: true, skipA: true, numeric: true, op: operatorMath, pri: 25},
	tokenizer.T_MINUS_EQUAL:         &operatorInternalDetails{write: true, skipA: true, numeric: true, op: operatorMath, pri: 25},
	tokenizer.T_PLUS_EQUAL:          &operatorInternalDetails{write: true, skipA: true, numeric: true, op: operatorMath, pri: 25},
	tokenizer.Rune('.'):             &operatorInternalDetails{op: operatorAppend, pri: 14},
	tokenizer.Rune('+'):             &operatorInternalDetails{numeric: true, op: operatorMath, pri: 14},
	tokenizer.Rune('-'):             &operatorInternalDetails{numeric: true, op: operatorMath, pri: 14},
	tokenizer.Rune('/'):             &operatorInternalDetails{numeric: true, op: operatorMath, pri: 13},
	tokenizer.Rune('*'):             &operatorInternalDetails{numeric: true, op: operatorMath, pri: 13},
	tokenizer.T_POW:                 &operatorInternalDetails{numeric: true, skipA: true, op: operatorMath, pri: 10},
	tokenizer.T_OR_EQUAL:            &operatorInternalDetails{write: true, skipA: true, numeric: true, op: operatorMathLogic, pri: 25},
	tokenizer.T_XOR_EQUAL:           &operatorInternalDetails{write: true, skipA: true, numeric: true, op: operatorMathLogic, pri: 25},
	tokenizer.T_AND_EQUAL:           &operatorInternalDetails{write: true, skipA: true, numeric: true, op: operatorMathLogic, pri: 25},
	tokenizer.T_MOD_EQUAL:           &operatorInternalDetails{write: true, skipA: true, numeric: true, op: operatorMathLogic, pri: 25},
	tokenizer.Rune('|'):             &operatorInternalDetails{op: operatorMathLogic, pri: 20},
	tokenizer.Rune('^'):             &operatorInternalDetails{op: operatorMathLogic, pri: 19},
	tokenizer.Rune('&'):             &operatorInternalDetails{op: operatorMathLogic, pri: 18},
	tokenizer.Rune('%'):             &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 13},
	tokenizer.Rune('~'):             &operatorInternalDetails{op: operatorMathLogic, pri: 11},
	tokenizer.T_SL:                  &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 15},
	tokenizer.T_SR:                  &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 15},
	tokenizer.T_LOGICAL_AND:         &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 26},
	tokenizer.T_LOGICAL_XOR:         &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 27},
	tokenizer.T_LOGICAL_OR:          &operatorInternalDetails{numeric: true, op: operatorMathLogic, pri: 28},
	tokenizer.T_SL_EQUAL:            &operatorInternalDetails{write: true, skipA: true, numeric: true, op: operatorMathLogic, pri: 25},
	tokenizer.T_SR_EQUAL:            &operatorInternalDetails{write: true, skipA: true, numeric: true, op: operatorMathLogic, pri: 25},
	tokenizer.Rune('<'):             &operatorInternalDetails{op: operatorCompare, pri: 16},
	tokenizer.Rune('>'):             &operatorInternalDetails{op: operatorCompare, pri: 16},
	tokenizer.T_IS_SMALLER_OR_EQUAL: &operatorInternalDetails{op: operatorCompare, pri: 16},
	tokenizer.T_IS_GREATER_OR_EQUAL: &operatorInternalDetails{op: operatorCompare, pri: 16},
	tokenizer.T_IS_EQUAL:            &operatorInternalDetails{op: operatorCompare, pri: 17},
	tokenizer.T_IS_IDENTICAL:        &operatorInternalDetails{op: operatorCompareStrict, pri: 17},
	tokenizer.T_IS_NOT_EQUAL:        &operatorInternalDetails{op: operatorCompare, pri: 17},
	tokenizer.T_SPACESHIP:           &operatorInternalDetails{op: operatorCompare, pri: 17},
	tokenizer.T_IS_NOT_IDENTICAL:    &operatorInternalDetails{op: operatorCompareStrict, pri: 17},
	tokenizer.Rune('!'):             &operatorInternalDetails{op: operatorNot, pri: 12},
	tokenizer.T_BOOLEAN_AND:         &operatorInternalDetails{op: operatorBoolLogic, pri: 21},
	tokenizer.T_BOOLEAN_OR:          &operatorInternalDetails{op: operatorBoolLogic, pri: 22},
	tokenizer.T_COALESCE:            &operatorInternalDetails{pri: 23, skipA: true}, // TODO
	tokenizer.T_INC:                 &operatorInternalDetails{op: operatorIncDec, pri: 11},
	tokenizer.T_DEC:                 &operatorInternalDetails{op: operatorIncDec, pri: 11},
	tokenizer.Rune('@'):             &operatorInternalDetails{pri: 11}, // TODO

	// cast operators
	tokenizer.T_BOOL_CAST: &operatorInternalDetails{op: func(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
		return b.As(ctx, phpv.ZtBool)
	}, pri: 11},
	tokenizer.T_INT_CAST: &operatorInternalDetails{op: func(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
		return b.As(ctx, phpv.ZtInt)
	}, pri: 11},
	tokenizer.T_DOUBLE_CAST: &operatorInternalDetails{op: func(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
		return b.As(ctx, phpv.ZtFloat)
	}, pri: 11},
	tokenizer.T_ARRAY_CAST: &operatorInternalDetails{op: func(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
		return b.As(ctx, phpv.ZtArray)
	}, pri: 11},
	tokenizer.T_OBJECT_CAST: &operatorInternalDetails{op: func(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
		return b.As(ctx, phpv.ZtObject)
	}, pri: 11},
	tokenizer.T_STRING_CAST: &operatorInternalDetails{op: func(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
		return b.As(ctx, phpv.ZtString)
	}, pri: 11},
}

func isOperator(t tokenizer.ItemType) bool {
	_, ok := operatorList[t]
	return ok
}
func isRightAssociative(t tokenizer.ItemType) bool {
	if op, ok := operatorList[t]; ok {
		return op.skipA
	}
	return false
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

func spawnOperator(ctx phpv.Context, op tokenizer.ItemType, a, b phpv.Runnable, l *phpv.Loc) (phpv.Runnable, error) {
	var err error
	opD, ok := operatorList[op]
	if !ok {
		return nil, l.Errorf(ctx, phpv.E_COMPILE_ERROR, "invalid operator %s", op)
	}

	//log.Printf("spawn operator %s %s %s", debugDump(a), op.Name(), debugDump(b))
	if rop, isop := a.(*runOperator); isop {
		if opD.pri < rop.opD.pri {
			// need to go down one level values
			rop.b, err = spawnOperator(ctx, op, rop.b, b, l)
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

func (r *runOperator) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	var a, b, res *phpv.ZVal
	var err error

	err = ctx.Tick(ctx, r.l)
	if err != nil {
		return nil, err
	}

	op := r.opD

	if r.op == tokenizer.Rune('@') {
		// silence errors
		ctx = phpctx.WithConfig(ctx, "error_reporting", phpv.ZInt(0).ZVal())
	}

	// read a and b
	if r.a != nil {
		if variable, ok := r.a.(*runVariable); ok {
			if !op.write && variable.IsUnDefined(ctx) {
				if err = ctx.Notice("Undefined variable: %s", variable.VarName()); err != nil {
					return nil, err
				}
			}
		}
		a, err = r.a.Run(ctx)
		if err != nil {
			return nil, err
		}

	}

	// short-circuit evaluation
	switch r.op {
	case tokenizer.T_LOGICAL_AND, tokenizer.T_BOOLEAN_AND:
		if !a.AsBool(ctx) {
			return phpv.ZFalse.ZVal(), nil
		}
	case tokenizer.T_LOGICAL_OR, tokenizer.T_BOOLEAN_OR:
		if a.AsBool(ctx) {
			return phpv.ZTrue.ZVal(), nil
		}
	}

	if r.b != nil {
		if variable, ok := r.b.(*runVariable); ok && variable.IsUnDefined(ctx) {
			if err = ctx.Notice("Undefined variable: %s", variable.VarName()); err != nil {
				return nil, err
			}
		}
		b, err = r.b.Run(ctx)
		if err != nil {
			return nil, err
		}
	}

	if op.numeric {
		a, _ = a.AsNumeric(ctx)
		b, _ = b.AsNumeric(ctx)

		// normalize types
		if a.GetType() == phpv.ZtFloat || b.GetType() == phpv.ZtFloat {
			a, _ = a.As(ctx, phpv.ZtFloat)
			b, _ = b.As(ctx, phpv.ZtFloat)
		} else {
			a, _ = a.As(ctx, phpv.ZtInt)
			b, _ = b.As(ctx, phpv.ZtInt)
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
		w, ok := r.a.(phpv.Writable)
		if !ok {
			return nil, ctx.Errorf("Can't use %#v value in write context", r.a)
		}

		// The PHP documentation states that the array's internal
		// pointer is reset when assigning to another variable
		// AND the internal pointer is at the end.
		// The following code handles that special case.
		if res.GetType() == phpv.ZtArray {
			res.AsArray(ctx).MainIterator().ResetIfEnd(ctx)
		}

		return res, w.WriteValue(ctx, res.ZVal())
	}

	return res, nil
}

func operatorAppend(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	a, _ = a.As(ctx, phpv.ZtString)
	b, _ = b.As(ctx, phpv.ZtString)

	return (a.AsString(ctx) + b.AsString(ctx)).ZVal(), nil
}

func operatorNot(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	b, _ = b.As(ctx, phpv.ZtBool)

	return (!b.Value().(phpv.ZBool)).ZVal(), nil
}

func doInc(ctx phpv.Context, v *phpv.ZVal, inc bool) error {
	switch v.GetType() {
	case phpv.ZtNull:
		if inc {
			v.Set(phpv.ZInt(1).ZVal())
		}
		return nil
	case phpv.ZtBool:
		return nil
	case phpv.ZtInt:
		n := v.Value().(phpv.ZInt)
		if inc {
			if n == math.MaxInt64 {
				v.Set((phpv.ZFloat(n) + 1).ZVal())
				return nil
			}
			n++
		} else {
			if n == math.MinInt64 {
				v.Set((phpv.ZFloat(n) - 1).ZVal())
				return nil
			}
			n--
		}
		v.Set(n.ZVal())
		return nil
	case phpv.ZtFloat:
		n := v.Value().(phpv.ZFloat)
		if inc {
			n++
		} else {
			n--
		}
		v.Set(n.ZVal())
		return nil
	case phpv.ZtString:
		s := v.Value().(phpv.ZString)
		// first, check if potentially numeric
		if s.IsNumeric() {
			if x, err := s.AsNumeric(); err == nil {
				v.Set(x.ZVal())
				return doInc(ctx, v, inc)
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
				v.Set(phpv.ZString(n).ZVal())
				return nil
			}
			v.Set(phpv.ZString(n).ZVal())
			return nil
		}

		switch c {
		case '9':
			v.Set(("1" + phpv.ZString(n)).ZVal())
			return nil
		case 'z':
			v.Set(("a" + phpv.ZString(n)).ZVal())
			return nil
		case 'Z':
			v.Set(("A" + phpv.ZString(n)).ZVal())
			return nil
		}
	}
	return ctx.Errorf("unsupported type for increment operator %s", v.GetType())
}

func operatorIncDec(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	inc := op == tokenizer.T_INC

	if a != nil {
		// post mode
		orig := a.Dup()
		return orig, doInc(ctx, a, inc)
	} else {
		// pre mode
		return b, doInc(ctx, b, inc)
	}
}

func operatorMath(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	switch a.Value().GetType() {
	case phpv.ZtInt:
		var res phpv.Val
		a := a.Value().(phpv.ZInt)
		b := b.Value().(phpv.ZInt)

		switch op {
		case tokenizer.T_PLUS_EQUAL, tokenizer.Rune('+'):
			c := a + b
			if (c > a) == (b > 0) {
				res = c
			} else {
				// overflow
				res = phpv.ZFloat(a) + phpv.ZFloat(b)
			}
		case tokenizer.T_MINUS_EQUAL, tokenizer.Rune('-'):
			c := a - b
			if (c < a) == (b > 0) {
				res = c
			} else {
				// overflow
				res = phpv.ZFloat(a) - phpv.ZFloat(b)
			}
		case tokenizer.T_DIV_EQUAL, tokenizer.Rune('/'):
			if b == 0 {
				return phpv.ZFloat(math.Inf(1)).ZVal(), ctx.Warn("Division by zero")
			}
			if a%b != 0 {
				// this is not goign to be a int result
				res = phpv.ZFloat(a) / phpv.ZFloat(b)
			} else {
				res = a / b
			}
		case tokenizer.T_MUL_EQUAL, tokenizer.Rune('*'):
			if a == 0 || b == 0 {
				res = phpv.ZInt(0)
				break
			}
			c := a * b
			// do overflow check (golang has no good way to perform this, so checking if c/b=a will have to do)
			if ((c < 0) == ((a < 0) != (b < 0))) && (c/b == a) {
				res = c
			} else {
				// do this as float
				res = phpv.ZFloat(a) * phpv.ZFloat(b)
			}
		case tokenizer.T_POW, tokenizer.T_POW_EQUAL:
			res = phpv.ZFloat(math.Pow(float64(a), float64(b)))
		}
		return res.ZVal(), nil
	case phpv.ZtFloat:
		var res phpv.ZFloat
		switch op {
		case tokenizer.T_PLUS_EQUAL, tokenizer.Rune('+'):
			res = a.Value().(phpv.ZFloat) + b.Value().(phpv.ZFloat)
		case tokenizer.T_MINUS_EQUAL, tokenizer.Rune('-'):
			res = a.Value().(phpv.ZFloat) - b.Value().(phpv.ZFloat)
		case tokenizer.T_DIV_EQUAL, tokenizer.Rune('/'):
			res = a.Value().(phpv.ZFloat) / b.Value().(phpv.ZFloat)
		case tokenizer.T_MUL_EQUAL, tokenizer.Rune('*'):
			res = a.Value().(phpv.ZFloat) * b.Value().(phpv.ZFloat)
		case tokenizer.T_POW, tokenizer.T_POW_EQUAL:
			res = phpv.ZFloat(math.Pow(float64(a.Value().(phpv.ZFloat)), float64(b.Value().(phpv.ZFloat))))
		}
		return res.ZVal(), nil
	default:
		return nil, ctx.Errorf("todo operator type unsupported %s", a.GetType())
	}
}

func operatorBoolLogic(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	switch op {
	case tokenizer.T_BOOLEAN_AND:
		return (a.AsBool(ctx) && b.AsBool(ctx)).ZVal(), nil
	case tokenizer.T_BOOLEAN_OR:
		return (a.AsBool(ctx) || b.AsBool(ctx)).ZVal(), nil
	default:
		return nil, ctx.Errorf("todo operator unsupported %s", op)
	}
}

func operatorMathLogic(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	if a == nil {
		a = b
	}

	switch a.Value().GetType() {
	case phpv.ZtInt:
		b, _ = b.As(ctx, phpv.ZtInt)
		var res phpv.ZInt
		switch op {
		case tokenizer.Rune('|'), tokenizer.T_OR_EQUAL:
			res = a.Value().(phpv.ZInt) | b.Value().(phpv.ZInt)
		case tokenizer.Rune('^'), tokenizer.T_XOR_EQUAL:
			res = a.Value().(phpv.ZInt) ^ b.Value().(phpv.ZInt)
		case tokenizer.Rune('&'), tokenizer.T_AND_EQUAL:
			res = a.Value().(phpv.ZInt) & b.Value().(phpv.ZInt)
		case tokenizer.Rune('%'), tokenizer.T_MOD_EQUAL:
			res = a.Value().(phpv.ZInt) % b.Value().(phpv.ZInt)
		case tokenizer.Rune('~'):
			res = ^b.Value().(phpv.ZInt)
		case tokenizer.T_SL, tokenizer.T_SL_EQUAL:
			// TODO error check on negative b
			res = a.Value().(phpv.ZInt) << uint(b.Value().(phpv.ZInt))
		case tokenizer.T_SR, tokenizer.T_SR_EQUAL:
			// TODO error check on negative b
			res = a.Value().(phpv.ZInt) >> uint(b.Value().(phpv.ZInt))
		}
		return res.ZVal(), nil
	case phpv.ZtFloat:
		// need to convert to int
		a, _ = a.As(ctx, phpv.ZtInt)
		b, _ = b.As(ctx, phpv.ZtInt)
		return operatorMathLogic(ctx, op, a, b)
	case phpv.ZtString:
		b, _ = b.As(ctx, phpv.ZtString) // force b to be string
		a := []byte(a.Value().(phpv.ZString))
		b := []byte(b.Value().(phpv.ZString))
		if len(a) != len(b) {
			if len(a) < len(b) {
				a, b = b, a
			}
			// a is longer than b
			switch op {
			case tokenizer.Rune('|'), tokenizer.T_OR_EQUAL: // make b longer in this case
				newb := make([]byte, len(a))
				copy(newb, b)
				b = newb
			default:
				a = a[:len(b)]
			}
		}

		switch op {
		case tokenizer.Rune('|'), tokenizer.T_OR_EQUAL:
			for i := 0; i < len(a); i++ {
				a[i] |= b[i]
			}
		case tokenizer.Rune('^'), tokenizer.T_XOR_EQUAL:
			for i := 0; i < len(a); i++ {
				a[i] ^= b[i]
			}
		case tokenizer.Rune('&'), tokenizer.T_AND_EQUAL:
			for i := 0; i < len(a); i++ {
				a[i] &= b[i]
			}
		case tokenizer.Rune('~'):
			for i := 0; i < len(a); i++ {
				b[i] = ^b[i]
			}
			a = b
		default:
			return nil, ctx.Errorf("todo operator unsupported on strings")
		}
		return phpv.ZString(a).ZVal(), nil
	default:
		return nil, ctx.Errorf("todo operator type unsupported: %s", a.GetType())
	}
}

func operatorCompareStrict(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	if a.GetType() != b.GetType() {
		// not same type → false
		return phpv.ZBool(op != tokenizer.T_IS_IDENTICAL).ZVal(), nil
	}

	var res bool

	switch a.GetType() {
	case phpv.ZtNull:
		res = true
	case phpv.ZtBool:
		res = a.Value().(phpv.ZBool) == b.Value().(phpv.ZBool)
	case phpv.ZtInt:
		res = a.Value().(phpv.ZInt) == b.Value().(phpv.ZInt)
	case phpv.ZtFloat:
		res = a.Value().(phpv.ZFloat) == b.Value().(phpv.ZFloat)
	case phpv.ZtString:
		res = a.Value().(phpv.ZString) == b.Value().(phpv.ZString)
	default:
		return nil, ctx.Errorf("unsupported compare type %s", a.GetType())
	}

	if op == tokenizer.T_IS_NOT_IDENTICAL {
		res = !res
	}

	return phpv.ZBool(res).ZVal(), nil
}

func operatorCompare(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	// operator compare (< > <= >= == === != !== <=>) involve a lot of dark magic in php, unless both values are of the same type (and even so)
	// loose comparison will convert number-y looking strings into numbers, etc
	var ia, ib *phpv.ZVal

	switch a.GetType() {
	case phpv.ZtInt, phpv.ZtFloat:
		ia = a
	case phpv.ZtString:
		if a.Value().(phpv.ZString).LooksInt() {
			ia, _ = a.As(ctx, phpv.ZtInt)
		} else if a.Value().(phpv.ZString).IsNumeric() {
			ia, _ = a.As(ctx, phpv.ZtFloat)
		}
	}

	switch b.GetType() {
	case phpv.ZtInt, phpv.ZtFloat:
		ib = b
	case phpv.ZtString:
		if b.Value().(phpv.ZString).LooksInt() {
			ib, _ = b.As(ctx, phpv.ZtInt)
		} else if b.Value().(phpv.ZString).IsNumeric() {
			ib, _ = b.As(ctx, phpv.ZtFloat)
		}
	}

	// if both are strings but only one is numeric, then do string comparison
	// this handle cases such as "a" > "9999"
	aIsNonNumericString := a.GetType() == phpv.ZtString && ia == nil
	bIsNonNumericString := b.GetType() == phpv.ZtString && ib == nil
	if (aIsNonNumericString && ib != nil && b.GetType() != phpv.ZtInt) ||
		(bIsNonNumericString && ia != nil && a.GetType() != phpv.ZtInt) {
		goto CompareStrings
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
			ia, _ = ia.As(ctx, phpv.ZtFloat)
			ib, _ = ib.As(ctx, phpv.ZtFloat)
		}

		// handle case where '' is compared to '0', so that '' < '0'
		if a.GetType() == phpv.ZtString && a.Value().(phpv.ZString) == "" {
			ia = phpv.ZInt(-1).ZVal()
		}
		if b.GetType() == phpv.ZtString && b.Value().(phpv.ZString) == "" {
			ib = phpv.ZInt(-1).ZVal()
		}

		var res phpv.Val
		switch ia.GetType() {
		case phpv.ZtInt:
			ia := ia.Value().(phpv.ZInt)
			ib := ib.Value().(phpv.ZInt)
			switch op {
			case tokenizer.Rune('<'):
				res = phpv.ZBool(ia < ib)
			case tokenizer.Rune('>'):
				res = phpv.ZBool(ia > ib)
			case tokenizer.T_IS_SMALLER_OR_EQUAL:
				res = phpv.ZBool(ia <= ib)
			case tokenizer.T_IS_GREATER_OR_EQUAL:
				res = phpv.ZBool(ia >= ib)
			case tokenizer.T_IS_EQUAL:
				res = phpv.ZBool(ia == ib)
			case tokenizer.T_IS_NOT_EQUAL:
				res = phpv.ZBool(ia != ib)
			case tokenizer.T_SPACESHIP:
				if ia < ib {
					res = phpv.ZInt(-1)
				} else if ia > ib {
					res = phpv.ZInt(1)
				} else {
					res = phpv.ZInt(0)
				}
			default:
				return nil, ctx.Errorf("unsupported operator %s", op)
			}
		case phpv.ZtFloat:
			switch op {
			case tokenizer.Rune('<'):
				res = phpv.ZBool(ia.Value().(phpv.ZFloat) < ib.Value().(phpv.ZFloat))
			case tokenizer.Rune('>'):
				res = phpv.ZBool(ia.Value().(phpv.ZFloat) > ib.Value().(phpv.ZFloat))
			case tokenizer.T_IS_SMALLER_OR_EQUAL:
				res = phpv.ZBool(ia.Value().(phpv.ZFloat) <= ib.Value().(phpv.ZFloat))
			case tokenizer.T_IS_GREATER_OR_EQUAL:
				res = phpv.ZBool(ia.Value().(phpv.ZFloat) >= ib.Value().(phpv.ZFloat))
			case tokenizer.T_IS_EQUAL:
				res = phpv.ZBool(ia.Value().(phpv.ZFloat) == ib.Value().(phpv.ZFloat))
			case tokenizer.T_IS_NOT_EQUAL:
				res = phpv.ZBool(ia.Value().(phpv.ZFloat) != ib.Value().(phpv.ZFloat))
			default:
				return nil, ctx.Errorf("unsupported operator %s", op)
			}
		}

		return res.ZVal(), nil
	}

	if a.GetType() == phpv.ZtNull && b.GetType() == phpv.ZtNull {
		return phpv.ZBool(true).ZVal(), nil
	}

	if a.GetType() == phpv.ZtBool || b.GetType() == phpv.ZtBool {
		// comparing any value to bool will cause a cast to bool
		a, _ = a.As(ctx, phpv.ZtBool)
		b, _ = b.As(ctx, phpv.ZtBool)
		var res bool
		var ab, bb int
		if a.Value().(phpv.ZBool) {
			ab = 1
		} else {
			ab = 0
		}
		if b.Value().(phpv.ZBool) {
			bb = 1
		} else {
			bb = 0
		}

		switch op {
		case tokenizer.Rune('<'):
			res = ab < bb
		case tokenizer.Rune('>'):
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
			return nil, ctx.Errorf("unsupported operator %s", op)
		}

		return phpv.ZBool(res).ZVal(), nil
	}

	// non numeric comparison
	if a.GetType() != b.GetType() {
		return phpv.ZBool(false).ZVal(), nil
	}

CompareStrings:
	var res bool

	switch a.Value().GetType() {
	case phpv.ZtString:
		av := a.AsString(ctx)
		bv := b.AsString(ctx)
		switch op {
		case tokenizer.Rune('<'):
			res = av < bv
		case tokenizer.Rune('>'):
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
			return nil, ctx.Errorf("unsupported operator %s", op)
		}
	case phpv.ZtArray:
		// Array with fewer members is smaller, if key from operand 1 is not found in operand 2
		// then arrays are incomparable, otherwise - compare value by value
		switch b.Value().GetType() {
		case phpv.ZtArray:
			arrA := a.AsArray(ctx)
			arrB := b.AsArray(ctx)
			switch op {
			case tokenizer.Rune('>'):
				res = arrA.Count(ctx) > arrB.Count(ctx)
			case tokenizer.Rune('<'):
				res = arrA.Count(ctx) < arrB.Count(ctx)
			case tokenizer.T_IS_SMALLER_OR_EQUAL:
				res = arrA.Count(ctx) < arrB.Count(ctx) || arrA.Equals(ctx, arrB)
			case tokenizer.T_IS_GREATER_OR_EQUAL:
				res = arrA.Count(ctx) > arrB.Count(ctx) || arrA.Equals(ctx, arrB)
			case tokenizer.T_IS_EQUAL:
				res = arrA.Equals(ctx, arrB)
			case tokenizer.T_IS_NOT_EQUAL:
				res = !arrA.Equals(ctx, arrB)
			default:
				return nil, ctx.Errorf("unsupported operator %s", op)
			}
		default:
			// Array > not-Array
			switch op {
			case tokenizer.Rune('>'), tokenizer.T_IS_GREATER_OR_EQUAL:
				res = true
			case tokenizer.Rune('<'),
				tokenizer.T_IS_SMALLER_OR_EQUAL,
				tokenizer.T_IS_EQUAL, tokenizer.T_IS_NOT_EQUAL:
				res = false
			default:
				return nil, ctx.Errorf("unsupported operator %s", op)
			}

		}

	case phpv.ZtObject:
		// TODO:
	default:
		return nil, ctx.Errorf("todo operator type unsupported %s", a.GetType())
	}

	return phpv.ZBool(res).ZVal(), nil
}
