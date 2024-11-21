package tokenizer

func lexPhpHeredoc(l *Lexer) lexState {
	// we have a string starting with <<<
	if !l.acceptFixed("<<<") {
		l.reset()
		return lexPhpOperator // I guess?
	}
	l.acceptSpaces()

	op := l.acceptPhpLabel()
	if op == "" {
		l.reset()
		return lexPhpOperator
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

		c := l.peek()

		switch c {
		case eof:
			l.emit(T_ENCAPSED_AND_WHITESPACE)
			//l.error("unexpected eof in heredoc")
			return nil
		case '\\':
			// advance (ignore) one
			l.next() // \
			l.next() // the escaped char
		case '$':
			// this is a variable
			if l.pos > l.start {
				l.emit(T_ENCAPSED_AND_WHITESPACE)
			}
			lexPhpVariable(l) // meh
		default:
			l.next()
		}
	}

	return l.base
}
