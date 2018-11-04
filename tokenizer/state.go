package tokenizer

import "strings"

type lexState func(l *Lexer) lexState

func lexText(l *Lexer) lexState {
	for {
		if strings.HasPrefix(l.input[l.pos:], "<?") {
			if l.pos > l.start {
				l.emit(ItemText)
			}
			return lexPhpOpen
		}
		if l.next() == eof {
			break
		}
	}

	// reached eof
	if l.pos > l.start {
		l.emit(ItemText)
	}
	l.emit(ItemEOF)
	return nil
}

func lexPhpOpen(l *Lexer) lexState {
	return nil // TODO
}

func lexNumber(l Lexer) lexState {
	// optional leading sign
	l.accept("+-")
	digits := "0123456789"
	allowDecimal := true
	t := T_LNUMBER
	if l.accept("0") {
		allowDecimal = false
		// can be octal or hexa
		if l.accept("xX") {
			// hex
			digits = "0123456789abcdefABCDEF"
		} else {
			// octal
			digits = "01234567"
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
		return l.error("bad number syntax")
	}
	l.emit(t)
	return nil // TODO
}
