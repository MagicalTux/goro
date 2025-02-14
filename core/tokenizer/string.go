package tokenizer

func lexPhpStringConst(l *Lexer) lexState {
	st_type := l.next() // " or '
	if st_type == '"' {
		// too lazy to work this out, let's switch to the other lexer
		l.emit(Rune('"'))
		l.push(stringLexer{'"'}.lexStringWhitespace)
		return l.base
	}
	if st_type == '`' {
		l.emit(Rune('`'))
		l.push(stringLexer{'`'}.lexStringWhitespace)
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

type stringLexer struct {
	delimeter rune
}

func (sl stringLexer) lexStringWhitespace(l *Lexer) lexState {
	for {
		c := l.peek()

		switch c {
		case eof:
			l.emit(T_ENCAPSED_AND_WHITESPACE)
			l.error("unexpected eof in string")
			return nil
		case sl.delimeter:
			// end of string
			if l.pos > l.start {
				l.emit(T_ENCAPSED_AND_WHITESPACE)
			}
			l.next() // "
			l.emit(Rune(c))
			l.pop() // return to previous context
			return l.base
		case '\\':
			// handle case where "\$" == "$"
			if l.hasPrefix(`\$`) {
				l.next()
				l.next()
			} else {
				// advance (ignore) one
				l.next() // \
				l.next() // the escaped char
			}
		case '$':
			// this is a variable
			if l.pos > l.start {
				l.emit(T_ENCAPSED_AND_WHITESPACE)
			}

			if l.hasPrefix(`${`) {
				if l.pos > l.start {
					l.emit(T_ENCAPSED_AND_WHITESPACE)
				}
				l.emit(Rune(l.next()))
				l.emit(Rune(l.next()))

				l.push(lexInterpolatedComplexVar)
				return l.base
			} else {
				lexPhpVariable(l)
				switch c := l.peek(); c {
				case '-':
					if l.peekString(2) == "->" {
						l.push(lexInterpolatedObjectOp)
						return l.base
					}
				case '[':
					l.push(lexInterpolatedArrayAccess)
					return l.base
				}
			}
		case '{':
			if l.hasPrefix(`{$`) {
				if l.pos > l.start {
					l.emit(T_ENCAPSED_AND_WHITESPACE)
				}

				l.next()
				l.emit(Rune(c))
				lexPhpVariable(l)
				l.push(lexInterpolatedComplexVar)
				return l.base
			} else {
				l.next()
			}
		default:
			l.next()
		}
	}
}

func lexInterpolatedObjectOp(l *Lexer) lexState {
	lexPhpOperator(l)
	lexPhpString(l)
	l.pop()
	return l.base
}

func lexInterpolatedArrayAccess(l *Lexer) lexState {
	lexPhpOperator(l)

	switch c := l.peek(); c {
	case '$':
		lexPhpVariable(l)
	default:
		switch {
		case '0' <= c && c <= '9':
			lexNumber(l)
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', c == '_', 0x7f <= c:
			lexPhpString(l)
		default:
			return l.error("unexpected character %c", c)
		}
	}

	lexPhpOperator(l)

	l.pop()
	return l.base
}

func lexInterpolatedComplexVar(l *Lexer) lexState {
	c := l.peek()
	if c == '}' {
		l.emit(Rune(l.next()))
		l.pop()
		return l.base
	}

	return lexPhp(l)
}
