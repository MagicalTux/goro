package phpv

type zhashtableIterator struct {
	t   *ZHashTable
	cur *hashTableVal
}

func (z *zhashtableIterator) Current(ctx Context) (*ZVal, error) {
	if !z.Valid(ctx) {
		return nil, nil
	}

	return z.cur.v, nil
}

func (z *zhashtableIterator) Key(ctx Context) (*ZVal, error) {
	if !z.Valid(ctx) {
		return nil, nil
	}

	return &ZVal{z.cur.k}, nil
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
