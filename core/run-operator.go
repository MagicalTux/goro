package core

import (
	"errors"
	"fmt"
)

type runOperator struct {
	op string

	a, b Runnable
}

type operatorInternalDetails struct {
	write   bool
	numeric bool
	skipA   bool
	op      func(ctx Context, op string, a, b *ZVal) (*ZVal, error)
}

var operatorList = map[string]*operatorInternalDetails{
	"=":  &operatorInternalDetails{write: true, skipA: true},
	".=": &operatorInternalDetails{write: true, op: operatorAppend},
	"/=": &operatorInternalDetails{write: true, numeric: true, op: operatorMath},
	"*=": &operatorInternalDetails{write: true, numeric: true, op: operatorMath},
	"-=": &operatorInternalDetails{write: true, numeric: true, op: operatorMath},
	"+=": &operatorInternalDetails{write: true, numeric: true, op: operatorMath},
	".":  &operatorInternalDetails{op: operatorAppend},
	"+":  &operatorInternalDetails{numeric: true, op: operatorMath},
	"-":  &operatorInternalDetails{numeric: true, op: operatorMath},
	"/":  &operatorInternalDetails{numeric: true, op: operatorMath},
	"*":  &operatorInternalDetails{numeric: true, op: operatorMath},
	"<":  &operatorInternalDetails{op: operatorCompare},
	">":  &operatorInternalDetails{op: operatorCompare},
	"<=": &operatorInternalDetails{op: operatorCompare},
	">=": &operatorInternalDetails{op: operatorCompare},
	"==": &operatorInternalDetails{op: operatorCompare},
	"!=": &operatorInternalDetails{op: operatorCompare},
	"!":  &operatorInternalDetails{op: operatorNot},
}

func (r *runOperator) Run(ctx Context) (*ZVal, error) {
	var a, b, res *ZVal
	var err error

	op, ok := operatorList[r.op]
	if !ok {
		return nil, errors.New("unknown operator")
	}

	// read a and b
	if !op.skipA {
		a, err = r.a.Run(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		a = &ZVal{nil}
	}

	b, err = r.b.Run(ctx)
	if err != nil {
		return nil, err
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

	return &ZVal{a.v.(ZString) + b.v.(ZString)}, nil
}

func operatorNot(ctx Context, op string, a, b *ZVal) (*ZVal, error) {
	b, _ = b.As(ctx, ZtBool)

	return &ZVal{!b.v.(ZBool)}, nil
}

func operatorMath(ctx Context, op string, a, b *ZVal) (*ZVal, error) {
	if op[len(op)-1] == '=' {
		op = op[:len(op)-1]
	}

	switch a.v.GetType() {
	case ZtInt:
		var res ZInt
		switch op {
		case "+":
			res = a.v.(ZInt) + b.v.(ZInt)
		case "-":
			res = a.v.(ZInt) - b.v.(ZInt)
		case "/":
			res = a.v.(ZInt) / b.v.(ZInt)
		case "*":
			res = a.v.(ZInt) * b.v.(ZInt)
		}
		return &ZVal{res}, nil
	case ZtFloat:
		var res ZFloat
		switch op {
		case "+":
			res = a.v.(ZFloat) + b.v.(ZFloat)
		case "-":
			res = a.v.(ZFloat) - b.v.(ZFloat)
		case "/":
			res = a.v.(ZFloat) / b.v.(ZFloat)
		case "*":
			res = a.v.(ZFloat) * b.v.(ZFloat)
		}
		return &ZVal{res}, nil
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
		res = a.v.(ZBool) == b.v.(ZBool)
	case ZtInt:
		res = a.v.(ZInt) == b.v.(ZInt)
	case ZtFloat:
		res = a.v.(ZFloat) == b.v.(ZFloat)
	case ZtString:
		res = a.v.(ZString) == b.v.(ZString)
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
		if a.v.(ZString).LooksInt() {
			ia, _ = a.As(ctx, ZtInt)
		} else if a.v.(ZString).IsNumeric() {
			ia, _ = a.As(ctx, ZtFloat)
		}
	}

	switch b.GetType() {
	case ZtInt, ZtFloat:
		ib = b
	case ZtString:
		if b.v.(ZString).LooksInt() {
			ib, _ = b.As(ctx, ZtInt)
		} else if b.v.(ZString).IsNumeric() {
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
				res = ia.v.(ZInt) < ib.v.(ZInt)
			case ">":
				res = ia.v.(ZInt) > ib.v.(ZInt)
			case "<=":
				res = ia.v.(ZInt) <= ib.v.(ZInt)
			case ">=":
				res = ia.v.(ZInt) >= ib.v.(ZInt)
			case "==":
				res = ia.v.(ZInt) == ib.v.(ZInt)
			case "!=":
				res = ia.v.(ZInt) != ib.v.(ZInt)
			default:
				return nil, fmt.Errorf("unsupported operator %s", op)
			}
		case ZtFloat:
			switch op {
			case "<":
				res = ia.v.(ZFloat) < ib.v.(ZFloat)
			case ">":
				res = ia.v.(ZFloat) > ib.v.(ZFloat)
			case "<=":
				res = ia.v.(ZFloat) <= ib.v.(ZFloat)
			case ">=":
				res = ia.v.(ZFloat) >= ib.v.(ZFloat)
			case "==":
				res = ia.v.(ZFloat) == ib.v.(ZFloat)
			case "!=":
				res = ia.v.(ZFloat) != ib.v.(ZFloat)
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
		if a.v.(ZBool) {
			ab = 1
		} else {
			ab = 0
		}
		if b.v.(ZBool) {
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

	switch a.v.GetType() {
	case ZtString:
		av := a.v.(ZString)
		bv := b.v.(ZString)
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
