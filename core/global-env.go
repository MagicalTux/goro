package core

import "strings"

func (g *Global) Getenv(key string) (string, bool) {
	// locate env
	env := g.environ
	if env == nil {
		env = g.p.environ
	}
	pfx := key + "="

	for _, s := range env {
		if strings.HasPrefix(s, pfx) {
			return s[len(pfx):], true
		}
	}
	return "", false
}

func (g *Global) Setenv(key, value string) error {
	if g.environ == nil {
		// if no environ for this global, copy from process
		g.environ = make([]string, len(g.p.environ))
		for k, v := range g.p.environ {
			g.environ[k] = v
		}
	}
	// lookup if it exists
	pfx := key + "="
	for i, s := range g.environ {
		if strings.HasPrefix(s, pfx) {
			// hit
			g.environ[i] = pfx + value
			return nil
		}
	}

	// no hit
	g.environ = append(g.environ, pfx+value)
	return nil
}

func (g *Global) Unsetenv(key string) error {
	if g.environ == nil {
		// if no environ for this global, copy from process
		g.environ = make([]string, len(g.p.environ))
		for k, v := range g.p.environ {
			g.environ[k] = v
		}
	}
	// lookup if it exists
	pfx := key + "="

	for i, s := range g.environ {
		if strings.HasPrefix(s, pfx) {
			g.environ = append(g.environ[:i], g.environ[i+1:]...)
			return nil
		}
	}
	return nil
}
