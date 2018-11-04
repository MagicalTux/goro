package tokenizer

func lexPhp(l *Lexer) lexState {
	// let's try to find out what we are dealing with
	for {
		c := l.peek()
		switch c {
		case ' ', '\r', '\n', '\t', '\f':
			l.acceptRun(" \r\n\t\f")
			l.emit(T_WHITESPACE)
		default:
			return l.error("unexpected character %c", c)
		}
	}
}
