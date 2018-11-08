package core

import "git.atonline.com/tristantech/gophp/core/tokenizer"

type arrayEntry struct {
	k, v Runnable
}

type runArray []*arrayEntry

func (a runArray) Run(ctx Context) (*ZVal, error) {
	var err error
	array := NewZArray()

	for _, e := range a {
		var k, v *ZVal

		if e.k != nil {
			k, err = e.k.Run(ctx)
			if err != nil {
				return nil, err
			}
		}
		v, err = e.v.Run(ctx)
		if err != nil {
			return nil, err
		}

		array.OffsetSet(k, v)
	}

	return &ZVal{array}, nil
}

func compileArray(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	var res runArray

	array_type := '?'

	if i.IsSingle('[') {
		array_type = ']'
	} else if i.Type == tokenizer.T_ARRAY {
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		if !i.IsSingle('(') {
			return nil, i.Unexpected()
		}
		array_type = ')'
	} else {
		return nil, i.Unexpected()
	}

	for {
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(array_type) {
			break
		}

		var k Runnable
		k, err = compileExpr(i, c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			res = append(res, &arrayEntry{v: k})
			continue
		}

		if i.IsSingle(array_type) {
			res = append(res, &arrayEntry{v: k})
			break
		}

		if i.Type != tokenizer.T_DOUBLE_ARROW {
			return nil, i.Unexpected()
		}

		// ok we got a value now
		var v Runnable
		v, err = compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		res = append(res, &arrayEntry{k: k, v: v})

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			// TODO: append k
			continue
		}

		if i.IsSingle(array_type) {
			// TODO: append k
			break
		}
		return nil, i.Unexpected()
	}

	return res, nil
}
