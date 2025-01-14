package phpv

import "strings"

func CompareObject(ctx Context, ao, bo ZObject) (int, error) {
	if ao.GetClass() != bo.GetClass() {
		return -1, nil
	}

	aIter := ao.NewIterator()
	bIter := bo.NewIterator()
	for aIter.Valid(ctx) && bIter.Valid(ctx) {
		av, _ := aIter.Current(ctx)
		bv, _ := aIter.Current(ctx)

		cmp, err := Compare(ctx, av, bv)
		if err != nil {
			return -1, err
		}
		if cmp != 0 {
			return -1, nil
		}

		aIter.Next(ctx)
		bIter.Next(ctx)
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
		bv, _ := aa.OffsetGet(ctx, k)

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
		if b.GetType() != ZtArray {
			return 1, nil
		}
		return CompareArray(ctx, a.AsArray(ctx), b.AsArray(ctx))
	}

	if b.GetType() == ZtArray {
		if a.GetType() != ZtArray {
			return -1, nil
		}
		return CompareArray(ctx, b.AsArray(ctx), a.AsArray(ctx))
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

	// if both are strings but only one is numeric, then do string comparison
	// this handle cases that compare values like "a" and "9999"
	aIsNonNumericString := a.GetType() == ZtString && ia == nil
	bIsNonNumericString := b.GetType() == ZtString && ib == nil
	if (aIsNonNumericString && ib != nil && b.GetType() != ZtInt) ||
		(bIsNonNumericString && ia != nil && a.GetType() != ZtInt) {
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

	if a.GetType() == ZtBool || b.GetType() == ZtBool {
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

CompareStrings:
	switch a.Value().GetType() {
	case ZtString:
		av := string(a.AsString(ctx))
		bv := string(b.AsString(ctx))
		return strings.Compare(av, bv), nil
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

	return 0, nil
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

	cmp, err := Compare(ctx, a, b)
	if err != nil {
		return false, err
	}
	return cmp == 0, nil
}
