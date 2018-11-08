package standard

import (
	"fmt"

	"git.atonline.com/tristantech/gophp/core"
)

func stdFuncVarDump(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	for _, z := range args {
		doVarDump(ctx, z)
	}
	return nil, nil
}

func doVarDump(ctx core.Context, z *core.ZVal) {
	switch z.GetType() {
	case core.ZtNull:
		ctx.Write([]byte("NULL\n"))
	case core.ZtBool:
		if z.Value().(core.ZBool) {
			ctx.Write([]byte("bool(true)\n"))
		} else {
			ctx.Write([]byte("bool(false)\n"))
		}
	case core.ZtInt:
		fmt.Fprintf(ctx, "int(%d)\n", z.Value())
	case core.ZtFloat:
		fmt.Fprintf(ctx, "float(%g)\n", z.Value())
	case core.ZtString:
		s := z.Value().(core.ZString)
		fmt.Fprintf(ctx, "string(%d) \"%s\"\n", len(s), s)
	default:
		fmt.Fprintf(ctx, "Unknown[%T]:%+v\n", z.Value(), z.Value())
	}
}
