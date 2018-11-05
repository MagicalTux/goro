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
			l.next()
			l.emit(ItemSingleChar)
			l.push(lexPhpStringWhitespace)
			return l.base
		}
	}
}

func lexPhpStringWhitespace(l *Lexer) lexState {
	for {
		c := l.peek()

		switch c {
		case '"':
			// end of string
			if l.pos > l.start {
				l.emit(T_ENCAPSED_AND_WHITESPACE)
			}
			l.next() // "
			l.emit(ItemSingleChar)
			l.pop() // return to previous context
			return l.base
		case '\\':
			// advance (ignore) one
			l.next() // \
			l.next() // the escaped char
		case '$':
			// this is a variable
			if l.pos > l.start {
				l.emit(T_ENCAPSED_AND_WHITESPACE)
			}
			return lexPhpVariable
		default:
			l.next()
		}
	}
}
