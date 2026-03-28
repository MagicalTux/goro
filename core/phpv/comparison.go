package phpv

import (
	"errors"
	"math"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
)

// ErrComparisonDepth is returned when object comparison exceeds the maximum
// recursion depth. Callers should convert this to a PHP Error throwable.
var ErrComparisonDepth = errors.New("Nesting level too deep - recursive dependency?")

// compareDepth tracks recursion depth to prevent stack overflow on circular references.
var compareDepth int

// CompareUncomparable is a sentinel value returned by CompareObject
// when two objects cannot be ordered (e.g., different enum cases).
// Callers should treat this as "not equal" for == and "incomparable"
// (all ordered comparisons return false) for <, >, <=, >=.
const CompareUncomparable = 2

// findCompareHandler walks up the class hierarchy to find a HandleCompare handler.
func findCompareHandler(c ZClass) func(Context, ZObject, ZObject) (int, error) {
	for c != nil {
		if h := c.Handlers(); h != nil && h.HandleCompare != nil {
			return h.HandleCompare
		}
		c = c.GetParent()
	}
	return nil
}

func CompareObject(ctx Context, ao, bo ZObject) (int, error) {
	// Same instance - always equal
	if ao == bo {
		return 0, nil
	}

	// Check for custom comparison handlers first - walk up the class hierarchy.
	// This allows subclasses to inherit compare handlers from parent classes
	// (e.g., MyDateTimeZone extends DateTimeZone).
	if handler := findCompareHandler(ao.GetClass()); handler != nil {
		return handler(ctx, ao, bo)
	}
	if handler := findCompareHandler(bo.GetClass()); handler != nil {
		return handler(ctx, ao, bo)
	}

	if ao.GetClass() != bo.GetClass() {
		return CompareUncomparable, nil
	}

	// Enum cases: different cases of the same enum are not orderable
	if ao.GetClass().GetType().Has(ZClassTypeEnum) {
		return CompareUncomparable, nil
	}

	compareDepth++
	if compareDepth > 256 {
		compareDepth--
		return 0, ErrComparisonDepth
	}
	defer func() { compareDepth-- }()

	// PHP's == operator compares ALL properties (public, protected, private),
	// not just public ones. Use the internal hash table which stores all
	// properties with their storage keys. Since both objects are of the same
	// class, they have the same key scheme.
	aHT := ao.HashTable()
	bHT := bo.HashTable()

	if aHT.Count() != bHT.Count() {
		if aHT.Count() > bHT.Count() {
			return 1, nil
		}
		return -1, nil
	}

	// Compare all entries
	aIter := aHT.NewIterator()
	for aIter.Valid(ctx) {
		aKey, _ := aIter.Key(ctx)
		aVal, _ := aIter.Current(ctx)

		bVal, ok := bHT.GetStringB(aKey.AsString(ctx))
		if !ok {
			return 1, nil // a has a key b doesn't
		}

		cmp, err := Compare(ctx, aVal, bVal)
		if err != nil {
			return -1, err
		}
		if cmp != 0 {
			return cmp, nil
		}

		aIter.Next(ctx)
	}

	return 0, nil
}

func CompareArray(ctx Context, aa, ba *ZArray) (int, error) {
	ac := aa.Count(ctx)
	bc := ba.Count(ctx)
	if ac != bc {
		if ac < bc {
			return -1, nil
		}
		return 1, nil
	}

	it := aa.NewIterator()
	for it.Valid(ctx) {
		k, err := it.Key(ctx)
		if err != nil {
			return -1, err
		}
		hasKey, err := ba.OffsetExists(ctx, k)
		if !hasKey {
			return -1, err
		}

		av, _ := aa.OffsetGet(ctx, k)
		bv, _ := ba.OffsetGet(ctx, k)

		cmp, err := Compare(ctx, av, bv)
		if err != nil {
			return -1, err
		}
		if cmp != 0 {
			return cmp, nil
		}

		it.Next(ctx)
	}

	return 0, nil
}

// compareIntStrings compares two integer-like strings with arbitrary precision.
// Both strings must consist of optional leading whitespace, optional '-', then digits.
// Returns -1, 0, or 1.
func CompareIntStrings(a, b string) int {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	aNeg := len(a) > 0 && a[0] == '-'
	bNeg := len(b) > 0 && b[0] == '-'
	if aNeg {
		a = a[1:]
	}
	if bNeg {
		b = b[1:]
	}
	// Strip leading zeros
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if a == "" {
		a = "0"
		aNeg = false
	}
	if b == "" {
		b = "0"
		bNeg = false
	}
	if aNeg != bNeg {
		if aNeg {
			return -1
		}
		return 1
	}
	// Both same sign. Compare magnitudes.
	var cmp int
	if len(a) != len(b) {
		if len(a) < len(b) {
			cmp = -1
		} else {
			cmp = 1
		}
	} else {
		cmp = strings.Compare(a, b)
	}
	if aNeg {
		cmp = -cmp // negate for negative numbers
	}
	return cmp
}

// compareObjectToNumeric converts an object to a numeric type for comparison,
// emitting E_NOTICE (not E_WARNING) as PHP does for implicit comparison conversions.
// targetType should be ZtInt or ZtFloat depending on the other operand.
func CompareObjectToNumeric(ctx Context, z *ZVal, targetType ZType) *ZVal {
	obj := z.AsObject(ctx)
	if targetType == ZtFloat {
		ctx.Notice("Object of class %s could not be converted to float", obj.GetClass().GetName(), logopt.NoFuncName(true))
		return NewZVal(ZFloat(1))
	}
	ctx.Notice("Object of class %s could not be converted to int", obj.GetClass().GetName(), logopt.NoFuncName(true))
	return NewZVal(ZInt(1))
}

func Compare(ctx Context, a, b *ZVal) (int, error) {
	// operator compare (< > <= >= == === != !== <=>) involve a lot of dark magic in php, unless both values are of the same type (and even so)
	// loose comparison will convert number-y looking strings into numbers, etc

	// NaN is never equal to anything, including itself (IEEE 754).
	// PHP returns 1 for NaN comparisons, making == false and all ordered comparisons false.
	if a.GetType() == ZtFloat && math.IsNaN(float64(a.Value().(ZFloat))) {
		return 1, nil
	}
	if b.GetType() == ZtFloat && math.IsNaN(float64(b.Value().(ZFloat))) {
		return 1, nil
	}

	if a.GetType() == ZtArray {
		if b.GetType() == ZtArray {
			return CompareArray(ctx, a.AsArray(ctx), b.AsArray(ctx))
		}
		// PHP: array vs bool/null → convert to bool comparison
		if b.GetType() == ZtBool || b.GetType() == ZtNull {
			ab := a.AsBool(ctx)
			bb := b.AsBool(ctx)
			if ab == bb {
				return 0, nil
			}
			if ab {
				return 1, nil
			}
			return -1, nil
		}
		// PHP 8: objects are always greater than arrays
		if b.GetType() == ZtObject {
			return -1, nil // array < object
		}
		return 1, nil // array > all other scalars
	}

	if b.GetType() == ZtArray {
		if a.GetType() == ZtArray {
			return CompareArray(ctx, a.AsArray(ctx), b.AsArray(ctx))
		}
		// PHP 8: objects are always greater than arrays
		if a.GetType() == ZtObject {
			return 1, nil // object > array
		}
		// PHP: bool/null vs array → convert to bool comparison
		if a.GetType() == ZtBool || a.GetType() == ZtNull {
			ab := a.AsBool(ctx)
			bb := b.AsBool(ctx)
			if ab == bb {
				return 0, nil
			}
			if ab {
				return 1, nil
			}
			return -1, nil
		}
		return -1, nil // all other scalars < array
	}

	var ia, ib *ZVal
	var aLooksInt, bLooksInt bool

	switch a.GetType() {
	case ZtInt, ZtFloat:
		ia = a
	case ZtString:
		aStr := a.Value().(ZString)
		aLooksInt = aStr.LooksInt()
		if aLooksInt || aStr.IsNumeric() {
			v, _ := aStr.AsNumeric()
			if v != nil {
				ia = NewZVal(v)
			}
		}
	}

	switch b.GetType() {
	case ZtInt, ZtFloat:
		ib = b
	case ZtString:
		bStr := b.Value().(ZString)
		bLooksInt = bStr.LooksInt()
		if bLooksInt || bStr.IsNumeric() {
			v, _ := bStr.AsNumeric()
			if v != nil {
				ib = NewZVal(v)
			}
		}
	}

	// PHP 8: when both operands are strings, only do numeric comparison
	// if BOTH are numeric strings. If either is non-numeric, use string comparison.
	if a.GetType() == ZtString && b.GetType() == ZtString {
		if ia == nil || ib == nil {
			// At least one string is non-numeric, use string comparison
			goto CompareStrings
		}
		// When both strings look like integers but at least one overflowed to float,
		// use arbitrary-precision integer comparison (float64 loses precision for large ints).
		if aLooksInt && bLooksInt {
			if ia.GetType() == ZtFloat || ib.GetType() == ZtFloat {
				return CompareIntStrings(string(a.Value().(ZString)), string(b.Value().(ZString))), nil
			}
		}
	}

	// PHP 8: when comparing a number with a non-numeric string,
	// convert the number to string and do string comparison
	if (a.GetType() == ZtInt || a.GetType() == ZtFloat) && b.GetType() == ZtString && ib == nil {
		goto CompareStrings
	}
	if (b.GetType() == ZtInt || b.GetType() == ZtFloat) && a.GetType() == ZtString && ia == nil {
		goto CompareStrings
	}

	// PHP 8: string vs object -> no numeric conversion, objects are greater than strings
	if a.GetType() == ZtString && b.GetType() == ZtObject {
		return -1, nil // string < object
	}
	if a.GetType() == ZtObject && b.GetType() == ZtString {
		return 1, nil // object > string
	}

	// PHP 8: when either operand is bool or null, always use bool comparison
	// (even if the other operand is numeric). This must come before numeric comparison.
	if a.GetType() == ZtBool || b.GetType() == ZtBool || a.GetType() == ZtNull || b.GetType() == ZtNull {
		ab := a.AsBool(ctx)
		bb := b.AsBool(ctx)
		if ab == bb {
			return 0, nil
		}
		if ab {
			return 1, nil
		}
		return -1, nil
	}

	if ia != nil || ib != nil {
		// if either part is a numeric, force the other one as numeric too and go through comparison
		if ia == nil {
			if a.GetType() == ZtObject {
				// Object in comparison: emit Notice (not Warning) with appropriate target type
				targetType := ZtInt
				if ib != nil && ib.GetType() == ZtFloat {
					targetType = ZtFloat
				}
				ia = CompareObjectToNumeric(ctx, a, targetType)
			} else {
				ia, _ = a.AsNumeric(ctx)
			}
		}
		if ib == nil {
			if b.GetType() == ZtObject {
				// Object in comparison: emit Notice (not Warning) with appropriate target type
				targetType := ZtInt
				if ia != nil && ia.GetType() == ZtFloat {
					targetType = ZtFloat
				}
				ib = CompareObjectToNumeric(ctx, b, targetType)
			} else {
				ib, _ = b.AsNumeric(ctx)
			}
		}

		// perform numeric comparison
		if ia.GetType() != ib.GetType() {
			// normalize type - at this point as both are numeric, it means either is a float. Make them both float
			ia, _ = ia.As(ctx, ZtFloat)
			ib, _ = ib.As(ctx, ZtFloat)
		}

		var res int
		switch ia.GetType() {
		case ZtFloat:
			ia := ia.Value().(ZFloat)
			ib := ib.Value().(ZFloat)
			if ia < ib {
				res = -1
			} else if ia > ib {
				res = 1
			} else {
				res = 0
			}
		case ZtInt:
			ia := ia.Value().(ZInt)
			ib := ib.Value().(ZInt)
			if ia < ib {
				res = -1
			} else if ia > ib {
				res = 1
			} else {
				res = 0
			}
		}

		return res, nil
	}

	if a.GetType() == ZtNull && b.GetType() == ZtNull {
		return 0, nil
	}

	if a.GetType() == ZtBool || b.GetType() == ZtBool || a.GetType() == ZtNull || b.GetType() == ZtNull {
		a, _ = a.As(ctx, ZtBool)
		b, _ = b.As(ctx, ZtBool)

		var ab, bb, res int
		if val, ok := a.Value().(ZBool); ok && bool(val) {
			ab = 1
		} else {
			ab = 0
		}
		if val, ok := b.Value().(ZBool); ok && bool(val) {
			bb = 1
		} else {
			bb = 0
		}

		if ab < bb {
			res = -1
		} else if ab > bb {
			res = 1
		} else {
			res = 0
		}

		return res, nil
	}

	if a.GetType() == ZtObject {
		if b.GetType() == ZtObject {
			return CompareObject(ctx, a.AsObject(ctx), b.AsObject(ctx))
		}
		// Check if object has HandleCast (e.g., GMP) for numeric comparison with scalars
		ao := a.AsObject(ctx)
		if h := ao.GetClass().Handlers(); h != nil && h.HandleCast != nil {
			// Try to cast to int for comparison
			val, err := h.HandleCast(ctx, ao, ZtInt)
			if err == nil {
				return Compare(ctx, val.ZVal(), b)
			}
		}
		return 1, nil
	}
	if b.GetType() == ZtObject {
		if a.GetType() == ZtObject {
			return CompareObject(ctx, a.AsObject(ctx), b.AsObject(ctx))
		}
		// Check if object has HandleCast (e.g., GMP) for numeric comparison with scalars
		bo := b.AsObject(ctx)
		if h := bo.GetClass().Handlers(); h != nil && h.HandleCast != nil {
			// Try to cast to int for comparison
			val, err := h.HandleCast(ctx, bo, ZtInt)
			if err == nil {
				return Compare(ctx, a, val.ZVal())
			}
		}
		return -1, nil
	}

CompareStrings:
	{
		av := string(a.AsString(ctx))
		bv := string(b.AsString(ctx))
		return strings.Compare(av, bv), nil
	}
}

func Equals(ctx Context, a, b *ZVal) (bool, error) {
	cmp, err := Compare(ctx, a, b)
	if err != nil {
		return false, err
	}
	return cmp == 0, nil
}

func StrictEquals(ctx Context, a, b *ZVal) (bool, error) {
	if a.GetType() != b.GetType() {
		return false, nil
	}

	// For arrays, use StrictEquals recursively (not loose Compare)
	if a.GetType() == ZtArray {
		return a.AsArray(ctx).StrictEquals(ctx, b.AsArray(ctx)), nil
	}

	cmp, err := Compare(ctx, a, b)
	if err != nil {
		return false, err
	}
	return cmp == 0, nil
}
