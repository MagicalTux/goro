package phpv

import (
	"io"
	"iter"
)

type IniValue struct {
	// values that are set in system php.ini or others
	Global *ZVal

	// values that are set by the user script,
	// using ini_set() or .htacess, or
	// user .ini files
	Local *ZVal
}

func (iv *IniValue) Get() *ZVal {
	if iv.Local != nil {
		return iv.Local
	}
	return iv.Global
}

func (iv *IniValue) GetString(ctx Context) ZString {
	if iv.Local != nil {
		return iv.Local.AsString(ctx)
	}
	return iv.Global.AsString(ctx)
}

type IniConfig interface {
	Get(name ZString) *IniValue
	CanIniSet(name ZString) bool
	RestoreConfig(ctx Context, name ZString)
	SetLocal(ctx Context, name ZString, value *ZVal) *ZVal
	SetGlobal(ctx Context, name ZString, value *ZVal) *ZVal
	IterateConfig() iter.Seq2[string, IniValue]
	Parse(ctx Context, r io.Reader) error
	EvalConfigValue(ctx Context, expr ZString) (*ZVal, error)
	LoadDefaults(ctx Context)
}
