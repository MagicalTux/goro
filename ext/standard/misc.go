package standard

import "git.atonline.com/tristantech/gophp/core"

//> func void exit ([ string|int $status ] )
func exit(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var ext **core.ZVal
	_, err := core.Expand(ctx, args, &ext)

	if ext == nil {
		return nil, core.ExitError(0)
	}

	z := *ext

	if z.GetType() == core.ZtInt {
		return nil, core.ExitError(z.AsInt(ctx))
	}

	z, err = z.As(ctx, core.ZtString)
	if err != nil {
		return nil, err
	}

	ctx.Write([]byte(z.String()))
	return nil, core.ExitError(0)
}
