package compiler

import (
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
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

var ternaryPri = 22

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
	tokenizer.T_LOGICAL_AND:         &operatorInternalDetails{op: operatorBoolLogic, pri: 26},
	tokenizer.T_LOGICAL_XOR:         &operatorInternalDetails{op: operatorLogicalXor, pri: 27},
	tokenizer.T_LOGICAL_OR:          &operatorInternalDetails{op: operatorBoolLogic, pri: 28},
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
	tokenizer.T_COALESCE:            &operatorInternalDetails{pri: 23, skipA: true, op: operatorCoalesce},
	tokenizer.T_COALESCE_EQUAL:      &operatorInternalDetails{write: true, skipA: true, pri: 25, op: operatorCoalesceAssign},
	tokenizer.T_INC:                 &operatorInternalDetails{op: operatorIncDec, pri: 11},
	tokenizer.T_DEC:                 &operatorInternalDetails{op: operatorIncDec, pri: 11},
	tokenizer.T_PIPE:                &operatorInternalDetails{op: operatorPipe, pri: 24},
	tokenizer.Rune('@'):             &operatorInternalDetails{pri: 11, op: operatorSilence},

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
	if r.a != nil {
		err := r.a.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err := w.Write([]byte(operatorSymbol(r.op)))
	if err != nil {
		return err
	}
	if r.b != nil {
		err = r.b.Dump(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// operatorSymbol returns the PHP source representation of an operator.
func operatorSymbol(op tokenizer.ItemType) string {
	switch op {
	case tokenizer.T_BOOLEAN_AND:
		return " && "
	case tokenizer.T_BOOLEAN_OR:
		return " || "
	case tokenizer.T_LOGICAL_AND:
		return " and "
	case tokenizer.T_LOGICAL_OR:
		return " or "
	case tokenizer.T_LOGICAL_XOR:
		return " xor "
	case tokenizer.T_IS_EQUAL:
		return " == "
	case tokenizer.T_IS_NOT_EQUAL:
		return " != "
	case tokenizer.T_IS_IDENTICAL:
		return " === "
	case tokenizer.T_IS_NOT_IDENTICAL:
		return " !== "
	case tokenizer.T_IS_SMALLER_OR_EQUAL:
		return " <= "
	case tokenizer.T_IS_GREATER_OR_EQUAL:
		return " >= "
	case tokenizer.T_SPACESHIP:
		return " <=> "
	case tokenizer.T_SL:
		return " << "
	case tokenizer.T_SR:
		return " >> "
	case tokenizer.T_COALESCE:
		return " ?? "
	case tokenizer.T_CONCAT_EQUAL:
		return " .= "
	case tokenizer.T_PLUS_EQUAL:
		return " += "
	case tokenizer.T_MINUS_EQUAL:
		return " -= "
	case tokenizer.T_MUL_EQUAL:
		return " *= "
	case tokenizer.T_DIV_EQUAL:
		return " /= "
	case tokenizer.T_MOD_EQUAL:
		return " %= "
	case tokenizer.T_AND_EQUAL:
		return " &= "
	case tokenizer.T_OR_EQUAL:
		return " |= "
	case tokenizer.T_XOR_EQUAL:
		return " ^= "
	case tokenizer.T_SL_EQUAL:
		return " <<= "
	case tokenizer.T_SR_EQUAL:
		return " >>= "
	case tokenizer.T_POW_EQUAL:
		return " **= "
	case tokenizer.T_COALESCE_EQUAL:
		return " ??= "
	case tokenizer.T_POW:
		return " ** "
	case tokenizer.T_INC:
		return "++"
	case tokenizer.T_DEC:
		return "--"
	case tokenizer.T_INSTANCEOF:
		return " instanceof "
	case tokenizer.T_BOOL_CAST:
		return "(bool)"
	case tokenizer.T_INT_CAST:
		return "(int)"
	case tokenizer.T_DOUBLE_CAST:
		return "(float)"
	case tokenizer.T_STRING_CAST:
		return "(string)"
	case tokenizer.T_ARRAY_CAST:
		return "(array)"
	case tokenizer.T_OBJECT_CAST:
		return "(object)"
	case tokenizer.Rune('!'):
		return "!"
	case tokenizer.Rune('@'):
		return "@"
	default:
		// Single-character operators like +, -, *, /, %, etc.
		if r := op.Rune(); r != 0 {
			return " " + string(r) + " "
		}
		return op.Name()
	}
}

func spawnOperator(ctx phpv.Context, op tokenizer.ItemType, a, b phpv.Runnable, l *phpv.Loc) (phpv.Runnable, error) {
	var err error
	opD, ok := operatorList[op]
	if !ok {
		return nil, l.Errorf(ctx, phpv.E_COMPILE_ERROR, "invalid operator %s", op)
	}

	// Compile-time check: Cannot re-assign $this
	if opD.write {
		if rv, ok := a.(*runVariable); ok && rv.v == "this" {
			phpErr := l.Errorf(ctx, phpv.E_COMPILE_ERROR, "Cannot re-assign $this")
			ctx.Global().LogError(phpErr)
			return nil, phpv.ExitError(255)
		}
		// Cannot assign to a class constant
		if _, ok := a.(*runClassStaticObjRef); ok {
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("syntax error, unexpected token \"::\""),
				Code: phpv.E_PARSE,
				Loc:  l,
			}
			ctx.Global().LogError(phpErr)
			return nil, phpv.ExitError(255)
		}
		// Cannot use nullsafe operator in write context (assignment, +=, ++, etc.)
		if containsNullSafe(a) {
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("Can't use nullsafe operator in write context"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
			ctx.Global().LogError(phpErr)
			return nil, phpv.ExitError(255)
		}
	}

	// Compile-time check: Cannot use nullsafe for pre/post increment/decrement
	if (op == tokenizer.T_INC || op == tokenizer.T_DEC) && a != nil && containsNullSafe(a) {
		phpErr := &phpv.PhpError{
			Err:  fmt.Errorf("Can't use nullsafe operator in write context"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  l,
		}
		ctx.Global().LogError(phpErr)
		return nil, phpv.ExitError(255)
	}
	if (op == tokenizer.T_INC || op == tokenizer.T_DEC) && b != nil && containsNullSafe(b) {
		phpErr := &phpv.PhpError{
			Err:  fmt.Errorf("Can't use nullsafe operator in write context"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  l,
		}
		ctx.Global().LogError(phpErr)
		return nil, phpv.ExitError(255)
	}

	// Compile-time check: ??= with [] (empty index) is not allowed
	if op == tokenizer.T_COALESCE_EQUAL {
		if ac, ok := a.(*runArrayAccess); ok && ac.offset == nil {
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use [] for reading"),
				Code: phpv.E_ERROR,
				Loc:  l,
			}
			ctx.Global().LogError(phpErr)
			return nil, phpv.ExitError(255)
		}
		// ??= on function call result with array access is not allowed
		if ac, ok := a.(*runArrayAccess); ok {
			lhs := ac.value
			for {
				if innerAc, ok := lhs.(*runArrayAccess); ok {
					lhs = innerAc.value
				} else {
					break
				}
			}
			switch lhs.(type) {
			case *runnableFunctionCall, *runnableFunctionCallRef:
				phpErr := &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use result of built-in function in write context"),
					Code: phpv.E_ERROR,
					Loc:  l,
				}
				ctx.Global().LogError(phpErr)
				return nil, phpv.ExitError(255)
			}
		}
	}

	// Short list syntax: [$a, $b] = expr → convert array literal to destructure target
	if opD.write {
		if arr, ok := a.(*runArray); ok {
			dest := arrayToDestructure(arr)
			if dest != nil {
				a = dest
			} else {
				// Spread operator in destructuring is not supported
				phpErr := &phpv.PhpError{
					Err:  fmt.Errorf("Spread operator is not supported in assignments"),
					Code: phpv.E_ERROR,
					Loc:  arr.l,
				}
				ctx.Global().LogError(phpErr)
				return nil, phpv.ExitError(255)
			}
		}
	}

	if top, ok := b.(*runnableIf); ok && top.ternary {
		rop, isop := a.(*runOperator)
		if (!isop && opD.pri <= ternaryPri) || (isop && rop.opD.pri <= ternaryPri) {
			// For short ternary (?:), yes and cond point to the same Runnable.
			// Track whether yes needs to be updated to match the new cond.
			top.cond, err = spawnOperator(ctx, op, a, top.cond, l)
			if err != nil {
				return nil, err
			}
			if top.shortTernary {
				top.yes = top.cond
			}
			return top, nil
		}
	}

	if rop, isop := a.(*runOperator); isop {
		// Don't restructure if rop is a unary operator (a == nil means unary prefix).
		// Unary operators always bind tightly to their operand regardless of priority.
		if rop.a != nil && opD.pri < rop.opD.pri {
			// need to go down one level values
			rop.b, err = spawnOperator(ctx, op, rop.b, b, l)
			if err != nil {
				return nil, err
			}
			return rop, nil
		}
	}

	final := &runOperator{op: op, opD: opD, a: a, b: b, l: l}
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
		// The @ operator in PHP saves the current error_reporting value,
		// sets it to 0, evaluates the expression, then restores it.
		// If error_reporting() is called explicitly during the expression,
		// it modifies the global value directly. The restore at the end of @
		// always restores to the saved value.
		//
		// We save/restore at the global level so that explicit calls to
		// error_reporting() within the @ scope take immediate effect
		// (allowing warnings to be emitted if the level is raised).
		savedER := ctx.Global().GetConfig("error_reporting", phpv.ZInt(0).ZVal())
		ctx.Global().SetLocalConfig("error_reporting", phpv.ZInt(0).ZVal())
		defer func() {
			currentER := ctx.Global().GetConfig("error_reporting", phpv.ZInt(0).ZVal()).AsInt(ctx)
			// Only restore if error_reporting() was NOT explicitly called
			// during the @ scope (i.e., current value still has only fatal
			// error bits or is 0).
			const fatalMask = int64(phpv.E_ERROR | phpv.E_CORE_ERROR | phpv.E_COMPILE_ERROR | phpv.E_USER_ERROR | phpv.E_RECOVERABLE_ERROR | phpv.E_PARSE)
			if int64(currentER) & ^fatalMask == 0 {
				ctx.Global().SetLocalConfig("error_reporting", savedER)
			}
		}()
	}

	// For plain assignment (=), evaluate LHS sub-expressions for side effects
	// BEFORE evaluating the RHS. PHP evaluates LHS target expressions (array
	// indices, variable-variable names) before the RHS value expression.
	if op.write && op.op == nil && r.a != nil {
		if pw, ok := r.a.(phpv.WritePreparable); ok {
			if err = pw.PrepareWrite(ctx); err != nil {
				return nil, err
			}
		}
	}

	// For compound write ops (.= += etc.), enable container caching on ArrayAccess LHS
	// so WriteValue doesn't re-evaluate the container chain (avoiding extra offsetGet calls)
	if op.write && op.op != nil {
		if ac, ok := r.a.(*runArrayAccess); ok {
			ac.compoundCache = true
		}
	}

	// For null coalescing (?? and ??=), use isset-like existence checking
	// to determine if the LHS value is "set". This correctly handles:
	// - undefined variables → treat as null
	// - out-of-bounds string offsets → treat as null
	// - ArrayAccess offsetExists returning false → treat as null
	// - null values → treat as null
	if r.op == tokenizer.T_COALESCE || r.op == tokenizer.T_COALESCE_EQUAL {
		// For ??=, prepare the write target first to cache offset expressions.
		// This ensures array offsets like id($foo) are evaluated only once.
		if r.op == tokenizer.T_COALESCE_EQUAL {
			if pw, ok := r.a.(phpv.WritePreparable); ok {
				if err = pw.PrepareWrite(ctx); err != nil {
					return nil, err
				}
			}
		}
		if r.a != nil {
			exists, checkErr := checkExistence(ctx, r.a, false)
			if checkErr != nil {
				// checkExistence doesn't handle non-variable expressions
				// (literals, function calls, etc.). For ??, evaluate directly
				// and check for null. For ??=, generate a write-context error.
				if r.op == tokenizer.T_COALESCE_EQUAL {
					// Provide a meaningful PHP error message for ??= on non-writable
					what := "expression"
					switch r.a.(type) {
					case *runnableFunctionCall:
						what = "function return value"
					case *runnableFunctionCallRef:
						what = "function return value"
					case *runObjectFunc:
						what = "method return value"
					}
					return nil, ctx.Errorf("Can't use %s in write context", what)
				}
				// For ??, evaluate the LHS directly
				a, err = r.a.Run(ctx)
				if err != nil {
					// Don't swallow fatal errors (e.g. "Cannot use [] for reading")
					if isFatalPhpError(err) {
						return nil, err
					}
					a = nil
					err = nil
				}
			} else if exists {
				a, err = r.a.Run(ctx)
				if err != nil {
					// Don't swallow fatal errors
					if isFatalPhpError(err) {
						return nil, err
					}
					a = nil
					err = nil
				}
			}
			if a != nil && !a.Nude().IsNull() {
				return a, nil
			}
		}
		// Fall through to evaluate RHS
		if r.b != nil {
			b, err = r.b.Run(ctx)
			if err != nil {
				return nil, err
			}
		}
		res = b
		if op.write {
			w, ok := r.a.(phpv.Writable)
			if !ok {
				// Provide a meaningful PHP error message
				what := "expression"
				switch r.a.(type) {
				case *runnableFunctionCall:
					what = "function return value"
				case *runObjectFunc:
					what = "method return value"
				}
				return nil, ctx.Errorf("Can't use %s in write context", what)
			}
			if !res.IsRef() {
				res = res.ZVal()
			}
			return res, w.WriteValue(ctx, res)
		}
		return res, nil
	}

	// read a and b
	if r.a != nil && !(op.write && op.op == nil) {
		// For ++/-- on object properties, set incDecCtx to get proper error message
		if r.op == tokenizer.T_INC || r.op == tokenizer.T_DEC {
			if ov, ok := r.a.(*runObjectVar); ok {
				ov.incDecCtx = true
				defer func() { ov.incDecCtx = false }()
			}
		}
		a, err = r.a.Run(ctx)
		if err != nil {
			return nil, err
		}
		// For .= (concat-assign), snapshot the LHS value before evaluating
		// the RHS. Side effects during RHS evaluation (ob callbacks,
		// error handlers, __toString) can modify the variable that a points
		// to, but PHP's concat_function captures the LHS string first.
		// Use NewZVal to create a lightweight snapshot (shares the Go value
		// but disconnects from the variable's ZVal pointer).
		if r.op == tokenizer.T_CONCAT_EQUAL && a != nil {
			a = phpv.NewZVal(a.Value())
		}
	}

	// PHP 8: compound assignment operators (+=, -=, .=, etc.) are not allowed on string offsets
	if op.write && op.op != nil && r.a != nil {
		if ac, isAA := r.a.(*runArrayAccess); isAA {
			if ac.cachedContainer != nil && ac.cachedContainer.GetType() == phpv.ZtString {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use assign-op operators with string offsets")
			}
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
		// For ++/-- on object properties (postfix), set incDecCtx
		if r.op == tokenizer.T_INC || r.op == tokenizer.T_DEC {
			if ov, ok := r.b.(*runObjectVar); ok {
				ov.incDecCtx = true
				defer func() { ov.incDecCtx = false }()
			}
		}
		b, err = r.b.Run(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Handle unary minus/plus on numeric types directly (r.a == nil = prefix unary).
	// For unary minus on floats, we must use Go negation (not 0-x) to produce
	// negative zero: -0.0 must yield float(-0), but 0.0-0.0 yields float(0).
	// Only apply this fast path for already-numeric types; strings/bools/null
	// still go through the standard numeric conversion for proper warnings.
	if r.a == nil && (r.op == tokenizer.Rune('-') || r.op == tokenizer.Rune('+')) {
		bType := b.GetType()
		// Check for object with HandleDoOperation (e.g., GMP unary -/+)
		if bType == phpv.ZtObject {
			if obj, ok := b.Value().(phpv.ZObject); ok {
				if h := obj.GetClass().Handlers(); h != nil && h.HandleDoOperation != nil {
					return h.HandleDoOperation(ctx, int(r.op), a, b)
				}
			}
		}
		if bType == phpv.ZtInt || bType == phpv.ZtFloat {
			if r.op == tokenizer.Rune('-') {
				switch v := b.Value().(type) {
				case phpv.ZInt:
					if v == 0 {
						return phpv.ZInt(0).ZVal(), nil
					}
					if v == math.MinInt64 {
						return phpv.ZFloat(-float64(v)).ZVal(), nil
					}
					return (-v).ZVal(), nil
				case phpv.ZFloat:
					return (-v).ZVal(), nil
				}
			}
			return b, nil
		}
	}

	// Handle array + array union BEFORE numeric conversion
	if op.numeric {
		aType := a.GetType()
		bType := b.GetType()
		isPlus := r.op == tokenizer.Rune('+') || r.op == tokenizer.T_PLUS_EQUAL

		// array + array = array union
		if isPlus && aType == phpv.ZtArray && bType == phpv.ZtArray {
			result := a.AsArray(ctx).Dup()
			bArr := b.AsArray(ctx)
			for k, v := range bArr.Iterate(ctx) {
				if exists, _ := result.OffsetExists(ctx, k); !exists {
					result.OffsetSet(ctx, k, v)
				}
			}
			return result.ZVal(), nil
		}

		// Bitwise operators on strings: PHP operates on the raw bytes directly
		// (no numeric conversion). Skip the numeric conversion below and let
		// operatorMathLogic handle string operands. Don't return early so that
		// compound assignment write-back (op.write) still runs.
		isBitwiseOp := r.op == tokenizer.Rune('|') || r.op == tokenizer.Rune('^') ||
			r.op == tokenizer.Rune('&') || r.op == tokenizer.Rune('~') ||
			r.op == tokenizer.T_OR_EQUAL || r.op == tokenizer.T_XOR_EQUAL ||
			r.op == tokenizer.T_AND_EQUAL
		skipNumericConversion := isBitwiseOp && aType == phpv.ZtString && (bType == phpv.ZtString || r.op == tokenizer.Rune('~'))
		_ = skipNumericConversion // used below

		if !skipNumericConversion {
			// PHP 8: throw TypeError for unsupported operand types in arithmetic
			if aType == phpv.ZtArray || bType == phpv.ZtArray {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Unsupported operand types: %s %s %s", phpTypeName(a), r.op.OpString(), phpTypeName(b)))
			}
			// Check for objects with HandleDoOperation (e.g., GMP operator overloading)
			if aType == phpv.ZtObject || bType == phpv.ZtObject {
				var handler func(phpv.Context, int, *phpv.ZVal, *phpv.ZVal) (*phpv.ZVal, error)
				if aType == phpv.ZtObject {
					if obj, ok := a.Value().(phpv.ZObject); ok {
						if h := obj.GetClass().Handlers(); h != nil && h.HandleDoOperation != nil {
							handler = h.HandleDoOperation
						}
					}
				}
				if handler == nil && bType == phpv.ZtObject {
					if obj, ok := b.Value().(phpv.ZObject); ok {
						if h := obj.GetClass().Handlers(); h != nil && h.HandleDoOperation != nil {
							handler = h.HandleDoOperation
						}
					}
				}
				if handler != nil {
					handlerRes, handlerErr := handler(ctx, int(r.op), a, b)
					if handlerErr != nil {
						return nil, handlerErr
					}
					// For compound assignment operators, write back
					if op.write {
						w, ok := r.a.(phpv.Writable)
						if ok {
							return handlerRes, w.WriteValue(ctx, handlerRes.ZVal())
						}
					}
					return handlerRes, nil
				}
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Unsupported operand types: %s %s %s", phpTypeName(a), r.op.OpString(), phpTypeName(b)))
			}
			if aType == phpv.ZtResource || bType == phpv.ZtResource {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Unsupported operand types: %s %s %s", phpTypeName(a), r.op.OpString(), phpTypeName(b)))
			}

			// PHP 8: handle non-numeric strings in arithmetic
			// - Completely non-numeric ("hello"): TypeError
			// - Leading numeric ("123abc"): Warning + use numeric part
			// - Fully numeric ("123"): no warning
			if aType == phpv.ZtString {
				s := string(a.Value().(phpv.ZString))
				if !isLeadingNumeric(s) {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Unsupported operand types: %s %s %s", phpTypeName(a), r.op.OpString(), phpTypeName(b)))
				}
				if !isNumericString(s) {
					if err := ctx.Warn("A non-numeric value encountered", logopt.Data{Loc: r.l, NoFuncName: true}); err != nil {
						return nil, err
					}
				}
			}
			if bType == phpv.ZtString {
				s := string(b.Value().(phpv.ZString))
				if !isLeadingNumeric(s) {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Unsupported operand types: %s %s %s", phpTypeName(a), r.op.OpString(), phpTypeName(b)))
				}
				if !isNumericString(s) {
					if err := ctx.Warn("A non-numeric value encountered", logopt.Data{Loc: r.l, NoFuncName: true}); err != nil {
						return nil, err
					}
				}
			}
			a, _ = a.AsNumeric(ctx)
			b, _ = b.AsNumeric(ctx)

			// normalize types - for bitwise/shift ops, always use int
			isBitwise := r.op == tokenizer.T_SL || r.op == tokenizer.T_SR ||
				r.op == tokenizer.T_SL_EQUAL || r.op == tokenizer.T_SR_EQUAL ||
				r.op == tokenizer.Rune('|') || r.op == tokenizer.Rune('^') ||
				r.op == tokenizer.Rune('&') || r.op == tokenizer.Rune('%') ||
				r.op == tokenizer.T_OR_EQUAL || r.op == tokenizer.T_XOR_EQUAL ||
				r.op == tokenizer.T_AND_EQUAL || r.op == tokenizer.T_MOD_EQUAL
			if isBitwise {
				a, err = implicitToInt(ctx, a)
				if err != nil {
					return nil, err
				}
				b, err = implicitToInt(ctx, b)
				if err != nil {
					return nil, err
				}
			} else if a.GetType() == phpv.ZtFloat || b.GetType() == phpv.ZtFloat {
				a, _ = a.As(ctx, phpv.ZtFloat)
				b, _ = b.As(ctx, phpv.ZtFloat)
			} else {
				a, _ = a.As(ctx, phpv.ZtInt)
				b, _ = b.As(ctx, phpv.ZtInt)
			}
		}
	}

	// For ++/-- on overloaded ArrayAccess (without &offsetGet), dup the value
	// before operatorIncDec so doInc doesn't modify the original value
	// returned by offsetGet. If &offsetGet returns by reference, doInc should
	// modify in-place through the reference.
	// Note: ObjectGet/static prop Read already return detached copies, so no dup needed.
	if r.op == tokenizer.T_INC || r.op == tokenizer.T_DEC {
		if r.a != nil {
			if ac, isAA := r.a.(*runArrayAccess); isAA && ac.lastContainerIsOverloaded && !ac.lastContainerOffsetGetReturnsRef {
				a = a.Dup()
			}
		} else if r.b != nil {
			if ac, isAA := r.b.(*runArrayAccess); isAA && ac.lastContainerIsOverloaded && !ac.lastContainerOffsetGetReturnsRef {
				b = b.Dup()
			}
		}
	}

	// For ++/-- on static properties, check asymmetric set visibility
	// before modifying, and use "Cannot indirectly modify" message.
	if r.op == tokenizer.T_INC || r.op == tokenizer.T_DEC {
		target := r.a
		if target == nil {
			target = r.b
		}
		if sv, isSV := target.(*runClassStaticVarRef); isSV {
			classNameVal, err2 := sv.className.Run(ctx)
			if err2 == nil {
				class, err2 := ctx.Global().GetClass(ctx, classNameVal.AsString(ctx), true)
				if err2 == nil {
					zc := class.(*phpobj.ZClass)
					if visErr := phpobj.CheckStaticPropIndirectSetVisibility(ctx, zc, sv.varName); visErr != "" {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, visErr)
					}
				}
			}
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

	// For ++/-- operators, write back the modified value.
	// doInc modifies the ZVal in-place, but for magic properties (__get/__set)
	// the returned ZVal is detached, so we need to call WriteValue to trigger __set.
	if r.op == tokenizer.T_INC || r.op == tokenizer.T_DEC {
		if r.a != nil {
			if w, ok := r.a.(phpv.Writable); ok && a != nil {
				// PHP: compound ops on ArrayAccess elements without &offsetGet
				// don't call offsetSet — emit "Indirect modification" notice.
				// But if offsetGet returns by reference, the modification went
				// through the reference, so no notice is needed and we write back.
				if ac, isAA := r.a.(*runArrayAccess); isAA && ac.lastContainerIsOverloaded {
					if ac.lastContainerOffsetGetReturnsRef {
						// &offsetGet: write back the modified value via offsetSet
						v := a.Value().ZVal()
						if err := w.WriteValue(ctx, v); err != nil {
							return nil, err
						}
					} else {
						if err := ctx.Notice("Indirect modification of overloaded element of %s has no effect", ac.lastContainerClassName, logopt.Data{Loc: r.l, NoFuncName: true}); err != nil {
							return nil, err
						}
					}
				} else {
					// Create a clean ZVal without variable name to avoid spurious warnings
					v := a.Value().ZVal()
					if err := w.WriteValue(ctx, v); err != nil {
						return nil, err
					}
				}
			}
		} else if r.b != nil {
			if w, ok := r.b.(phpv.Writable); ok && b != nil {
				if ac, isAA := r.b.(*runArrayAccess); isAA && ac.lastContainerIsOverloaded {
					if ac.lastContainerOffsetGetReturnsRef {
						v := b.Value().ZVal()
						if err := w.WriteValue(ctx, v); err != nil {
							return nil, err
						}
					} else {
						if err := ctx.Notice("Indirect modification of overloaded element of %s has no effect", ac.lastContainerClassName, logopt.Data{Loc: r.l, NoFuncName: true}); err != nil {
							return nil, err
						}
					}
				} else {
					v := b.Value().ZVal()
					if err := w.WriteValue(ctx, v); err != nil {
						return nil, err
					}
				}
			}
		}
		return res, nil
	}

	if op.write {
		w, ok := r.a.(phpv.Writable)
		if !ok {
			// Provide a meaningful PHP error message instead of Go struct dump
			what := "expression"
			switch r.a.(type) {
			case *runnableFunctionCall:
				what = "function return value"
			case *runObjectFunc:
				what = "method return value"
			}
			return nil, ctx.Errorf("Can't use %s in write context", what)
		}

		// Check for reference assignment to ArrayAccess element
		if res.IsRef() {
			if acc, isAA := r.a.(*runArrayAccess); isAA {
				if acc.IsOverloaded(ctx) {
					ctx.Notice("Indirect modification of overloaded element of %s has no effect",
						acc.lastContainerClassName, logopt.NoFuncName(true))
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						"Cannot assign by reference to an array dimension of an object")
				}
			}
		}

		// The PHP documentation states that the array's internal
		// pointer is reset when assigning to another variable
		// AND the internal pointer is at the end.
		// The following code handles that special case.
		if res.GetType() == phpv.ZtArray {
			res.AsArray(ctx).MainIterator().ResetIfEnd(ctx)
		}

		if !res.IsRef() {
			res = res.ZVal()
		}

		// Track reference aliases: when storing a reference value (from =&),
		// increment the inner ZVal's refCount so UnRefIfAlone knows
		// not to unwrap compound writable by-ref args.
		if res.IsRef() {
			res.RefInner()
		}

		return res, w.WriteValue(ctx, res)
	}

	return res, nil
}

func operatorAppend(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	var err error
	a, err = a.As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}
	b, err = b.As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	return (a.AsString(ctx) + b.AsString(ctx)).ZVal(), nil
}

func operatorNot(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	b, _ = b.As(ctx, phpv.ZtBool)

	return (!b.Value().(phpv.ZBool)).ZVal(), nil
}

func doInc(ctx phpv.Context, v *phpv.ZVal, inc bool) error {
	if v == nil {
		return nil
	}
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

		// PHP 8.3: Incrementing non-numeric strings is deprecated
		if err := ctx.Deprecated("Increment on non-numeric string is deprecated, use str_increment() instead", logopt.NoFuncName(true)); err != nil {
			return err
		}

		// do string increment...
		// Handle empty string: "" → "1"
		if len(s) == 0 {
			v.Set(phpv.ZString("1").ZVal())
			return nil
		}

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
				return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "Division by zero")
			}
			if a == math.MinInt64 && b == -1 {
				// INT64_MIN / -1 overflows, return as float
				res = phpv.ZFloat(a) / phpv.ZFloat(b)
			} else if a%b != 0 {
				// this is not going to be an int result
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
			// PHP 8.4: 0 ** negative is deprecated
			if a == 0 && b < 0 {
				ctx.Deprecated("Power of base 0 and negative exponent is deprecated", logopt.NoFuncName(true))
			}
			if b >= 0 {
				// Try to compute as integer first
				result := phpv.ZFloat(math.Pow(float64(a), float64(b)))
				intResult := phpv.ZInt(result)
				if phpv.ZFloat(intResult) == result && result >= math.MinInt64 && result <= math.MaxInt64 {
					res = intResult
				} else {
					res = result
				}
			} else {
				res = phpv.ZFloat(math.Pow(float64(a), float64(b)))
			}
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
			bv := b.Value().(phpv.ZFloat)
			if bv == 0 {
				return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "Division by zero")
			}
			res = a.Value().(phpv.ZFloat) / bv
		case tokenizer.T_MUL_EQUAL, tokenizer.Rune('*'):
			res = a.Value().(phpv.ZFloat) * b.Value().(phpv.ZFloat)
		case tokenizer.T_POW, tokenizer.T_POW_EQUAL:
			af := a.Value().(phpv.ZFloat)
			bf := b.Value().(phpv.ZFloat)
			// PHP 8.4: 0 ** negative is deprecated
			if af == 0 && bf < 0 {
				ctx.Deprecated("Power of base 0 and negative exponent is deprecated", logopt.NoFuncName(true))
			}
			res = phpv.ZFloat(math.Pow(float64(af), float64(bf)))
		}
		return res.ZVal(), nil
	default:
		return nil, ctx.Errorf("todo operator type unsupported %s", a.GetType())
	}
}

func operatorBoolLogic(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	switch op {
	case tokenizer.T_BOOLEAN_AND, tokenizer.T_LOGICAL_AND:
		return (a.AsBool(ctx) && b.AsBool(ctx)).ZVal(), nil
	case tokenizer.T_BOOLEAN_OR, tokenizer.T_LOGICAL_OR:
		return (a.AsBool(ctx) || b.AsBool(ctx)).ZVal(), nil
	default:
		return nil, ctx.Errorf("unsupported boolean operator %s", op)
	}
}

func operatorCoalesce(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	if a != nil && !a.IsNull() {
		return a, nil
	}
	return b, nil
}

func operatorCoalesceAssign(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	// Short-circuit already handled above; if we get here, a was null
	return b, nil
}

func operatorSilence(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	// The @ operator silences errors. The error_reporting config is already set
	// in the Run() method before this is called, so we just return the value.
	return b, nil
}

func operatorLogicalXor(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	ab := a.AsBool(ctx)
	bb := b.AsBool(ctx)
	return phpv.ZBool(ab != bb).ZVal(), nil
}

func operatorMathLogic(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	// Check for unary ~ on objects with HandleDoOperation (e.g., GMP)
	if a == nil && b != nil && b.GetType() == phpv.ZtObject {
		if obj, ok := b.Value().(phpv.ZObject); ok {
			if h := obj.GetClass().Handlers(); h != nil && h.HandleDoOperation != nil {
				return h.HandleDoOperation(ctx, int(op), nil, b)
			}
		}
	}

	if a == nil {
		a = b
	}

	switch a.Value().GetType() {
	case phpv.ZtBool, phpv.ZtNull:
		// Boolean and null values should be converted to int for bitwise ops
		a, _ = a.As(ctx, phpv.ZtInt)
		b, _ = b.As(ctx, phpv.ZtInt)
		return operatorMathLogic(ctx, op, a, b)
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
			bv := b.Value().(phpv.ZInt)
			if bv == 0 {
				return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "Modulo by zero")
			}
			av := a.Value().(phpv.ZInt)
			if av == math.MinInt64 && bv == -1 {
				res = phpv.ZInt(0)
			} else {
				res = av % bv
			}
		case tokenizer.Rune('~'):
			res = ^b.Value().(phpv.ZInt)
		case tokenizer.T_SL, tokenizer.T_SL_EQUAL:
			bv := b.Value().(phpv.ZInt)
			if bv < 0 {
				return nil, phpobj.ThrowError(ctx, phpobj.ArithmeticError, "Bit shift by negative number")
			}
			if bv >= 64 {
				res = 0
			} else {
				res = a.Value().(phpv.ZInt) << uint(bv)
			}
		case tokenizer.T_SR, tokenizer.T_SR_EQUAL:
			bv := b.Value().(phpv.ZInt)
			if bv < 0 {
				return nil, phpobj.ThrowError(ctx, phpobj.ArithmeticError, "Bit shift by negative number")
			}
			if bv >= 64 {
				if a.Value().(phpv.ZInt) < 0 {
					res = -1
				} else {
					res = 0
				}
			} else {
				res = a.Value().(phpv.ZInt) >> uint(bv)
			}
		}
		return res.ZVal(), nil
	case phpv.ZtFloat:
		// need to convert to int (implicit conversion emits deprecation for precision loss)
		var err error
		a, err = implicitToInt(ctx, a)
		if err != nil {
			return nil, err
		}
		b, err = implicitToInt(ctx, b)
		if err != nil {
			return nil, err
		}
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
	case phpv.ZtObject:
		// For objects, === checks identity (same instance).
		// We compare hash tables because ZObject.Unwrap() creates a new struct
		// that shares the same hash table but has a different Go pointer.
		aObj := a.AsObject(ctx)
		bObj := b.AsObject(ctx)
		if aObj != nil && bObj != nil {
			res = aObj.HashTable() == bObj.HashTable()
		} else {
			res = aObj == nil && bObj == nil
		}
	case phpv.ZtArray:
		// For arrays, === checks same keys and values in same order with strict comparison
		res = a.AsArray(ctx).StrictEquals(ctx, b.AsArray(ctx))
	default:
		return nil, ctx.Errorf("unsupported compare type %s", a.GetType())
	}

	if op == tokenizer.T_IS_NOT_IDENTICAL {
		res = !res
	}

	return phpv.ZBool(res).ZVal(), nil
}

// operatorPipe implements the |> pipe operator (PHP 8.5).
// $expr |> callable passes $expr as the first argument to callable.
func operatorPipe(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	// b is the callable (right side). Resolve it to a Callable.
	if PipeResolveCallable == nil {
		return nil, fmt.Errorf("pipe operator not available: callable resolver not initialized")
	}
	callable, err := PipeResolveCallable(ctx, b)
	if err != nil {
		return nil, err
	}
	// Call the callable with a as the first argument
	return ctx.CallZVal(ctx, callable, []*phpv.ZVal{a})
}

// PipeResolveCallable is set from the core package to avoid circular imports.
var PipeResolveCallable func(ctx phpv.Context, v *phpv.ZVal) (phpv.Callable, error)

func operatorCompare(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	// Handle array comparisons first - arrays are always greater than scalars in PHP 8
	if a.GetType() == phpv.ZtArray || b.GetType() == phpv.ZtArray {
		cmp, err := phpv.Compare(ctx, a, b)
		if err != nil {
			if errors.Is(err, phpv.ErrComparisonDepth) {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, err.Error())
			}
			return nil, err
		}
		var res bool
		switch op {
		case tokenizer.Rune('<'):
			res = cmp < 0
		case tokenizer.Rune('>'):
			res = cmp > 0
		case tokenizer.T_IS_SMALLER_OR_EQUAL:
			res = cmp <= 0
		case tokenizer.T_IS_GREATER_OR_EQUAL:
			res = cmp >= 0
		case tokenizer.T_IS_EQUAL:
			res = cmp == 0
		case tokenizer.T_IS_NOT_EQUAL:
			res = cmp != 0
		case tokenizer.T_SPACESHIP:
			return phpv.ZInt(cmp).ZVal(), nil
		}
		return phpv.ZBool(res).ZVal(), nil
	}

	// NaN is never equal to anything, including itself (IEEE 754).
	// PHP returns false for all == comparisons involving NaN, and 1 for <=>.
	if a.GetType() == phpv.ZtFloat && math.IsNaN(float64(a.Value().(phpv.ZFloat))) {
		if op == tokenizer.T_IS_EQUAL {
			return phpv.ZBool(false).ZVal(), nil
		}
		if op == tokenizer.T_IS_NOT_EQUAL {
			return phpv.ZBool(true).ZVal(), nil
		}
		if op == tokenizer.T_SPACESHIP {
			return phpv.ZInt(1).ZVal(), nil
		}
		// For <, >, <=, >= with NaN: always false
		return phpv.ZBool(false).ZVal(), nil
	}
	if b.GetType() == phpv.ZtFloat && math.IsNaN(float64(b.Value().(phpv.ZFloat))) {
		if op == tokenizer.T_IS_EQUAL {
			return phpv.ZBool(false).ZVal(), nil
		}
		if op == tokenizer.T_IS_NOT_EQUAL {
			return phpv.ZBool(true).ZVal(), nil
		}
		if op == tokenizer.T_SPACESHIP {
			return phpv.ZInt(1).ZVal(), nil
		}
		// For <, >, <=, >= with NaN: always false
		return phpv.ZBool(false).ZVal(), nil
	}

	// operator compare (< > <= >= == === != !== <=>) involve a lot of dark magic in php, unless both values are of the same type (and even so)
	// loose comparison will convert number-y looking strings into numbers, etc
	var ia, ib *phpv.ZVal
	var aLooksInt, bLooksInt bool

	switch a.GetType() {
	case phpv.ZtInt, phpv.ZtFloat:
		ia = a
	case phpv.ZtString:
		aStr := a.Value().(phpv.ZString)
		aLooksInt = aStr.LooksInt()
		if aLooksInt || aStr.IsNumeric() {
			v, _ := aStr.AsNumeric()
			if v != nil {
				ia = phpv.NewZVal(v)
			}
		}
	}

	switch b.GetType() {
	case phpv.ZtInt, phpv.ZtFloat:
		ib = b
	case phpv.ZtString:
		bStr := b.Value().(phpv.ZString)
		bLooksInt = bStr.LooksInt()
		if bLooksInt || bStr.IsNumeric() {
			v, _ := bStr.AsNumeric()
			if v != nil {
				ib = phpv.NewZVal(v)
			}
		}
	}

	// PHP 8: when both operands are strings, only do numeric comparison
	// if BOTH are numeric strings. If either is non-numeric, use string comparison.
	aIsNonNumericString := a.GetType() == phpv.ZtString && ia == nil
	bIsNonNumericString := b.GetType() == phpv.ZtString && ib == nil
	if a.GetType() == phpv.ZtString && b.GetType() == phpv.ZtString {
		if aIsNonNumericString || bIsNonNumericString {
			goto CompareStrings
		}
		// When both strings look like integers but at least one overflowed to float,
		// use arbitrary-precision integer comparison (float64 loses precision for large ints).
		if aLooksInt && bLooksInt {
			if ia.GetType() == phpv.ZtFloat || ib.GetType() == phpv.ZtFloat {
				cmp := phpv.CompareIntStrings(string(a.Value().(phpv.ZString)), string(b.Value().(phpv.ZString)))
				var res phpv.Val
				switch op {
				case tokenizer.Rune('<'):
					res = phpv.ZBool(cmp < 0)
				case tokenizer.Rune('>'):
					res = phpv.ZBool(cmp > 0)
				case tokenizer.T_IS_SMALLER_OR_EQUAL:
					res = phpv.ZBool(cmp <= 0)
				case tokenizer.T_IS_GREATER_OR_EQUAL:
					res = phpv.ZBool(cmp >= 0)
				case tokenizer.T_IS_EQUAL:
					res = phpv.ZBool(cmp == 0)
				case tokenizer.T_IS_NOT_EQUAL:
					res = phpv.ZBool(cmp != 0)
				case tokenizer.T_SPACESHIP:
					res = phpv.ZInt(cmp)
				default:
					return nil, ctx.Errorf("unsupported operator %s", op)
				}
				return res.ZVal(), nil
			}
		}
	}

	// PHP 8: when comparing a number with a non-numeric string,
	// convert the number to string and do string comparison
	if (a.GetType() == phpv.ZtInt || a.GetType() == phpv.ZtFloat) && bIsNonNumericString {
		goto CompareStrings
	}
	if (b.GetType() == phpv.ZtInt || b.GetType() == phpv.ZtFloat) && aIsNonNumericString {
		goto CompareStrings
	}

	// PHP 8: string vs object -> no numeric conversion, objects are greater than strings
	// But objects with HandleCast (like GMP) can be compared to strings
	if (a.GetType() == phpv.ZtString && b.GetType() == phpv.ZtObject) ||
		(a.GetType() == phpv.ZtObject && b.GetType() == phpv.ZtString) {
		// Check if the object has HandleCast for numeric comparison
		var objVal *phpv.ZVal
		if a.GetType() == phpv.ZtObject {
			ao := a.AsObject(ctx)
			if h := ao.GetClass().Handlers(); h != nil && h.HandleCast != nil {
				objVal = a
			}
		}
		if b.GetType() == phpv.ZtObject {
			bo := b.AsObject(ctx)
			if h := bo.GetClass().Handlers(); h != nil && h.HandleCast != nil {
				objVal = b
			}
		}
		if objVal == nil {
			// Skip numeric comparison, fall through to object > scalar comparison
			goto ObjectScalarCompare
		}
		// Let the numeric comparison path handle it - the HandleCast will be used there
	}

	// PHP 8: when either operand is bool or null, always use bool comparison
	// (even if the other operand is numeric). This must come before numeric comparison.
	if a.GetType() == phpv.ZtBool || b.GetType() == phpv.ZtBool || a.GetType() == phpv.ZtNull || b.GetType() == phpv.ZtNull {
		return operatorCompareBool(ctx, op, a, b)
	}

	if ia != nil || ib != nil {
		// if either part is a numeric, force the other one as numeric too and go through comparison
		if ia == nil {
			if a.GetType() == phpv.ZtObject {
				ao := a.AsObject(ctx)
				// Check for HandleCast (e.g., GMP) before generic object-to-numeric
				if h := ao.GetClass().Handlers(); h != nil && h.HandleCast != nil {
					targetType := phpv.ZtInt
					if ib != nil && ib.GetType() == phpv.ZtFloat {
						targetType = phpv.ZtFloat
					}
					val, err := h.HandleCast(ctx, ao, targetType)
					if err == nil {
						ia = val.ZVal()
					}
				}
				if ia == nil {
					// Object in comparison: emit Notice (not Warning) with appropriate target type
					targetType := phpv.ZtInt
					if ib != nil && ib.GetType() == phpv.ZtFloat {
						targetType = phpv.ZtFloat
					}
					ia = phpv.CompareObjectToNumeric(ctx, a, targetType)
				}
			} else {
				ia, _ = a.AsNumeric(ctx)
			}
		}
		if ib == nil {
			if b.GetType() == phpv.ZtObject {
				bo := b.AsObject(ctx)
				// Check for HandleCast (e.g., GMP) before generic object-to-numeric
				if h := bo.GetClass().Handlers(); h != nil && h.HandleCast != nil {
					targetType := phpv.ZtInt
					if ia != nil && ia.GetType() == phpv.ZtFloat {
						targetType = phpv.ZtFloat
					}
					val, err := h.HandleCast(ctx, bo, targetType)
					if err == nil {
						ib = val.ZVal()
					}
				}
				if ib == nil {
					// Object in comparison: emit Notice (not Warning) with appropriate target type
					targetType := phpv.ZtInt
					if ia != nil && ia.GetType() == phpv.ZtFloat {
						targetType = phpv.ZtFloat
					}
					ib = phpv.CompareObjectToNumeric(ctx, b, targetType)
				}
			} else {
				ib, _ = b.AsNumeric(ctx)
			}
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
			fa := ia.Value().(phpv.ZFloat)
			fb := ib.Value().(phpv.ZFloat)
			switch op {
			case tokenizer.Rune('<'):
				res = phpv.ZBool(fa < fb)
			case tokenizer.Rune('>'):
				res = phpv.ZBool(fa > fb)
			case tokenizer.T_IS_SMALLER_OR_EQUAL:
				res = phpv.ZBool(fa <= fb)
			case tokenizer.T_IS_GREATER_OR_EQUAL:
				res = phpv.ZBool(fa >= fb)
			case tokenizer.T_IS_EQUAL:
				res = phpv.ZBool(fa == fb)
			case tokenizer.T_IS_NOT_EQUAL:
				res = phpv.ZBool(fa != fb)
			case tokenizer.T_SPACESHIP:
				if fa < fb {
					res = phpv.ZInt(-1)
				} else if fa > fb {
					res = phpv.ZInt(1)
				} else {
					res = phpv.ZInt(0)
				}
			default:
				return nil, ctx.Errorf("unsupported operator %s", op)
			}
		}

		return res.ZVal(), nil
	}

	if a.GetType() == phpv.ZtNull && b.GetType() == phpv.ZtNull {
		switch op {
		case tokenizer.T_IS_NOT_EQUAL:
			return phpv.ZBool(false).ZVal(), nil
		case tokenizer.Rune('<'), tokenizer.Rune('>'):
			return phpv.ZBool(false).ZVal(), nil
		case tokenizer.T_SPACESHIP:
			return phpv.ZInt(0).ZVal(), nil
		default:
			return phpv.ZBool(true).ZVal(), nil
		}
	}

	if a.GetType() == phpv.ZtBool || b.GetType() == phpv.ZtBool || a.GetType() == phpv.ZtNull || b.GetType() == phpv.ZtNull {
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
		case tokenizer.T_SPACESHIP:
			if ab < bb {
				return phpv.ZInt(-1).ZVal(), nil
			} else if ab > bb {
				return phpv.ZInt(1).ZVal(), nil
			}
			return phpv.ZInt(0).ZVal(), nil
		default:
			return nil, ctx.Errorf("unsupported operator %s", op)
		}

		return phpv.ZBool(res).ZVal(), nil
	}

ObjectScalarCompare:
	// PHP 8: objects are greater than all scalar types
	// But objects with HandleCompare (like GMP) can use their comparison handler
	if a.GetType() == phpv.ZtObject && b.GetType() != phpv.ZtObject {
		ao := a.AsObject(ctx)
		if h := ao.GetClass().Handlers(); h != nil && h.HandleCompare != nil {
			// For GMP-like objects: try to convert scalar to same type for comparison
			// Use the Compare function which handles HandleCast
			cmp, err := phpv.Compare(ctx, a, b)
			if err == nil {
				return operatorCompareResult(op, cmp)
			}
		}
		switch op {
		case tokenizer.Rune('<'):
			return phpv.ZBool(false).ZVal(), nil
		case tokenizer.Rune('>'):
			return phpv.ZBool(true).ZVal(), nil
		case tokenizer.T_IS_SMALLER_OR_EQUAL:
			return phpv.ZBool(false).ZVal(), nil
		case tokenizer.T_IS_GREATER_OR_EQUAL:
			return phpv.ZBool(true).ZVal(), nil
		case tokenizer.T_IS_EQUAL:
			return phpv.ZBool(false).ZVal(), nil
		case tokenizer.T_IS_NOT_EQUAL:
			return phpv.ZBool(true).ZVal(), nil
		case tokenizer.T_SPACESHIP:
			return phpv.ZInt(1).ZVal(), nil
		default:
			return nil, ctx.Errorf("unsupported operator %s", op)
		}
	}
	if b.GetType() == phpv.ZtObject && a.GetType() != phpv.ZtObject {
		bo := b.AsObject(ctx)
		if h := bo.GetClass().Handlers(); h != nil && h.HandleCompare != nil {
			cmp, err := phpv.Compare(ctx, a, b)
			if err == nil {
				return operatorCompareResult(op, cmp)
			}
		}
		switch op {
		case tokenizer.Rune('<'):
			return phpv.ZBool(true).ZVal(), nil
		case tokenizer.Rune('>'):
			return phpv.ZBool(false).ZVal(), nil
		case tokenizer.T_IS_SMALLER_OR_EQUAL:
			return phpv.ZBool(true).ZVal(), nil
		case tokenizer.T_IS_GREATER_OR_EQUAL:
			return phpv.ZBool(false).ZVal(), nil
		case tokenizer.T_IS_EQUAL:
			return phpv.ZBool(false).ZVal(), nil
		case tokenizer.T_IS_NOT_EQUAL:
			return phpv.ZBool(true).ZVal(), nil
		case tokenizer.T_SPACESHIP:
			return phpv.ZInt(-1).ZVal(), nil
		default:
			return nil, ctx.Errorf("unsupported operator %s", op)
		}
	}

	// non numeric comparison for same-type non-objects
	if a.GetType() != b.GetType() {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Object comparison (both objects)
	if a.GetType() == phpv.ZtObject && b.GetType() == phpv.ZtObject {
		cmp, err := phpv.CompareObject(ctx, a.AsObject(ctx), b.AsObject(ctx))
		if err != nil {
			if errors.Is(err, phpv.ErrComparisonDepth) {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, err.Error())
			}
			return nil, err
		}
		// Handle uncomparable objects (e.g., different enum cases)
		if cmp == phpv.CompareUncomparable {
			switch op {
			case tokenizer.T_IS_EQUAL:
				return phpv.ZBool(false).ZVal(), nil
			case tokenizer.T_IS_NOT_EQUAL:
				return phpv.ZBool(true).ZVal(), nil
			default:
				// All ordered comparisons (<, >, <=, >=, <=>) return false/0
				if op == tokenizer.T_SPACESHIP {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZBool(false).ZVal(), nil
			}
		}
		switch op {
		case tokenizer.T_IS_EQUAL:
			return phpv.ZBool(cmp == 0).ZVal(), nil
		case tokenizer.T_IS_NOT_EQUAL:
			return phpv.ZBool(cmp != 0).ZVal(), nil
		case tokenizer.Rune('<'):
			return phpv.ZBool(cmp < 0).ZVal(), nil
		case tokenizer.Rune('>'):
			return phpv.ZBool(cmp > 0).ZVal(), nil
		case tokenizer.T_IS_SMALLER_OR_EQUAL:
			return phpv.ZBool(cmp <= 0).ZVal(), nil
		case tokenizer.T_IS_GREATER_OR_EQUAL:
			return phpv.ZBool(cmp >= 0).ZVal(), nil
		case tokenizer.T_SPACESHIP:
			return phpv.ZInt(cmp).ZVal(), nil
		default:
			return nil, ctx.Errorf("unsupported operator %s", op)
		}
	}

CompareStrings:
	var res bool

	if a.GetType() == phpv.ZtArray || b.GetType() == phpv.ZtArray {
		goto CompareArrays
	}
	{
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
		case tokenizer.T_SPACESHIP:
			if av < bv {
				return phpv.ZInt(-1).ZVal(), nil
			} else if av > bv {
				return phpv.ZInt(1).ZVal(), nil
			}
			return phpv.ZInt(0).ZVal(), nil
		default:
			return nil, ctx.Errorf("unsupported operator %s", op)
		}
		return phpv.ZBool(res).ZVal(), nil
	}

CompareArrays:
	switch a.Value().GetType() {
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
			case tokenizer.T_SPACESHIP:
				ca := arrA.Count(ctx)
				cb := arrB.Count(ctx)
				if ca < cb {
					return phpv.ZInt(-1).ZVal(), nil
				} else if ca > cb {
					return phpv.ZInt(1).ZVal(), nil
				}
				return phpv.ZInt(0).ZVal(), nil
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
			case tokenizer.T_SPACESHIP:
				return phpv.ZInt(1).ZVal(), nil
			default:
				return nil, ctx.Errorf("unsupported operator %s", op)
			}

		}

	case phpv.ZtObject:
		if b.GetType() == phpv.ZtObject {
			cmp, err := phpv.CompareObject(ctx, a.AsObject(ctx), b.AsObject(ctx))
			if err != nil {
				if errors.Is(err, phpv.ErrComparisonDepth) {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, err.Error())
				}
				return nil, err
			}
			// Handle uncomparable objects (e.g., different enum cases)
			if cmp == phpv.CompareUncomparable {
				switch op {
				case tokenizer.T_IS_EQUAL:
					res = false
				case tokenizer.T_IS_NOT_EQUAL:
					res = true
				default:
					res = false // all ordered comparisons return false for uncomparable
				}
			} else {
				switch op {
				case tokenizer.T_IS_EQUAL:
					res = cmp == 0
				case tokenizer.T_IS_NOT_EQUAL:
					res = cmp != 0
				case tokenizer.Rune('<'):
					res = cmp < 0
				case tokenizer.Rune('>'):
					res = cmp > 0
				case tokenizer.T_IS_SMALLER_OR_EQUAL:
					res = cmp <= 0
				case tokenizer.T_IS_GREATER_OR_EQUAL:
					res = cmp >= 0
				default:
					return nil, ctx.Errorf("unsupported operator %s", op)
				}
			}
		}
	default:
		return nil, ctx.Errorf("todo operator type unsupported %s", a.GetType())
	}

	return phpv.ZBool(res).ZVal(), nil
}

// operatorCompareBool performs a bool comparison between two values.
// In PHP 8, when either operand is bool or null, comparison is always done
// by casting both to bool.
func operatorCompareBool(ctx phpv.Context, op tokenizer.ItemType, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	a, _ = a.As(ctx, phpv.ZtBool)
	b, _ = b.As(ctx, phpv.ZtBool)
	var ab, bb int
	if a.Value().(phpv.ZBool) {
		ab = 1
	}
	if b.Value().(phpv.ZBool) {
		bb = 1
	}

	switch op {
	case tokenizer.Rune('<'):
		return phpv.ZBool(ab < bb).ZVal(), nil
	case tokenizer.Rune('>'):
		return phpv.ZBool(ab > bb).ZVal(), nil
	case tokenizer.T_IS_SMALLER_OR_EQUAL:
		return phpv.ZBool(ab <= bb).ZVal(), nil
	case tokenizer.T_IS_GREATER_OR_EQUAL:
		return phpv.ZBool(ab >= bb).ZVal(), nil
	case tokenizer.T_IS_EQUAL:
		return phpv.ZBool(ab == bb).ZVal(), nil
	case tokenizer.T_IS_NOT_EQUAL:
		return phpv.ZBool(ab != bb).ZVal(), nil
	case tokenizer.T_SPACESHIP:
		if ab < bb {
			return phpv.ZInt(-1).ZVal(), nil
		} else if ab > bb {
			return phpv.ZInt(1).ZVal(), nil
		}
		return phpv.ZInt(0).ZVal(), nil
	default:
		return nil, ctx.Errorf("unsupported operator %s", op)
	}
}

// operatorCompareResult converts a comparison result (cmp) to a ZVal based on the operator.
func operatorCompareResult(op tokenizer.ItemType, cmp int) (*phpv.ZVal, error) {
	switch op {
	case tokenizer.Rune('<'):
		return phpv.ZBool(cmp < 0).ZVal(), nil
	case tokenizer.Rune('>'):
		return phpv.ZBool(cmp > 0).ZVal(), nil
	case tokenizer.T_IS_SMALLER_OR_EQUAL:
		return phpv.ZBool(cmp <= 0).ZVal(), nil
	case tokenizer.T_IS_GREATER_OR_EQUAL:
		return phpv.ZBool(cmp >= 0).ZVal(), nil
	case tokenizer.T_IS_EQUAL:
		return phpv.ZBool(cmp == 0).ZVal(), nil
	case tokenizer.T_IS_NOT_EQUAL:
		return phpv.ZBool(cmp != 0).ZVal(), nil
	case tokenizer.T_SPACESHIP:
		return phpv.ZInt(cmp).ZVal(), nil
	default:
		return phpv.ZBool(false).ZVal(), nil
	}
}

// phpTypeName returns the PHP type name for error messages (e.g., "string", "int", "float")
func phpTypeName(v *phpv.ZVal) string {
	switch v.GetType() {
	case phpv.ZtString:
		return "string"
	case phpv.ZtInt:
		return "int"
	case phpv.ZtFloat:
		return "float"
	case phpv.ZtBool:
		return "bool"
	case phpv.ZtNull:
		return "null"
	case phpv.ZtArray:
		return "array"
	case phpv.ZtObject:
		if obj := v.AsObject(nil); obj != nil {
			return string(obj.GetClass().GetName())
		}
		return "object"
	default:
		return v.GetType().String()
	}
}

// implicitToInt converts a ZVal to int, emitting a "Deprecated: Implicit conversion
// from float X to int loses precision" warning when a float with fractional part is
// implicitly converted to int (PHP 8.1+).
func implicitToInt(ctx phpv.Context, z *phpv.ZVal) (*phpv.ZVal, error) {
	if z.GetType() == phpv.ZtFloat {
		v, err := phpv.FloatToIntImplicit(ctx, z.Value().(phpv.ZFloat))
		return v.ZVal(), err
	}
	r, err := z.As(ctx, phpv.ZtInt)
	return r, err
}

// isLeadingNumeric is defined in compile-array.go

// isFatalPhpError returns true if the error is a PHP fatal error (E_ERROR
// or E_COMPILE_ERROR) that should not be silently swallowed by operators
// like ?? that normally suppress errors on their LHS.
func isFatalPhpError(err error) bool {
	var phpErr *phpv.PhpError
	if errors.As(err, &phpErr) {
		return phpErr.Code == phpv.E_ERROR || phpErr.Code == phpv.E_COMPILE_ERROR
	}
	return false
}
