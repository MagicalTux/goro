package ini

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

const (
	PHP_INI_NONE = 0
	PHP_INI_USER = 1 << iota
	PHP_INI_PERDIR
	PHP_INI_SYSTEM

	PHP_INI_ALL = PHP_INI_USER | PHP_INI_PERDIR | PHP_INI_SYSTEM
)

type Value struct {
	Global *phpv.ZVal
	Local  *phpv.ZVal
}

type Evaluator func(expr string) (*phpv.ZVal, error)

type Config struct {
	Values    map[string]*Value
	evaluator Evaluator
}

func NewEmpty(eval Evaluator) *Config {
	return &Config{
		Values:    map[string]*Value{},
		evaluator: eval,
	}
}

func NewWithDefaults(eval Evaluator) *Config {
	c := &Config{
		Values:    map[string]*Value{},
		evaluator: eval,
	}
	for varName, entry := range Defaults {
		expr, err := c.eval(entry.RawDefault)
		if err != nil {
			panic(err)
		}
		c.Values[varName] = &Value{Global: expr}
	}
	return c
}

func (c *Config) Get(varName string) *phpv.ZVal {
	if val, ok := c.Values[varName]; ok {
		if val.Local != nil {
			return val.Local
		}
		return val.Global
	}
	if val, ok := Defaults[varName]; ok {
		val, err := c.eval(val.RawDefault)
		if err != nil {
			panic(err)
		}
		return val
	}
	return phpv.ZNULL.ZVal()
}

func (c *Config) SetLocal(varName string, val *phpv.ZVal) {
	if _, ok := Defaults[varName]; !ok {
		return
	}
	entry, ok := c.Values[varName]
	if ok && entry != nil {
		entry.Local = val
	}

}

func (c *Config) eval(expr string) (*phpv.ZVal, error) {
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
	return c.evaluator(expr)
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

		expr, err := c.eval(l)
		if err != nil {
			return err
		}
		c.Values[k] = &Value{
			Global: expr,
		}

	}

	return nil
}
