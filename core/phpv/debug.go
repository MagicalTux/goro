package phpv

import (
	"bytes"
)

func DebugDump(v Runnable) string {
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
