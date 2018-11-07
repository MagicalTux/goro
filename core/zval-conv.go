package core

import "errors"

func (z *ZVal) As(t ZType) (*ZVal, error) {
	if z.v.GetType() == t {
		// nothing to do
		return z, nil
	}

	return nil, errors.New("todo")
}
