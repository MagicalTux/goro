package phpv

import "iter"

type zhashtableIterator struct {
	t   *ZHashTable
	cur *hashTableVal
}

func (z *zhashtableIterator) Current(ctx Context) (*ZVal, error) {
	if !z.Valid(ctx) {
		return nil, nil
	}

	value := z.cur.v
	if !value.IsRef() {
		value = value.Dup()
	}

	return value, nil
}

func (z *zhashtableIterator) Key(ctx Context) (*ZVal, error) {
	if !z.Valid(ctx) {
		return nil, nil
	}

	return NewZVal(z.cur.k).Dup(), nil
}

func (z *zhashtableIterator) Next(ctx Context) (*ZVal, error) {
	if !z.Valid(ctx) {
		return nil, nil
	}

	z.cur = z.cur.next
	return z.Current(ctx)
}

func (z *zhashtableIterator) Prev(ctx Context) (*ZVal, error) {
	for {
		if z.cur == nil {
			return nil, nil
		}
		if z.cur.deleted {
			z.cur = z.cur.prev
			continue
		}
		break
	}

	z.cur = z.cur.prev
	if z.cur == nil {
		return nil, nil
	}
	return z.cur.v, nil
}

func (z *zhashtableIterator) Reset(ctx Context) (*ZVal, error) {
	z.cur = z.t.first
	return z.Current(ctx)
}

func (z *zhashtableIterator) ResetIfEnd(ctx Context) (*ZVal, error) {
	if !z.Valid(ctx) {
		z.cur = z.t.first
		return z.Current(ctx)
	}
	return nil, nil
}

func (z *zhashtableIterator) End(ctx Context) (*ZVal, error) {
	z.cur = z.t.last
	return z.Current(ctx)
}

func (z *zhashtableIterator) Valid(ctx Context) bool {
	for {
		if z.cur == nil {
			return false
		}
		if z.cur.deleted {
			z.cur = z.cur.next
			continue
		}
		return true
	}
}

func (a *zhashtableIterator) Iterate(ctx Context) iter.Seq2[*ZVal, *ZVal] {
	return func(yield func(*ZVal, *ZVal) bool) {
		for ; a.Valid(ctx); a.Next(ctx) {
			key, _ := a.Key(ctx)
			value, _ := a.Current(ctx)

			if !value.IsRef() {
				value = value.Dup()
			}

			if !yield(key.Dup(), value) {
				break
			}
		}
	}
}
