package compiler

import (
	"bytes"
	"io"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableCatch struct {
	typeNames []phpv.ZString
	varname   phpv.ZString
	body      phpv.Runnable
}

type runnableTry struct {
	try     phpv.Runnable
	catches []*runnableCatch
	finally phpv.Runnable
}

func (rt *runnableTry) Dump(w io.Writer) error {
	var buf bytes.Buffer
	buf.WriteString("try { ... }\n")
	for _, c := range rt.catches {
		buf.WriteString("catch (")
		if len(c.typeNames) > 0 {
			buf.WriteString(string(c.typeNames[0]))
			for _, t := range c.typeNames[1:] {
				buf.WriteByte('|')
				buf.WriteString(string(t))
			}
		}
		buf.WriteByte(' ')
		buf.WriteString(string(c.varname))
		buf.WriteString(") { ... }\n")
	}

	if rt.finally != nil {
		buf.WriteString("finally { ... }\n")
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func (rt *runnableTry) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	var throwErr *phperr.PhpThrow
	_, err := rt.try.Run(ctx)

	if err != nil {
		var ok bool
		throwErr, ok = err.(*phperr.PhpThrow)
		if !ok {
			return nil, err
		}

		for _, c := range rt.catches {
			var match bool
			for _, className := range c.typeNames {
				class, err := ctx.Global().GetClass(ctx, className, false)
				if err != nil {
					return nil, err
				}
				subClass := throwErr.Obj.GetClass().InstanceOf(class)
				implements := throwErr.Obj.GetClass().Implements(class)
				if class != nil && (subClass || implements) {
					match = true
					break
				}
			}

			if match {
				ctx.OffsetSet(ctx, c.varname, throwErr.Obj.ZVal())
				_, err = c.body.Run(ctx)
				if err != nil {
					return nil, err
				}
				throwErr = nil
				break
			}
		}

	}

	if rt.finally != nil {
		_, err = rt.finally.Run(ctx)
		if err != nil {
			return nil, err
		}
	}

	if throwErr != nil {
		return nil, throwErr
	}

	return nil, nil
}

func compileCatch(i *tokenizer.Item, c compileCtx) (*runnableCatch, error) {
	var err error
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle(')') {
		return nil, i.Unexpected()
	}

	res := &runnableCatch{}
	for {
		if i.Type != tokenizer.T_STRING {
			return nil, i.Unexpected()
		}
		res.typeNames = append(res.typeNames, phpv.ZString(i.Data))

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.IsSingle('|') {
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		if i.IsSingle(')') {
			return nil, i.Unexpected()
		}
		if i.Type == tokenizer.T_VARIABLE {
			break
		}
	}

	res.varname = phpv.ZString(i.Data)[1:]

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle(')') {
		return nil, i.Unexpected()
	}

	res.body, err = compileBaseSingle(nil, c)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func compileTry(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	res := &runnableTry{}
	res.try, err = compileBaseSingle(nil, c)
	if err != nil {
		return nil, err
	}

	done := false
	for !done {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		switch i.Type {
		case tokenizer.T_CATCH:
			var catch *runnableCatch
			catch, err = compileCatch(nil, c)
			if err != nil {
				return nil, err
			}
			res.catches = append(res.catches, catch)
		case tokenizer.T_FINALLY:
			res.finally, err = compileBaseSingle(nil, c)
			if err != nil {
				return nil, err
			}
			done = true
		default:
			c.backup()
			done = true
		}
	}

	return res, nil
}
