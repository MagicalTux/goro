package ini

import (
	"bufio"
	"fmt"
	"io"
	"iter"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

const (
	INI_NONE = 0
	INI_USER = 1 << iota
	INI_PERDIR
	INI_SYSTEM

	INI_ALL = INI_USER | INI_PERDIR | INI_SYSTEM
)

type Config struct {
	Values map[string]*phpv.IniValue
	ctx    phpv.Context
}

func New() *Config {
	var iniCtx = phpctx.NewIniContext(&Config{
		Values: map[string]*phpv.IniValue{},
	})
	c := &Config{
		Values: map[string]*phpv.IniValue{},
		ctx:    iniCtx,
	}
	for varName, entry := range Defaults {
		expr, err := c.EvalConfigValue(entry.RawDefault)
		if err != nil {
			panic(fmt.Sprintf("failed to initialize ini default for %s: %s", varName, err))
		}
		c.Values[varName] = &phpv.IniValue{Global: expr}
	}
	return c
}

func (c *Config) Get(varName phpv.ZString) *phpv.IniValue {
	if val, ok := c.Values[string(varName)]; ok {
		return val
	}
	return nil
}

func (c *Config) SetLocal(varName phpv.ZString, val *phpv.ZVal) {
	if _, ok := Defaults[string(varName)]; !ok {
		return
	}
	entry, ok := c.Values[string(varName)]
	if ok && entry != nil {
		entry.Local = val
	}
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

func (c *Config) EvalConfigValue(expr string) (*phpv.ZVal, error) {
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
	return core.Eval(c.ctx, expr)
}

func (c *Config) Parse(r io.Reader) error {
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

		expr, err := c.EvalConfigValue(l)
		if err != nil {
			return err
		}
		c.Values[k] = &phpv.IniValue{
			Global: expr,
		}

	}

	return nil
}
