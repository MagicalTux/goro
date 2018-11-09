package standard

import (
	"fmt"

	"git.atonline.com/tristantech/gophp/core"
)

func stdFuncVarDump(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	for _, z := range args {
		doVarDump(ctx, z, "")
	}
	return nil, nil
}

func doVarDump(ctx core.Context, z *core.ZVal, linePfx string) {
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
		fmt.Fprintf(ctx, "%sfloat(%g)\n", linePfx, z.Value())
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
			k := it.Key(ctx)
			if k.GetType() == core.ZtInt {
				fmt.Fprintf(ctx, "%s[%s]=>\n", localPfx, k)
			} else {
				fmt.Fprintf(ctx, "%s[\"%s\"]=>\n", localPfx, k)
			}
			doVarDump(ctx, it.Current(ctx), localPfx)
			it.Next(ctx)
		}
		fmt.Fprintf(ctx, "%s}\n", linePfx)
	default:
		fmt.Fprintf(ctx, "Unknown[%T]:%+v\n", z.Value(), z.Value())
	}
}
