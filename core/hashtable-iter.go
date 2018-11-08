package core

type zhashtableIterator struct {
	t   *ZHashTable
	cur *hashTableVal
}

func (z *zhashtableIterator) Current(ctx Context) *ZVal {
	if !z.Valid(ctx) {
		return nil
	}

	return z.cur.v
}

func (z *zhashtableIterator) Key(ctx Context) *ZVal {
	if !z.Valid(ctx) {
		return nil
	}

	return &ZVal{z.cur.k}
}

func (z *zhashtableIterator) Next(ctx Context) {
	if !z.Valid(ctx) {
		return
	}

	z.cur = z.cur.next
}

func (z *zhashtableIterator) Rewind(ctx Context) {
	z.cur = z.t.first
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
