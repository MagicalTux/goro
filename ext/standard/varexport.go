package standard

import (
	"bytes"
	"fmt"
	"io"
	"unsafe"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func mixed var_export ( mixed $expression  [, $return = FALSE ] )
func stdFuncVarExport(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var expr *phpv.ZVal
	var returnOutput *phpv.ZBool
	_, err := core.Expand(ctx, args, &expr, &returnOutput)
	if err != nil {
		return nil, ctx.Error(err)
	}

	if returnOutput != nil && *returnOutput {
		var buf bytes.Buffer
		doVarExport(ctx, &buf, expr, "", nil)
		return phpv.ZStr(buf.String()), nil
	} else {
		doVarExport(ctx, ctx, expr, "", nil)
		return phpv.ZNULL.ZVal(), nil
	}
}

func doVarExport(ctx phpv.Context, w io.Writer, z *phpv.ZVal, linePfx string, recurs map[uintptr]bool) error {
	if recurs == nil {
		recurs = make(map[uintptr]bool)
	} else {
		// duplicate
		n := make(map[uintptr]bool)
		for k, v := range recurs {
			n[k] = v
		}
		recurs = n
	}

	v := uintptr(unsafe.Pointer(z))
	if _, n := recurs[v]; n {
		fmt.Fprintf(w, "%s*RECURSION*\n", linePfx)
		return nil
	} else {
		recurs[v] = true
	}

	// TODO: improve formatting

	switch z.GetType() {
	case phpv.ZtNull:
		fmt.Fprintf(w, "%sNULL", linePfx)
	case phpv.ZtBool:
		if z.Value().(phpv.ZBool) {
			fmt.Fprintf(w, "%strue", linePfx)
		} else {
			fmt.Fprintf(w, "%sfalse", linePfx)
		}
	case phpv.ZtInt:
		fmt.Fprintf(w, "%s%d", linePfx, z.Value())
	case phpv.ZtFloat:
		z2, _ := z.As(ctx, phpv.ZtString)
		fmt.Fprintf(w, "%s%s", linePfx, z2)
	case phpv.ZtString:
		s := z.Value().(phpv.ZString)
		fmt.Fprintf(w, "%s\"%s\"", linePfx, s)
	case phpv.ZtArray:
		fmt.Fprintf(w, "%sarray(\n", linePfx)
		localPfx := linePfx + " "
		it := z.NewIterator()
		for {
			if !it.Valid(ctx) {
				break
			}
			k, err := it.Key(ctx)
			if err != nil {
				return err
			}
			if k.GetType() == phpv.ZtInt {
				fmt.Fprintf(w, "%s%s =>", localPfx, k)
			} else {
				fmt.Fprintf(w, "%s\"%s\" =>", localPfx, k)
			}
			v, err := it.Current(ctx)
			if err != nil {
				return err
			}
			doVarExport(ctx, w, v, localPfx, recurs)
			fmt.Fprintf(w, ",\n")
			it.Next(ctx)
		}
		fmt.Fprintf(w, "%s)", linePfx)
	case phpv.ZtObject:
		v := z.Value()
		if obj, ok := v.(*phpobj.ZObject); ok {
			// TODO: fix static methods, error call to undefined function
			fmt.Fprintf(w, "%s%s::__set_state(array(\n", linePfx, obj.Class.GetName())
		} else {
			fmt.Fprintf(w, "%sarray(\n", linePfx)
		}
		localPfx := linePfx + " "
		it := z.NewIterator()
		if it != nil {
			for {
				if !it.Valid(ctx) {
					break
				}
				k, err := it.Key(ctx)
				if err != nil {
					return err
				}
				if k.GetType() == phpv.ZtInt {
					fmt.Fprintf(w, "%s%s=>\n", localPfx, k)
				} else {
					fmt.Fprintf(w, "%s\"%s\"=>\n", localPfx, k)
				}
				v, err := it.Current(ctx)
				if err != nil {
					return err
				}
				doVarExport(ctx, w, v, localPfx, recurs)
				it.Next(ctx)
			}
		}

		if _, ok := v.(*phpobj.ZObject); ok {
			fmt.Fprintf(w, "%s))\n", linePfx)
		} else {
			fmt.Fprintf(w, "%s)\n", linePfx)
		}
	default:
		fmt.Fprintf(w, "// Unknown[%T]:%+v\n", z.Value(), z.Value())
	}
	return nil
}
