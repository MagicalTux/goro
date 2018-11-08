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

	z, err := ctx.GetGlobal().GetConstant(ZString(r))
	if err != nil {
		return nil, err
	}

	if z == nil {
		// TODO issue warning Use of undefined constant tata - assumed 'tata' (this will throw an Error in a future version of PHP)
		return &ZVal{ZString(r)}, nil
	}
	return z, nil
}
