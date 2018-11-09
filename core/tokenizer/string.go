package tokenizer

func lexPhpStringConst(l *Lexer) lexState {
	st_type := l.next() // " or '
	if st_type == '"' {
		// too lazy to work this out, let's switch to the other lexer
		l.emit(ItemSingleChar)
		l.push(lexPhpStringWhitespace)
		return l.base
	}
	if st_type == '`' {
		l.emit(ItemSingleChar)
		l.push(lexPhpStringWhitespaceBack)
		return l.base
	}

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
	}
}

func lexPhpStringWhitespace(l *Lexer) lexState {
	for {
		c := l.peek()

		switch c {
		case eof:
			l.emit(T_ENCAPSED_AND_WHITESPACE)
			l.error("unexpected eof in string")
			return nil
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
			// meh :(
			return lexPhpVariable
		default:
			l.next()
		}
	}
}

func lexPhpStringWhitespaceBack(l *Lexer) lexState {
	for {
		c := l.peek()

		switch c {
		case eof:
			l.emit(T_ENCAPSED_AND_WHITESPACE)
			l.error("unexpected eof in string")
			return nil
		case '`':
			// end of string
			if l.pos > l.start {
				l.emit(T_ENCAPSED_AND_WHITESPACE)
			}
			l.next() // `
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
			// meh :(
			return lexPhpVariable
		default:
			l.next()
		}
	}
}
