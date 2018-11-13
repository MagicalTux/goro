package standard

import (
	"bytes"

	"github.com/MagicalTux/gophp/core"
)

//> func mixed str_replace ( mixed $search , mixed $replace , mixed $subject [, int &$count ] )
func stdStrReplace(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var search, replace, subject *core.ZVal
	var count *core.ZInt

	_, err := core.Expand(ctx, args, &search, &replace, &subject, &count)
	if err != nil {
		return nil, err
	}

	if count == nil {
		// avoid crash
		count = new(core.ZInt)
	}

	if subject.GetType() == core.ZtArray {
		res := subject.Dup()
		it := res.NewIterator()

		for {
			if !it.Valid(ctx) {
				break
			}

			v, err := it.Current(ctx)
			if err != nil {
				return nil, err
			}

			err = v.CastTo(ctx, core.ZtString)
			if err != nil {
				return nil, err
			}

			vs := v.Value().(core.ZString)

			vs, err = doStrReplace(ctx, vs, search, replace, count)
			if err != nil {
				return nil, err
			}

			v.Set(vs.ZVal())
			it.Next(ctx)
		}

		return res, nil
	}

	subject, err = subject.As(ctx, core.ZtString)
	if err != nil {
		return nil, err
	}

	vs := subject.Value().(core.ZString)

	vs, err = doStrReplace(ctx, vs, search, replace, count)
	if err != nil {
		return nil, err
	}

	return vs.ZVal(), nil
}

func doStrReplace(ctx core.Context, subject core.ZString, search, replace *core.ZVal, count *core.ZInt) (core.ZString, error) {
	if search.GetType() == core.ZtArray {
		if replace.GetType() == core.ZtArray {
			it1 := search.NewIterator()
			it2 := replace.NewIterator()

			for {
				if !it1.Valid(ctx) {
					// end of source,
					return subject, nil
				}
				from, err := it1.Current(ctx)
				if err != nil {
					return subject, err
				}

				from, err = from.As(ctx, core.ZtString)
				if err != nil {
					return subject, err
				}

				from_b := []byte(from.AsString(ctx))

				cnt := bytes.Count([]byte(subject), from_b)
				if cnt == 0 {
					// nothing to replace, skip
					it1.Next(ctx)
					it2.Next(ctx)
					continue
				}

				var to *core.ZVal
				if it2.Valid(ctx) {
					to, err = it2.Current(ctx)
					if err != nil {
						return subject, err
					}
				}

				to, err = to.As(ctx, core.ZtString)
				if err != nil {
					return subject, err
				}

				to_b := []byte(to.AsString(ctx))
				subject = core.ZString(bytes.Replace([]byte(subject), from_b, to_b, cnt))
				*count += core.ZInt(cnt)

				it1.Next(ctx)
				it2.Next(ctx)
			}

			return subject, nil
		}

		var err error
		replace, err = replace.As(ctx, core.ZtString)
		if err != nil {
			return subject, err
		}

		to_b := []byte(replace.AsString(ctx))

		it1 := search.NewIterator()

		for {
			if !it1.Valid(ctx) {
				// end of source,
				return subject, nil
			}
			from, err := it1.Current(ctx)
			if err != nil {
				return subject, err
			}

			from, err = from.As(ctx, core.ZtString)
			if err != nil {
				return subject, err
			}

			from_b := []byte(from.AsString(ctx))

			cnt := bytes.Count([]byte(subject), from_b)
			if cnt == 0 {
				// nothing to replace, skip
				it1.Next(ctx)
				continue
			}

			subject = core.ZString(bytes.Replace([]byte(subject), from_b, to_b, cnt))
			*count += core.ZInt(cnt)

			it1.Next(ctx)
		}

		return subject, nil
	}

	search, err := search.As(ctx, core.ZtString)
	if err != nil {
		return subject, err
	}

	from_b := []byte(search.AsString(ctx))

	cnt := bytes.Count([]byte(subject), from_b)
	if cnt == 0 {
		return subject, nil
	}

	replace, err = replace.As(ctx, core.ZtString)
	if err != nil {
		return subject, err
	}

	to_b := []byte(replace.AsString(ctx))
	subject = core.ZString(bytes.Replace([]byte(subject), from_b, to_b, cnt))
	*count += core.ZInt(cnt)

	return subject, err
}
