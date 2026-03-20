package compiler

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type matchArm struct {
	conditions []phpv.Runnable // nil for default arm
	body       phpv.Runnable
}

type runMatch struct {
	cond phpv.Runnable
	arms []*matchArm
	def  *matchArm // default arm, nil if none
	l    *phpv.Loc
}

func (r *runMatch) Dump(w io.Writer) error {
	_, err := w.Write([]byte("match ("))
	if err != nil {
		return err
	}
	err = r.cond.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(") { ... }"))
	return err
}

func (r *runMatch) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	cond, err := r.cond.Run(ctx)
	if err != nil {
		return nil, err
	}

	for _, arm := range r.arms {
		for _, c := range arm.conditions {
			v, err := c.Run(ctx)
			if err != nil {
				return nil, err
			}
			// match uses strict comparison (===)
			res, err := operatorCompareStrict(ctx, tokenizer.T_IS_IDENTICAL, cond, v)
			if err != nil {
				return nil, err
			}
			if res.AsBool(ctx) {
				return arm.body.Run(ctx)
			}
		}
	}

	// Try default arm
	if r.def != nil {
		return r.def.body.Run(ctx)
	}

	// No match and no default → UnhandledMatchError
	condStr := formatUnhandledMatchValue(ctx, cond)
	return nil, phpobj.ThrowError(ctx, phpobj.UnhandledMatchError,
		fmt.Sprintf("Unhandled match case %s", condStr))
}

// formatUnhandledMatchValue formats a value for the UnhandledMatchError message,
// respecting zend.exception_ignore_args and zend.exception_string_param_max_len INI settings.
func formatUnhandledMatchValue(ctx phpv.Context, val *phpv.ZVal) string {
	// Check zend.exception_ignore_args — when set, all types use "of type X"
	ignoreArgs := false
	if ctx != nil {
		ignoreVal := ctx.GetConfig("zend.exception_ignore_args", phpv.ZBool(false).ZVal())
		if ignoreVal != nil && ignoreVal.AsBool(ctx) {
			ignoreArgs = true
		}
	}

	if ignoreArgs {
		switch val.GetType() {
		case phpv.ZtNull:
			return "of type null"
		case phpv.ZtBool:
			return "of type bool"
		case phpv.ZtInt:
			return "of type int"
		case phpv.ZtFloat:
			return "of type float"
		case phpv.ZtString:
			return "of type string"
		case phpv.ZtArray:
			return "of type array"
		case phpv.ZtObject:
			if obj, ok := val.Value().(phpv.ZObject); ok {
				return fmt.Sprintf("of type %s", obj.GetClass().GetName())
			}
			return "of type object"
		default:
			return "of type " + val.GetType().TypeName()
		}
	}

	// Get max string length for exception messages
	maxLen := 15
	if ctx != nil {
		maxLenVal := ctx.GetConfig("zend.exception_string_param_max_len", phpv.ZInt(15).ZVal())
		if maxLenVal != nil {
			maxLen = int(maxLenVal.AsInt(ctx))
			if maxLen < 0 {
				maxLen = 15
			}
		}
	}

	switch val.GetType() {
	case phpv.ZtNull:
		return "NULL"
	case phpv.ZtBool:
		if val.AsBool(ctx) {
			return "true"
		}
		return "false"
	case phpv.ZtInt:
		return val.String()
	case phpv.ZtFloat:
		f := float64(val.Value().(phpv.ZFloat))
		// Use %G-like formatting that preserves .0 for whole numbers
		return formatMatchFloat(f)
	case phpv.ZtString:
		if maxLen == 0 {
			return "of type string"
		}
		s := val.String()
		return escapeMatchString(s, maxLen)
	case phpv.ZtArray:
		return "of type array"
	case phpv.ZtObject:
		if obj, ok := val.Value().(phpv.ZObject); ok {
			// Enum cases: format as EnumName::CaseName
			if obj.GetClass().GetType().Has(phpv.ZClassTypeEnum) {
				if nameVal := obj.HashTable().GetString("name"); nameVal != nil && nameVal.GetType() == phpv.ZtString {
					return fmt.Sprintf("%s::%s", obj.GetClass().GetName(), nameVal.String())
				}
			}
			return fmt.Sprintf("of type %s", obj.GetClass().GetName())
		}
		return "of type object"
	default:
		return val.String()
	}
}

// formatMatchFloat formats a float for match error messages.
// Unlike normal PHP float formatting, this preserves ".0" for whole numbers
// (e.g., 5.0 → "5.0" instead of "5").
func formatMatchFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "INF"
	}
	if math.IsInf(f, -1) {
		return "-INF"
	}
	if math.IsNaN(f) {
		return "NAN"
	}
	s := strconv.FormatFloat(f, 'G', 14, 64)
	// If the result has no decimal point or exponent, add ".0"
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	// Go's %G uses uppercase E; PHP uses uppercase E too, but we want
	// to handle the decimal-only case for consistency
	return s
}

// escapeMatchString escapes a string for the UnhandledMatchError message.
// Uses PHP-style escaping (\n, \r, \t, etc.) and truncates to maxLen characters.
func escapeMatchString(s string, maxLen int) string {
	var buf strings.Builder
	buf.WriteByte('\'')
	charCount := 0
	truncated := false
	for i := 0; i < len(s); i++ {
		if charCount >= maxLen {
			truncated = true
			break
		}
		b := s[i]
		switch b {
		case '\n':
			buf.WriteString("\\n")
		case '\r':
			buf.WriteString("\\r")
		case '\t':
			buf.WriteString("\\t")
		case '\v':
			buf.WriteString("\\v")
		case '\f':
			buf.WriteString("\\f")
		case '\\':
			buf.WriteString("\\\\")
		case '\'':
			buf.WriteString("\\'")
		default:
			buf.WriteByte(b)
		}
		charCount++
	}
	if truncated {
		buf.WriteString("...'")
	} else {
		buf.WriteByte('\'')
	}
	return buf.String()
}

func (r *runMatch) Loc() *phpv.Loc {
	return r.l
}

// compileMatch compiles a match expression:
//
//	match (expr) {
//	    cond1, cond2 => body1,
//	    cond3 => body2,
//	    default => body3,
//	}
func compileMatch(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	m := &runMatch{l: i.Loc()}

	// Expect (
	err := c.ExpectSingle('(')
	if err != nil {
		return nil, err
	}

	m.cond, err = compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	err = c.ExpectSingle(')')
	if err != nil {
		return nil, err
	}

	// Expect {
	err = c.ExpectSingle('{')
	if err != nil {
		return nil, err
	}

	// Parse arms
	hasDefault := false
	for {
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle('}') {
			break
		}

		arm := &matchArm{}

		if i.Type == tokenizer.T_DEFAULT {
			// default arm — check for duplicate
			if hasDefault {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Match expressions may only contain one default arm"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}
			hasDefault = true
			m.def = arm

			// After "default", expect either "=>" or ","
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			if i.IsSingle(',') {
				// default, => body — trailing comma after default
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
			}
			if i.Type != tokenizer.T_DOUBLE_ARROW {
				return nil, i.Unexpected()
			}
		} else {
			// Parse condition list: expr1, expr2, ... => body
			c.backup()
			for {
				cond, err := compileExpr(nil, c)
				if err != nil {
					return nil, err
				}
				arm.conditions = append(arm.conditions, cond)

				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.Type == tokenizer.T_DOUBLE_ARROW {
					break
				}
				if !i.IsSingle(',') {
					return nil, i.Unexpected()
				}
				// Check if next is => (trailing comma before =>)
				next, err := c.NextItem()
				if err != nil {
					return nil, err
				}
				if next.Type == tokenizer.T_DOUBLE_ARROW {
					break
				}
				c.backup()
			}
			m.arms = append(m.arms, arm)
		}

		// Parse body expression
		arm.body, err = compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		// Expect , or }
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.IsSingle('}') {
			break
		}
		if !i.IsSingle(',') {
			return nil, i.Unexpected()
		}
	}

	return m, nil
}
