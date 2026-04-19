package ini

import (
	"bufio"
	"fmt"
	"io"
	"iter"
	"strings"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpv"
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
			value = phpv.ZStr(entry.RawDefault)
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
		old := entry.Get()
		entry.Global = value
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
	lower := strings.ToLower(string(expr))
	switch lower {
	case "1", "on", "true", "yes":
		return phpv.ZStr("1"), nil
	case "0", "off", "false", "no":
		return phpv.ZStr("0"), nil
	case "none", "":
		return phpv.ZStr(""), nil
	case "null":
		return phpv.ZNULL.ZVal(), nil
	}
	// Fast path: if the value is a plain number or doesn't contain PHP
	// operators/constants, return it as a string without eval. This avoids
	// creating a full tokenizer+compiler for simple INI defaults.
	s := string(expr)
	if iniIsPlainValue(s) {
		return phpv.ZStr(s), nil
	}
	ctx = &IniContext{ctx.Global()}
	result, err := core.Eval(ctx, s)
	if err != nil {
		// If evaluation as PHP fails, treat the value as a raw string.
		// INI values like paths (/path/to/dir) or URLs are not PHP expressions.
		return phpv.ZStr(s), nil
	}
	return result, nil
}

// iniIsPlainValue returns true if the value doesn't need PHP evaluation.
// Returns false for strings containing PHP operators, quotes, or that look
// like PHP constants (e.g., E_ALL, PHP_INT_MAX).
func iniIsPlainValue(s string) bool {
	if len(s) == 0 {
		return true
	}
	hasUpper := false
	hasUnderscore := false
	allIdentChar := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '&', '|', '~', '^', '(', ')', '$', '"', '\'':
			return false
		}
		if c >= 'A' && c <= 'Z' {
			hasUpper = true
		} else if c == '_' {
			hasUnderscore = true
		} else if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			allIdentChar = false
		}
	}
	// If it looks like a PHP constant (uppercase with underscore, e.g., E_ALL),
	// it needs evaluation to resolve to its numeric value.
	if allIdentChar && hasUpper && hasUnderscore {
		return false
	}
	return true
}

func (c *Config) Parse(ctx phpv.Context, r io.Reader) error {
	buf := bufio.NewReader(r)
	var lineNo int

	for {
		lineNo += 1
		l, err := buf.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		atEOF := err == io.EOF
		l = strings.TrimSpace(l)
		if l == "" {
			if atEOF {
				break
			}
			continue
		}
		if l[0] == ';' {
			if atEOF {
				break
			}
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
			if atEOF {
				break
			}
			continue
		}

		// l should be in the form of var_name=value
		pos := strings.IndexByte(l, '=')
		if pos == -1 {
			// lines without values are considered to be ignored by php
			if atEOF {
				break
			}
			continue
		}

		k := strings.TrimSpace(l[:pos])
		l = strings.TrimSpace(l[pos+1:])

		// Strip inline comments: ; starts a comment unless inside quotes
		if !strings.HasPrefix(l, "\"") && !strings.HasPrefix(l, "'") {
			if semi := strings.IndexByte(l, ';'); semi != -1 {
				l = strings.TrimSpace(l[:semi])
			}
		}

		expr, err := c.EvalConfigValue(ctx, phpv.ZString(l))
		if err != nil {
			return err
		}
		// Emit deprecation warning for deprecated directives (startup only).
		if DeprecatedDirectives[k] {
			fmt.Fprintf(ctx, "\nDeprecated: %s is deprecated in Unknown on line 0\n", k)
		}
		// Emit deprecation for deprecated INI settings (assert.*) only when
		// the value differs from the default (PHP 8.3+ behavior).
		if defaultVal, ok := DeprecatedINISettings[k]; ok {
			val := strings.TrimSpace(l)
			if val != defaultVal {
				fmt.Fprintf(ctx, "\nDeprecated: PHP Startup: %s INI setting is deprecated in Unknown on line 0\n", k)
			}
		}

		c.Values[k] = &phpv.IniValue{
			Global: expr,
		}

		if atEOF {
			break
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
