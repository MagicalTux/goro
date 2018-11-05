package tokenizer

func lexNumber(l *Lexer) lexState {
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
	return l.base
}
