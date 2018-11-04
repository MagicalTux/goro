package tokenizer

func lexPhp(l *Lexer) lexState {
	// let's try to find out what we are dealing with
	for {
		c := l.peek()
		switch c {
		case ' ', '\r', '\n', '\t', '\f':
			l.acceptRun(" \r\n\t\f")
			l.emit(T_WHITESPACE)
		case '(':
			return lexPhpPossibleCast
		case ')', ',', '{', '}', ';':
			l.advance(1)
			l.emit(ItemSingleChar)
		case '$':
			return lexPhpVariable
		case '#':
			return lexPhpEolComment
		case '/':
			// check if // or /* (comments)
			if l.hasPrefix("//") {
				return lexPhpEolComment
			}
			if l.hasPrefix("/*") {
				return lexPhpBlockComment
			}
			return lexPhpOperator
		case '*', '+', '-', '&', '|', '^', '?', '.', '<', '>', '=', ':':
			return lexPhpOperator
		case '\'':
			return lexPhpConstantString
		case eof:
			l.emit(itemEOF)
			return nil
		default:
			// check for potential label start
			switch {
			case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', c == '_', 0x7f <= c:
				return lexPhpString
			}
			return l.error("unexpected character %c", c)
		}
	}
}
