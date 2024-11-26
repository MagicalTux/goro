package standard

import (
	"bytes"
	"fmt"
	"io"
	"strings"
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

	p := uintptr(unsafe.Pointer(z))
	if _, n := recurs[p]; n {
		ctx.Warn("does not handle circular references")
		fmt.Fprintf(w, "NULL")
		return nil
	}

	switch z.GetType() {
	case phpv.ZtNull:
		fmt.Fprintf(w, "NULL")
	case phpv.ZtBool:
		if z.Value().(phpv.ZBool) {
			fmt.Fprintf(w, "true")
		} else {
			fmt.Fprintf(w, "false")
		}
	case phpv.ZtInt:
		fmt.Fprintf(w, "%d", z.Value())
	case phpv.ZtFloat:
		z2, _ := z.As(ctx, phpv.ZtString)
		fmt.Fprintf(w, "%s", z2)
	case phpv.ZtString:
		s := z.Value().(phpv.ZString)
		fmt.Fprintf(w, "'%s'", strings.ReplaceAll(string(s), `'`, `\'`))
	case phpv.ZtArray:
		p := uintptr(unsafe.Pointer(z))
		recurs[p] = true

		fmt.Fprintf(w, "array(\n")
		localPfx := linePfx + "  "
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
				fmt.Fprintf(w, "%s%s => ", localPfx, k)
			} else {
				fmt.Fprintf(w, "%s'%s' => ", localPfx, strings.ReplaceAll(k.String(), `'`, `\'`))
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
		p := uintptr(unsafe.Pointer(z))
		recurs[p] = true

		v := z.Value()
		if obj, ok := v.(*phpobj.ZObject); ok {
			fmt.Fprintf(w, "\n%s%s::__set_state(array(\n", linePfx, obj.Class.GetName())
		} else {
			fmt.Fprintf(w, "%sarray(\n", linePfx)
		}

		localPfx := linePfx + "  "
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
					fmt.Fprintf(w, "%s%s => ", localPfx, k)
				} else {
					fmt.Fprintf(w, "%s'%s' => ", localPfx, strings.ReplaceAll(k.String(), `'`, `\'`))
				}
				v, err := it.Current(ctx)
				if err != nil {
					return err
				}

				doVarExport(ctx, w, v, localPfx, recurs)
				fmt.Fprintf(w, ",\n")
				it.Next(ctx)
			}
		}

		if _, ok := v.(*phpobj.ZObject); ok {
			fmt.Fprintf(w, "%s))", linePfx)
		} else {
			fmt.Fprintf(w, "%s)", linePfx)
		}
	default:
		fmt.Fprintf(w, "// Unknown[%T]:%+v\n", z.Value(), z.Value())
	}
	return nil
}
