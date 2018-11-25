package core

func (g *Global) Getenv(key string) (string, bool) {
	// locate env
	r, ok := g.environ.GetStringB(ZString(key))
	return r.String(), ok
}

func (g *Global) Setenv(key, value string) error {
	return g.environ.SetString(ZString(key), ZString(value).ZVal())
}

func (g *Global) Unsetenv(key string) error {
	return g.environ.UnsetString(ZString(key))
}
