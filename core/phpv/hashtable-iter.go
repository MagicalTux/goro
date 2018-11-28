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

func (z *zhashtableIterator) Next(ctx Context) error {
	if !z.Valid(ctx) {
		return nil
	}

	z.cur = z.cur.next
	return nil
}

func (z *zhashtableIterator) Rewind(ctx Context) error {
	z.cur = z.t.first
	return nil
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
