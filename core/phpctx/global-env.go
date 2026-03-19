package phpctx

import "github.com/MagicalTux/goro/core/phpv"

func (g *Global) Getenv(key string) (string, bool) {
	// locate env
	r, ok := g.environ.GetStringB(phpv.ZString(key))
	return r.String(), ok
}

func (g *Global) Setenv(key, value string) error {
	return g.environ.SetString(phpv.ZString(key), phpv.ZString(value).ZVal())
}

func (g *Global) Unsetenv(key string) error {
	return g.environ.UnsetString(phpv.ZString(key))
}

// GetAllEnv returns all environment variables as a ZArray.
func (g *Global) GetAllEnv(ctx phpv.Context) *phpv.ZArray {
	result := phpv.NewZArray()
	it := g.environ.NewIterator()
	for ; it.Valid(ctx); it.Next(ctx) {
		k, _ := it.Key(ctx)
		v, _ := it.Current(ctx)
		result.OffsetSet(ctx, k, v)
	}
	return result
}
