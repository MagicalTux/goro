package standard

import (
	"fmt"

	"github.com/MagicalTux/gophp/core"
)

//> func void var_dump ( mixed $expression [, mixed $... ] )
func stdFuncVarDump(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	for _, z := range args {
		err := doVarDump(ctx, z, "")
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func doVarDump(ctx core.Context, z *core.ZVal, linePfx string) error {
	switch z.GetType() {
	case core.ZtNull:
		fmt.Fprintf(ctx, "%sNULL\n", linePfx)
	case core.ZtBool:
		if z.Value().(core.ZBool) {
			fmt.Fprintf(ctx, "%sbool(true)\n", linePfx)
		} else {
			fmt.Fprintf(ctx, "%sbool(false)\n", linePfx)
		}
	case core.ZtInt:
		fmt.Fprintf(ctx, "%sint(%d)\n", linePfx, z.Value())
	case core.ZtFloat:
		z2, _ := z.As(ctx, core.ZtString)
		fmt.Fprintf(ctx, "%sfloat(%s)\n", linePfx, z2)
	case core.ZtString:
		s := z.Value().(core.ZString)
		fmt.Fprintf(ctx, "%sstring(%d) \"%s\"\n", linePfx, len(s), s)
	case core.ZtArray:
		c := z.Value().(core.ZCountable).Count(ctx)
		fmt.Fprintf(ctx, "%sarray(%d) {\n", linePfx, c)
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
			doVarDump(ctx, v, localPfx)
			it.Next(ctx)
		}
		fmt.Fprintf(ctx, "%s}\n", linePfx)
	case core.ZtObject:
		v := z.Value()
		if obj, ok := v.(*core.ZObject); ok {
			fmt.Fprintf(ctx, "%sobject(%s) (%d) {\n", linePfx, obj.Class.Name, obj.Count(ctx))
		} else if c, ok := v.(core.ZCountable); ok {
			fmt.Fprintf(ctx, "%sobject(?) (%d) {\n", linePfx, c.Count(ctx))
		} else {
			fmt.Fprintf(ctx, "%sobject(?) (#) {\n", linePfx)
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
				doVarDump(ctx, v, localPfx)
				it.Next(ctx)
			}
		}
		fmt.Fprintf(ctx, "%s}\n", linePfx)
	default:
		fmt.Fprintf(ctx, "Unknown[%T]:%+v\n", z.Value(), z.Value())
	}
	return nil
}
