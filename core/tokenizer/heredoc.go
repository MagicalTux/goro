package tokenizer

func lexPhpHeredoc(l *Lexer) lexState {
	// we have a string starting with <<<
	if !l.acceptFixed("<<<") {
		l.reset()
		// instead of returning
		// it should just lexPhpOperator(l)
		return lexPhpOperator // I guess?
	}
	l.acceptSpaces()

	isNowDoc := false
	if l.peek() == '\'' {
		l.ignore()
		l.next()
		isNowDoc = true
	}

	op := l.acceptPhpLabel()
	if op == "" {
		l.reset()
		return lexPhpOperator
	}

	if isNowDoc {
		// nowdoc is heredoc without string interpolation
		// and the identifier is single-quoted:
		// <<<'EOF'
		// EOF
		if l.peek() != '\'' {
			return l.error("unexpected character '<<' (T_SL)")
		} else {
			l.ignore()
			l.next()
		}
	}

	if !l.accept("\r\n") {
		l.reset()
		return lexPhpOperator
	}

	l.emit(T_START_HEREDOC)
	if l.hasPrefix(op) {
		// handle case where heredoc is empty
		l.advance(len(op))
		l.emit(T_END_HEREDOC)
		return l.base
	}

	op = "\n" + op

	for {
		if l.hasPrefix(op) {
			if l.pos > l.start {
				l.emit(T_ENCAPSED_AND_WHITESPACE)
			}
			l.advance(len(op))
			l.emit(T_END_HEREDOC)
			break
		}

		if isNowDoc {
			l.next()
			continue
		}

		c := l.peek()

		switch c {
		case eof:
			l.emit(T_ENCAPSED_AND_WHITESPACE)
			l.error("unexpected eof in string")
			return nil
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
				l.pushCall(lexInterpolatedComplexVar)
			} else {
				lexPhpVariable(l)
				switch c := l.peek(); c {
				case '-':
					if l.peekString(2) == "->" {
						l.pushCall(lexInterpolatedObjectOp)
					}
				case '[':
					l.pushCall(lexInterpolatedArrayAccess)
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
				l.pushCall(lexInterpolatedComplexVar)
			} else {
				l.next()
			}
		default:
			l.next()
		}
	}

	return l.base
}
