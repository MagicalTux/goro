package phpv

import "strings"

// compareDepth tracks recursion depth to prevent stack overflow on circular references.
var compareDepth int

// CompareUncomparable is a sentinel value returned by CompareObject
// when two objects cannot be ordered (e.g., different enum cases).
// Callers should treat this as "not equal" for == and "incomparable"
// (all ordered comparisons return false) for <, >, <=, >=.
const CompareUncomparable = 2

func CompareObject(ctx Context, ao, bo ZObject) (int, error) {
	// Same instance - always equal
	if ao == bo {
		return 0, nil
	}

	if ao.GetClass() != bo.GetClass() {
		return 1, nil
	}

	// Enum cases: different cases of the same enum are not orderable
	if ao.GetClass().GetType().Has(ZClassTypeEnum) {
		return CompareUncomparable, nil
	}

	compareDepth++
	if compareDepth > 256 {
		compareDepth--
		return 0, nil // treat deeply nested comparisons as equal to avoid stack overflow
	}
	defer func() { compareDepth-- }()

	aIter := ao.NewIterator()
	bIter := bo.NewIterator()
	for aIter.Valid(ctx) && bIter.Valid(ctx) {
		av, _ := aIter.Current(ctx)
		bv, _ := bIter.Current(ctx)

		cmp, err := Compare(ctx, av, bv)
		if err != nil {
			return -1, err
		}
		if cmp != 0 {
			return cmp, nil
		}

		aIter.Next(ctx)
		bIter.Next(ctx)
	}

	// Check if one has more properties than the other
	if aIter.Valid(ctx) {
		return 1, nil
	}
	if bIter.Valid(ctx) {
		return -1, nil
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

func Compare(ctx Context, a, b *ZVal) (int, error) {
	// operator compare (< > <= >= == === != !== <=>) involve a lot of dark magic in php, unless both values are of the same type (and even so)
	// loose comparison will convert number-y looking strings into numbers, etc
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
		return 1, nil // array > all other scalars
	}

	if b.GetType() == ZtArray {
		if a.GetType() == ZtArray {
			return CompareArray(ctx, a.AsArray(ctx), b.AsArray(ctx))
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

	// PHP 8: when both operands are strings, only do numeric comparison
	// if BOTH are numeric strings. If either is non-numeric, use string comparison.
	if a.GetType() == ZtString && b.GetType() == ZtString {
		if ia == nil || ib == nil {
			// At least one string is non-numeric, use string comparison
			goto CompareStrings
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
		if b.GetType() != ZtObject {
			return 1, nil
		}
		return CompareObject(ctx, a.AsObject(ctx), b.AsObject(ctx))
	}
	if b.GetType() == ZtObject {
		if a.GetType() != ZtObject {
			return -1, nil
		}
		return CompareObject(ctx, b.AsObject(ctx), a.AsObject(ctx))
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
