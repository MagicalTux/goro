package standard

import (
	"path"

	"github.com/MagicalTux/goro/core"
)

//> func string dirname ( string $path [, int $levels = 1 ] )
func fncDirname(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var p string
	var lvl *core.ZInt
	_, err := core.Expand(ctx, args, &p, &lvl)
	if err != nil {
		return nil, err
	}

	for {
		if len(p) == 1 {
			break
		}
		if p[len(p)-1] != '/' {
			break
		}
		p = p[:len(p)-1]
	}

	if lvl == nil {
		return core.ZString(path.Dir(p)).ZVal(), nil
	}

	for i := core.ZInt(0); i < *lvl; i++ {
		p = path.Dir(p)
	}
	return core.ZString(p).ZVal(), nil
}
