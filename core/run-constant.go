package core

import "strings"

type runConstant string

func (r runConstant) Run(ctx Context) (l *ZVal, err error) {
	switch strings.ToLower(string(r)) {
	case "null":
		return &ZVal{nil}, nil
	case "true":
		return &ZVal{ZBool(true)}, nil
	case "false":
		return &ZVal{ZBool(false)}, nil
	}

	return ctx.GetConstant(ZString(r))
}
