package tokenizer

import "strings"

func lexNumber(l *Lexer) lexState {
	// optional leading sign
	l.accept("+-")
	digits := "0123456789_"
	allowDecimal := true
	t := T_LNUMBER
	if l.accept("0") {
		// can be octal, hex, or binary
		if l.accept("xX") {
			// hex
			digits = "0123456789abcdefABCDEF_"
			allowDecimal = false
		} else if l.accept("bB") {
			// binary
			digits = "01_"
			allowDecimal = false
		} else if l.accept("oO") {
			// explicit octal (PHP 8.1+)
			digits = "01234567_"
			allowDecimal = false
		} else if l.peek() != '.' {
			// octal
			digits = "01234567_"
			allowDecimal = false
		}
	}
	l.acceptRun(digits)

	if allowDecimal {
		if l.accept(".") {
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
		l.output.Reset()
		l.output.WriteString(strings.ReplaceAll(s, "_", ""))
	}

	l.emit(t)
	return l.base
}
