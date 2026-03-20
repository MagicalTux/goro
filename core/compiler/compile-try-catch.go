package compiler

import (
	"bytes"
	"fmt"
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
	var pendingErr error // error to propagate after finally (PhpThrow or other)
	_, err := rt.try.Run(ctx)

	if err != nil {
		throwErr, ok := err.(*phperr.PhpThrow)
		if !ok {
			// Check if this is an exit/die - do NOT run finally blocks
			if _, isExit := err.(*phpv.PhpExit); isExit {
				return nil, err
			}
			// For return statements, snapshot the return value before running finally
			// so that modifications in finally don't affect the returned value.
			// Skip snapshotting for return-by-reference functions.
			returnsByRef := false
			if fc := ctx.Func(); fc != nil {
				if cc, ok := fc.(interface{ Callable() phpv.Callable }); ok {
					if c := cc.Callable(); c != nil {
						if rr, ok := c.(interface{ ReturnsByRef() bool }); ok && rr.ReturnsByRef() {
							returnsByRef = true
						}
					}
				}
			}
			if !returnsByRef {
				if ret, isReturn := err.(*phperr.PhpReturn); isReturn && rt.finally != nil {
					if ret.V != nil {
						ret.V = ret.V.Dup()
					}
				}
				if retWrap, isError := err.(*phpv.PhpError); isError {
					if ret, isReturn := retWrap.Err.(*phperr.PhpReturn); isReturn && rt.finally != nil {
						if ret.V != nil {
							ret.V = ret.V.Dup()
						}
					}
				}
			}
			// Non-PHP-throw error (e.g. return, break, continue):
			// still need to run finally
			if rt.finally != nil {
				_, ferr := rt.finally.Run(ctx)
				if ferr != nil {
					return nil, ferr
				}
			}
			return nil, err
		}

		caught := false
		for _, c := range rt.catches {
			var match bool
			for _, className := range c.typeNames {
				class, err := ctx.Global().GetClass(ctx, className, false)
				if err != nil {
					// Class not found - skip this catch type
					continue
				}
				subClass := throwErr.Obj.GetClass().InstanceOf(class)
				implements := throwErr.Obj.GetClass().Implements(class)
				if class != nil && (subClass || implements) {
					match = true
					break
				}
			}

			if match {
				if c.varname != "" {
					ctx.OffsetSet(ctx, c.varname, throwErr.Obj.ZVal())
				}
				_, err = c.body.Run(ctx)
				if err != nil {
					// Catch block threw/returned - still need to run finally
					// Snapshot return value so finally cannot modify it.
					if ret, isReturn := err.(*phperr.PhpReturn); isReturn && rt.finally != nil {
						if ret.V != nil {
							ret.V = ret.V.Dup()
						}
					}
					pendingErr = err
				}
				caught = true
				break
			}
		}

		if !caught {
			// Uncaught exception - still need to run finally
			pendingErr = throwErr
		}
	}

	if rt.finally != nil {
		_, ferr := rt.finally.Run(ctx)
		if ferr != nil {
			// Finally block threw an error.
			// If there was a pending PHP exception, chain it as previous
			// of the finally exception (unless it would create a cycle).
			if pendingThrow, ok := pendingErr.(*phperr.PhpThrow); ok {
				if finallyThrow, ok2 := ferr.(*phperr.PhpThrow); ok2 {
					chainExceptionPrevious(ctx, finallyThrow.Obj, pendingThrow.Obj)
				}
			}
			return nil, ferr
		}
	}

	if pendingErr != nil {
		return nil, pendingErr
	}

	return nil, nil
}

// chainExceptionPrevious sets pendingObj as the $previous of finallyObj,
// but only if it wouldn't create a cycle in the exception chain.
func chainExceptionPrevious(ctx phpv.Context, finallyObj phpv.ZObject, pendingObj phpv.ZObject) {
	// Check if pendingObj is already in the finallyObj's previous chain (would create cycle)
	seen := make(map[phpv.ZObject]bool)
	seen[finallyObj] = true
	cur := finallyObj
	for {
		prev := cur.HashTable().GetString("previous")
		if prev == nil || prev.GetType() == phpv.ZtNull {
			break
		}
		prevObj, ok := prev.Value().(phpv.ZObject)
		if !ok {
			break
		}
		if seen[prevObj] {
			return // cycle detected, don't chain
		}
		seen[prevObj] = true
		cur = prevObj
	}

	// Check if finallyObj is in pendingObj's previous chain (would create cycle)
	cur = pendingObj
	for cur != nil {
		if seen[cur] {
			return // would create cycle
		}
		seen[cur] = true
		prev := cur.HashTable().GetString("previous")
		if prev == nil || prev.GetType() == phpv.ZtNull {
			break
		}
		prevObj, ok := prev.Value().(phpv.ZObject)
		if !ok {
			break
		}
		cur = prevObj
	}

	// Find the end of the finallyObj's previous chain and append pendingObj
	cur = finallyObj
	for {
		prev := cur.HashTable().GetString("previous")
		if prev == nil || prev.GetType() == phpv.ZtNull {
			cur.HashTable().SetString("previous", pendingObj.ZVal())
			return
		}
		prevObj, ok := prev.Value().(phpv.ZObject)
		if !ok {
			cur.HashTable().SetString("previous", pendingObj.ZVal())
			return
		}
		cur = prevObj
	}
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
		// Handle leading \ for fully-qualified names like \TypeError
		fullyQualified := false
		if i.Type == tokenizer.T_NS_SEPARATOR {
			fullyQualified = true
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}
		// Check for reserved names that cannot be used in catch
		if i.Type == tokenizer.T_STATIC || (i.Type == tokenizer.T_STRING &&
			(phpv.ZString(i.Data).ToLower() == "static")) {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Bad class name in the catch statement"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}
		if i.Type != tokenizer.T_STRING {
			return nil, i.Unexpected()
		}
		// Build full name (consume namespace parts)
		name := i.Data
		for {
			next, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if next.Type == tokenizer.T_NS_SEPARATOR {
				part, err := c.NextItem()
				if err != nil {
					return nil, err
				}
				if part.Type != tokenizer.T_STRING {
					return nil, part.Unexpected()
				}
				name += "\\" + part.Data
			} else {
				c.backup()
				break
			}
		}
		// Resolve through namespace
		var resolved phpv.ZString
		if fullyQualified {
			resolved = c.resolveClassName("\\" + phpv.ZString(name))
		} else {
			resolved = c.resolveClassName(phpv.ZString(name))
		}
		res.typeNames = append(res.typeNames, resolved)

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
			// PHP 8: catch without variable — e.g., catch (Throwable) { ... }
			res.body, err = compileBaseSingle(nil, c)
			if err != nil {
				return nil, err
			}
			return res, nil
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
	l := i.Loc()
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

	// Check that try has at least one catch or a finally block
	if len(res.catches) == 0 && res.finally == nil {
		return nil, &phpv.PhpError{
			Err:  fmt.Errorf("Cannot use try without catch or finally"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  l,
		}
	}

	return res, nil
}
