package core

import (
	"bytes"

	"github.com/MagicalTux/goro/core/phpv"
)

func debugDump(v phpv.Runnable) string {
	if v == nil {
		return "<NULL>"
	}
	// simple function
	o := &bytes.Buffer{}
	err := v.Dump(o)
	if err != nil {
		return err.Error()
	}
	return o.String()
}
