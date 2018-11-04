package tokenizer

func lexPhpConstantString(l *Lexer) lexState {
	st_type := l.next() // " or '

	for {
		c := l.next()
		if c == st_type {
			// end of string
			l.emit(T_CONSTANT_ENCAPSED_STRING)
			return lexPhp
		}

		if c == '\\' {
			// advance (ignore) one
			l.advance(1)
			continue
		}
	}
}
