package standard

import (
	"fmt"
	"unsafe"

	"github.com/MagicalTux/gophp/core"
)

//> func void var_dump ( mixed $expression [, mixed $... ] )
func stdFuncVarDump(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	for _, z := range args {
		err := doVarDump(ctx, z, "", nil)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func doVarDump(ctx core.Context, z *core.ZVal, linePfx string, recurs map[uintptr]bool) error {
	var isRef string
	if z.IsRef() {
		isRef = "&"
	}

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
		fmt.Fprintf(ctx, "%s*RECURSION*\n", linePfx)
		return nil
	} else {
		recurs[v] = true
	}

	switch z.GetType() {
	case core.ZtNull:
		fmt.Fprintf(ctx, "%s%sNULL\n", linePfx, isRef)
	case core.ZtBool:
		if z.Value().(core.ZBool) {
			fmt.Fprintf(ctx, "%s%sbool(true)\n", linePfx, isRef)
		} else {
			fmt.Fprintf(ctx, "%s%sbool(false)\n", linePfx, isRef)
		}
	case core.ZtInt:
		fmt.Fprintf(ctx, "%s%sint(%d)\n", linePfx, isRef, z.Value())
	case core.ZtFloat:
		z2, _ := z.As(ctx, core.ZtString)
		fmt.Fprintf(ctx, "%s%sfloat(%s)\n", linePfx, isRef, z2)
	case core.ZtString:
		s := z.Value().(core.ZString)
		fmt.Fprintf(ctx, "%s%sstring(%d) \"%s\"\n", linePfx, isRef, len(s), s)
	case core.ZtArray:
		c := z.Value().(core.ZCountable).Count(ctx)
		fmt.Fprintf(ctx, "%s%sarray(%d) {\n", linePfx, isRef, c)
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
			if k.GetType() == core.ZtInt {
				fmt.Fprintf(ctx, "%s[%s]=>\n", localPfx, k)
			} else {
				fmt.Fprintf(ctx, "%s[\"%s\"]=>\n", localPfx, k)
			}
			v, err := it.Current(ctx)
			if err != nil {
				return err
			}
			doVarDump(ctx, v, localPfx, recurs)
			it.Next(ctx)
		}
		fmt.Fprintf(ctx, "%s}\n", linePfx)
	case core.ZtObject:
		v := z.Value()
		if obj, ok := v.(*core.ZObject); ok {
			fmt.Fprintf(ctx, "%s%sobject(%s) (%d) {\n", linePfx, isRef, obj.Class.Name, obj.Count(ctx))
		} else if c, ok := v.(core.ZCountable); ok {
			fmt.Fprintf(ctx, "%s%sobject(?) (%d) {\n", linePfx, isRef, c.Count(ctx))
		} else {
			fmt.Fprintf(ctx, "%s%sobject(?) (#) {\n", linePfx, isRef)
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
				if k.GetType() == core.ZtInt {
					fmt.Fprintf(ctx, "%s[%s]=>\n", localPfx, k)
				} else {
					fmt.Fprintf(ctx, "%s[\"%s\"]=>\n", localPfx, k)
				}
				v, err := it.Current(ctx)
				if err != nil {
					return err
				}
				doVarDump(ctx, v, localPfx, recurs)
				it.Next(ctx)
			}
		}
		fmt.Fprintf(ctx, "%s}\n", linePfx)
	default:
		fmt.Fprintf(ctx, "Unknown[%T]:%+v\n", z.Value(), z.Value())
	}
	return nil
}
