package ini

import (
	"bufio"
	"fmt"
	"io"
	"iter"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

const (
	INI_NONE = 0
	INI_USER = 1 << iota
	INI_PERDIR
	INI_SYSTEM

	INI_ALL = INI_USER | INI_PERDIR | INI_SYSTEM
)

// notes:
// - ini_set and ini_get always return a string
// - if $x is not a string, then ini_set("foo", $x)
//   will convert the string first
// - malformed expressions are treated as strings
//   when evaluating ini values from files or CLI

type Config struct {
	Values map[string]*phpv.IniValue
}

type IniContext struct {
	phpv.GlobalContext
}

func (ic *IniContext) Global() phpv.GlobalContext {
	return ic
}

func (ic *IniContext) ConstantGet(k phpv.ZString) (phpv.Val, bool) {
	// override so no warnings are shown on non-existent constants
	// e.g., just return the CONSTANT_FOO as "CONSTANT_FOO"
	if v, ok := ic.GlobalContext.ConstantGet(k); ok {
		return v, true
	}
	return k.ZVal(), true
}

// ideally, ini values will have a separate mini-compilers,
// but this will do for now
func GetFunction(ctx phpv.Context, name phpv.ZString) (phpv.Callable, error) {
	return nil, ctx.Errorf("Cannot use functions inside ini")
}
func GetClass(ctx phpv.Context, name phpv.ZString, autoload bool) (phpv.ZClass, error) {
	return nil, ctx.Errorf("Cannot use classes inside ini")
}

func New() phpv.IniConfig {
	c := &Config{
		Values: map[string]*phpv.IniValue{},
	}
	return c
}

func (c *Config) LoadDefaults(ctx phpv.Context) {
	for varName, entry := range Defaults {
		value, err := c.EvalConfigValue(ctx, phpv.ZString(entry.RawDefault))
		if err != nil {
			panic(fmt.Sprintf("failed to initialize ini default for %s: %s", varName, err))
		}
		c.Values[varName] = &phpv.IniValue{Global: value}
	}
}

func (c *Config) Get(varName phpv.ZString) *phpv.IniValue {
	if val, ok := c.Values[string(varName)]; ok {
		return val
	}
	return nil
}

func (c *Config) RestoreConfig(ctx phpv.Context, varName phpv.ZString) {
	if val, ok := c.Values[string(varName)]; ok {
		val.Local = nil
	}
}

func (c *Config) SetLocal(ctx phpv.Context, varName phpv.ZString, value *phpv.ZVal) *phpv.ZVal {
	if _, ok := Defaults[string(varName)]; !ok {
		return nil
	}

	entry, ok := c.Values[string(varName)]
	if ok && entry != nil {
		old := entry.Local
		if old == nil {
			old = entry.Global
		}

		entry.Local = value
		return old
	}
	return nil
}

func (c *Config) SetGlobal(ctx phpv.Context, varName phpv.ZString, value *phpv.ZVal) *phpv.ZVal {
	if _, ok := Defaults[string(varName)]; !ok {
		return nil
	}

	entry, ok := c.Values[string(varName)]
	if ok && entry != nil {
		old := entry.Local
		if old == nil {
			old = entry.Global
		}

		entry.Local = value
		return old
	}
	return nil
}

func (c *Config) IterateConfig() iter.Seq2[string, phpv.IniValue] {
	return func(yield func(key string, value phpv.IniValue) bool) {
		for k, v := range c.Values {
			proceed := yield(k, phpv.IniValue{
				Global: v.Global,
				Local:  v.Local,
			})
			if !proceed {
				break
			}
		}
	}
}

func (c *Config) EvalConfigValue(ctx phpv.Context, expr phpv.ZString) (*phpv.ZVal, error) {
	switch expr {
	case "1", "On", "True", "Yes":
		return phpv.ZStr("1"), nil
	case "0", "Off", "False", "No":
		return phpv.ZStr("0"), nil
	case "None", "":
		return phpv.ZStr(""), nil
	case "NULL", "null":
		return phpv.ZNULL.ZVal(), nil
	}
	ctx = &IniContext{ctx.Global()}
	return core.Eval(ctx, string(expr))
}

func (c *Config) Parse(ctx phpv.Context, r io.Reader) error {
	buf := bufio.NewReader(r)
	var lineNo int

	for {
		lineNo += 1
		l, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		l = strings.TrimSpace(l)
		if l == "" {
			// empty line
			continue
		}
		if l[0] == ';' {
			// comment only line
			continue
		}

		if l[0] == '[' {
			// this is a section identifier

			// check for comments
			pos := strings.IndexByte(l, ';')
			if pos != -1 {
				l = strings.TrimSpace(l[:pos])
			}

			if l[len(l)-1] != ']' {
				// syntax error
				return fmt.Errorf("ini: unable to parse section declaration on line %d", lineNo)
			}

			// s = l[1 : len(l)-1]
			continue
		}

		// l should be in the form of var_name=value
		pos := strings.IndexByte(l, '=')
		if pos == -1 {
			// lines without values are considered to be ignored by php
			continue
		}

		k := l[:pos]
		l = l[pos+1:]

		expr, err := c.EvalConfigValue(ctx, phpv.ZString(l))
		if err != nil {
			return err
		}
		c.Values[k] = &phpv.IniValue{
			Global: expr,
		}

	}

	return nil
}

func (c *Config) CanIniSet(name phpv.ZString) bool {
	if val, ok := Defaults[string(name)]; ok {
		return val.Mode&(INI_USER) > 0
	}
	return false
}
