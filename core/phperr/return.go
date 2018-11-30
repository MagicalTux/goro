package phperr

import "github.com/MagicalTux/goro/core/phpv"

type PhpReturn struct {
	L *phpv.Loc
	V *phpv.ZVal
}

func (r *PhpReturn) Error() string {
	return "You shouldn't see this - return not caught"
}

func CatchReturn(v *phpv.ZVal, err error) (*phpv.ZVal, error) {
	if err == nil {
		return v, err
	}
	switch err := err.(type) {
	case *PhpReturn:
		return err.V, nil
	case *phpv.PhpError:
		switch err := err.Err.(type) {
		case *PhpReturn:
			return err.V, nil
		}
	}
	return v, err
}
