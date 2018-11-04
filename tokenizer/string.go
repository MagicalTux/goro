package tokenizer

func lexPhpStringConst(l *Lexer) lexState {
	l.pushState()

	st_type := l.next() // " or '

	for {
		c := l.next()
		if c == st_type {
			// end of string
			l.emit(T_CONSTANT_ENCAPSED_STRING)
			return l.base
		}

		if c == '\\' {
			// advance (ignore) one
			l.next()
			continue
		}

		if st_type == '"' && c == '$' {
			// need to switch to whitespace variation
			l.popState()
			return lexPhpStringWhitespace
		}
	}
}

func lexPhpStringWhitespace(l *Lexer) lexState {
	// starts with "
	l.next()
	l.emit(ItemSingleChar)

	for {
		c := l.next()

		switch c {
		case '"':
			// end of string
			l.backup()
			l.emit(T_ENCAPSED_AND_WHITESPACE)
			l.next() // "
			l.emit(ItemSingleChar)
			return l.base
		case '\\':
			// advance (ignore) one
			l.next()
		case '$':
			// this is a variable
			l.backup()
			if l.pos > l.start {
				l.emit(T_ENCAPSED_AND_WHITESPACE)
			}
			l.next() // $
			if l.acceptPhpLabel() == "" {
				l.emit(ItemSingleChar)
			} else {
				l.emit(T_VARIABLE)
			}
		}
	}
}
