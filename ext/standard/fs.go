package standard

import (
	"path"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

//> func string dirname ( string $path [, int $levels = 1 ] )
func fncDirname(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var p string
	var lvl *phpv.ZInt
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
		return phpv.ZString(path.Dir(p)).ZVal(), nil
	}

	for i := phpv.ZInt(0); i < *lvl; i++ {
		p = path.Dir(p)
	}
	return phpv.ZString(p).ZVal(), nil
}
