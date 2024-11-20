package tokenizer

func lexPhpStringConst(l *Lexer) lexState {
	st_type := l.next() // " or '
	if st_type == '"' {
		// too lazy to work this out, let's switch to the other lexer
		l.emit(Rune('"'))
		l.push(lexPhpStringWhitespace)
		return l.base
	}
	if st_type == '`' {
		l.emit(Rune('`'))
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
			l.emit(Rune(c))
			l.pop() // return to previous context
			return l.base
		case '\\':
			// handle case where "\$" == "$"
			if l.hasPrefix(`\$`) {
				l.input.ReadRune() // skip \
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
			// meh :(
			return lexPhpVariable
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
			if l.prevItem != nil && l.prevItem.Type == T_VARIABLE {
				switch c {
				case '-':
					l.push(lexInterpolatedObjectOp)
					return l.base
				case '[':
					l.push(lexInterpolatedArrayAccess)
					return l.base
				default:
					l.next()
				}
			} else {
				l.next()
			}
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

	c := l.peek()
	switch {
	case '0' <= c && c <= '9':
		lexNumber(l)
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', c == '_', 0x7f <= c:
		lexPhpString(l)
	default:
		return l.error("unexpected character %c", c)
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
			l.emit(Rune('`'))
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
