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
				l.next()
				return l.error("syntax error, unexpected identifier")
			}
			digits = "0123456789abcdefABCDEF_"
			allowDecimal = false
			hasPrefix = true
		} else if l.accept("bB") {
			// binary - underscore must not come immediately after 0b
			if l.peek() == '_' {
				l.next()
				return l.error("syntax error, unexpected identifier")
			}
			digits = "01_"
			allowDecimal = false
			hasPrefix = true
		} else if l.accept("oO") {
			// explicit octal (PHP 8.1+) - underscore must not come immediately after 0o
			if l.peek() == '_' {
				l.next()
				return l.error("syntax error, unexpected identifier")
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
		if len(valPart) > 0 && valPart[len(valPart)-1] == '_' {
			// trailing underscore
			return l.error("syntax error, unexpected identifier")
		}
		if strings.Contains(valPart, "__") {
			// adjacent underscores
			return l.error("syntax error, unexpected identifier")
		}
		if strings.Contains(valPart, "_.") || strings.Contains(valPart, "._") {
			// underscore adjacent to decimal point
			return l.error("syntax error, unexpected identifier")
		}
		// Check underscore adjacent to exponent marker
		for i := 0; i < len(valPart)-1; i++ {
			if valPart[i] == '_' && (valPart[i+1] == 'e' || valPart[i+1] == 'E') {
				return l.error("syntax error, unexpected identifier")
			}
			if (valPart[i] == 'e' || valPart[i] == 'E') && i+1 < len(valPart) {
				next := valPart[i+1]
				if next == '_' {
					return l.error("syntax error, unexpected identifier")
				}
				// Also check after sign: e+_ or e-_
				if (next == '+' || next == '-') && i+2 < len(valPart) && valPart[i+2] == '_' {
					return l.error("syntax error, unexpected identifier")
				}
			}
		}
		l.output.Reset()
		l.output.WriteString(strings.ReplaceAll(s, "_", ""))
	}

	l.emit(t)
	return l.base
}
