package core

import (
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

func CompareObject(ctx phpv.Context, ao, bo phpv.ZObject) (int, error) {
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

func CompareArray(ctx phpv.Context, aa, ba *phpv.ZArray) (int, error) {
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

func Compare(ctx phpv.Context, a, b *phpv.ZVal) (int, error) {
	// operator compare (< > <= >= == === != !== <=>) involve a lot of dark magic in php, unless both values are of the same type (and even so)
	// loose comparison will convert number-y looking strings into numbers, etc
	if a.GetType() == phpv.ZtArray {
		if b.GetType() != phpv.ZtArray {
			return 1, nil
		}
		return CompareArray(ctx, a.AsArray(ctx), b.AsArray(ctx))
	}

	if b.GetType() == phpv.ZtArray {
		if a.GetType() != phpv.ZtArray {
			return -1, nil
		}
		return CompareArray(ctx, b.AsArray(ctx), a.AsArray(ctx))
	}

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

		var res int
		switch ia.GetType() {
		case phpv.ZtFloat:
			ia := ia.Value().(phpv.ZFloat)
			ib := ib.Value().(phpv.ZFloat)
			if ia < ib {
				res = -1
			} else if ia > ib {
				res = 1
			} else {
				res = 0
			}
		case phpv.ZtInt:
			ia := ia.Value().(phpv.ZInt)
			ib := ib.Value().(phpv.ZInt)
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

	if a.GetType() == phpv.ZtNull && b.GetType() == phpv.ZtNull {
		return 0, nil
	}

	if a.GetType() == phpv.ZtBool || b.GetType() == phpv.ZtBool {
		a, _ = a.As(ctx, phpv.ZtBool)
		b, _ = b.As(ctx, phpv.ZtBool)

		var ab, bb, res int
		if val, ok := a.Value().(phpv.ZBool); ok && bool(val) {
			ab = 1
		} else {
			ab = 0
		}
		if val, ok := b.Value().(phpv.ZBool); ok && bool(val) {
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

	switch a.Value().GetType() {
	case phpv.ZtString:
		av := string(a.Value().(phpv.ZString))
		bv := string(b.Value().(phpv.ZString))
		return strings.Compare(av, bv), nil
	}

	if a.GetType() == phpv.ZtObject {
		if b.GetType() != phpv.ZtObject {
			return 1, nil
		}
		return CompareObject(ctx, a.AsObject(ctx), b.AsObject(ctx))
	}
	if b.GetType() == phpv.ZtObject {
		if a.GetType() != phpv.ZtObject {
			return -1, nil
		}
		return CompareObject(ctx, b.AsObject(ctx), a.AsObject(ctx))
	}

	return 0, nil
}

func Equals(ctx phpv.Context, a, b *phpv.ZVal) (bool, error) {
	cmp, err := Compare(ctx, a, b)
	if err != nil {
		return false, err
	}
	return cmp == 0, nil
}

func StrictEquals(ctx phpv.Context, a, b *phpv.ZVal) (bool, error) {
	if a.GetType() != b.GetType() {
		return false, nil
	}

	cmp, err := Compare(ctx, a, b)
	if err != nil {
		return false, err
	}
	return cmp == 0, nil
}
