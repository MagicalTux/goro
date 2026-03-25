package tokenizer

import "strings"

func lexNumber(l *Lexer) lexState {
	// optional leading sign
	l.accept("+-")
	digits := "0123456789_"
	allowDecimal := true
	t := T_LNUMBER
	hasPrefix := false // true if we consumed 0x/0b/0o prefix
	if l.accept("0") {
		// can be octal, hex, or binary
		if l.accept("xX") {
			// hex - underscore must not come immediately after 0x
			if l.peek() == '_' {
				prefix := l.output.String()
				suffix := string(prefix[len(prefix)-1]) // x or X
				l.next()
				suffix += "_"
				for isAlphaNumeric(l.peek()) || l.peek() == '_' {
					suffix += string(l.next())
				}
				return l.error("syntax error, unexpected identifier \"%s\"", suffix)
			}
			digits = "0123456789abcdefABCDEF_"
			allowDecimal = false
			hasPrefix = true
		} else if l.accept("bB") {
			// binary - underscore must not come immediately after 0b
			if l.peek() == '_' {
				prefix := l.output.String()
				suffix := string(prefix[len(prefix)-1]) // b or B
				l.next()
				suffix += "_"
				for isAlphaNumeric(l.peek()) || l.peek() == '_' {
					suffix += string(l.next())
				}
				return l.error("syntax error, unexpected identifier \"%s\"", suffix)
			}
			digits = "01_"
			allowDecimal = false
			hasPrefix = true
		} else if l.accept("oO") {
			// explicit octal (PHP 8.1+) - underscore must not come immediately after 0o
			if l.peek() == '_' {
				prefix := l.output.String()
				suffix := string(prefix[len(prefix)-1]) // o or O
				l.next()
				suffix += "_"
				for isAlphaNumeric(l.peek()) || l.peek() == '_' {
					suffix += string(l.next())
				}
				return l.error("syntax error, unexpected identifier \"%s\"", suffix)
			}
			digits = "01234567_"
			allowDecimal = false
			hasPrefix = true
		} else if l.peek() != '.' {
			// octal
			digits = "01234567_"
			allowDecimal = false
		}
	}
	_ = hasPrefix
	l.acceptRun(digits)

	if allowDecimal {
		if l.accept(".") {
			// Underscore must not appear right before or right after the decimal point
			l.acceptRun(digits)
			t = T_DNUMBER
		}
		if l.accept("eE") {
			l.accept("+-")
			l.acceptRun(digits)
			t = T_DNUMBER
		}
	}

	// next thing mustn't be alphanumeric
	if isAlphaNumeric(l.peek()) {
		l.next()
		return l.error("Invalid numeric literal")
	}

	// Strip underscores from the number before emitting
	// (PHP 7.4+ allows underscores as visual separators in numeric literals)
	s := l.output.String()
	if strings.Contains(s, "_") {
		// Validate underscore placement rules:
		// 1. No trailing underscore
		// 2. No adjacent underscores
		// 3. No underscore before/after decimal point
		// 4. No underscore before/after exponent
		stripped := s
		// Remove any leading +/- for validation
		valPart := stripped
		for len(valPart) > 0 && (valPart[0] == '+' || valPart[0] == '-') {
			valPart = valPart[1:]
		}

		// identSuffix computes the identifier that PHP would report in the error,
		// starting from the bad underscore position and consuming remaining alnum/_ chars.
		identSuffix := func(from int) string {
			// Consume any remaining alphanumeric/underscore characters from the input
			extra := ""
			for isAlphaNumeric(l.peek()) || l.peek() == '_' {
				extra += string(l.next())
			}
			suffix := valPart[from:] + extra
			return suffix
		}

		if len(valPart) > 0 && valPart[len(valPart)-1] == '_' {
			// trailing underscore - consume any remaining identifier chars
			suffix := identSuffix(len(valPart) - 1)
			return l.error("syntax error, unexpected identifier \"%s\"", suffix)
		}
		if idx := strings.Index(valPart, "__"); idx >= 0 {
			// adjacent underscores
			suffix := identSuffix(idx)
			return l.error("syntax error, unexpected identifier \"%s\"", suffix)
		}
		if idx := strings.Index(valPart, "_."); idx >= 0 {
			// underscore before decimal point
			suffix := identSuffix(idx)
			return l.error("syntax error, unexpected identifier \"%s\"", suffix)
		}
		if idx := strings.Index(valPart, "._"); idx >= 0 {
			// underscore after decimal point
			suffix := identSuffix(idx + 1)
			return l.error("syntax error, unexpected identifier \"%s\"", suffix)
		}
		// Check underscore adjacent to exponent marker (only for decimal numbers,
		// not hex/binary/octal where 'e'/'E' are valid digits)
		if allowDecimal {
			for i := 0; i < len(valPart)-1; i++ {
				if valPart[i] == '_' && (valPart[i+1] == 'e' || valPart[i+1] == 'E') {
					suffix := identSuffix(i)
					return l.error("syntax error, unexpected identifier \"%s\"", suffix)
				}
				if (valPart[i] == 'e' || valPart[i] == 'E') && i+1 < len(valPart) {
					next := valPart[i+1]
					if next == '_' {
						suffix := identSuffix(i)
						return l.error("syntax error, unexpected identifier \"%s\"", suffix)
					}
					// Also check after sign: e+_ or e-_
					if (next == '+' || next == '-') && i+2 < len(valPart) && valPart[i+2] == '_' {
						suffix := identSuffix(i)
						return l.error("syntax error, unexpected identifier \"%s\"", suffix)
					}
				}
			}
		}
		l.output.Reset()
		l.output.WriteString(strings.ReplaceAll(s, "_", ""))
	}

	l.emit(t)
	return l.base
}
