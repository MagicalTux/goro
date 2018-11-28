package standard

import (
	"bytes"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

//> func mixed str_replace ( mixed $search , mixed $replace , mixed $subject [, int &$count ] )
func stdStrReplace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var search, replace, subject *phpv.ZVal
	var count *phpv.ZInt

	_, err := core.Expand(ctx, args, &search, &replace, &subject, &count)
	if err != nil {
		return nil, err
	}

	if count == nil {
		// avoid crash
		count = new(phpv.ZInt)
	}

	if subject.GetType() == phpv.ZtArray {
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

			err = v.CastTo(ctx, phpv.ZtString)
			if err != nil {
				return nil, err
			}

			vs := v.Value().(phpv.ZString)

			vs, err = doStrReplace(ctx, vs, search, replace, count)
			if err != nil {
				return nil, err
			}

			v.Set(vs.ZVal())
			it.Next(ctx)
		}

		return res, nil
	}

	subject, err = subject.As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	vs := subject.Value().(phpv.ZString)

	vs, err = doStrReplace(ctx, vs, search, replace, count)
	if err != nil {
		return nil, err
	}

	return vs.ZVal(), nil
}

func doStrReplace(ctx phpv.Context, subject phpv.ZString, search, replace *phpv.ZVal, count *phpv.ZInt) (phpv.ZString, error) {
	if search.GetType() == phpv.ZtArray {
		if replace.GetType() == phpv.ZtArray {
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

				from, err = from.As(ctx, phpv.ZtString)
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

				var to *phpv.ZVal
				if it2.Valid(ctx) {
					to, err = it2.Current(ctx)
					if err != nil {
						return subject, err
					}
				}

				to, err = to.As(ctx, phpv.ZtString)
				if err != nil {
					return subject, err
				}

				to_b := []byte(to.AsString(ctx))
				subject = phpv.ZString(bytes.Replace([]byte(subject), from_b, to_b, cnt))
				*count += phpv.ZInt(cnt)

				it1.Next(ctx)
				it2.Next(ctx)
			}
		}

		var err error
		replace, err = replace.As(ctx, phpv.ZtString)
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

			from, err = from.As(ctx, phpv.ZtString)
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

			subject = phpv.ZString(bytes.Replace([]byte(subject), from_b, to_b, cnt))
			*count += phpv.ZInt(cnt)

			it1.Next(ctx)
		}
	}

	search, err := search.As(ctx, phpv.ZtString)
	if err != nil {
		return subject, err
	}

	from_b := []byte(search.AsString(ctx))

	cnt := bytes.Count([]byte(subject), from_b)
	if cnt == 0 {
		return subject, nil
	}

	replace, err = replace.As(ctx, phpv.ZtString)
	if err != nil {
		return subject, err
	}

	to_b := []byte(replace.AsString(ctx))
	subject = phpv.ZString(bytes.Replace([]byte(subject), from_b, to_b, cnt))
	*count += phpv.ZInt(cnt)

	return subject, err
}

//> func string str_rot13 ( string $str )
func fncStrRot13(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	obuf := make([]byte, len(s))
	for i, v := range []byte(s) {
		if v >= 'a' && v <= 'z' {
			v = 'a' + ((v - 'a' + 13) % 26)
		} else if v >= 'A' && v <= 'Z' {
			v = 'A' + ((v - 'A' + 13) % 26)
		}
		obuf[i] = v
	}

	return phpv.ZString(obuf).ZVal(), nil
}

//> func string strtolower ( string $string )
func fncStrToLower(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	return s.ToLower().ZVal(), nil
}
