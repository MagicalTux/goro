package tokenizer

func lexPhp(l *Lexer) lexState {
	// let's try to find out what we are dealing with
	for {
		c := l.peek()
		switch c {
		case ' ', '\r', '\n', '\t':
			l.acceptRun(" \r\n\t")
			l.emit(T_WHITESPACE)
			return l.base
		case '(':
			return lexPhpPossibleCast
		case ')', ',', '{', '}', ';':
			l.emit(Rune(l.next()))
			return l.base
		case '$':
			return lexPhpVariable
		case '#':
			if l.hasPrefix("#[") {
				return lexPhpAttribute
			}
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
		case '*', '+', '-', '&', '|', '^', '?', '>', '=', ':', '!', '@', '[', ']', '%', '~':
			return lexPhpOperator
		case '.':
			v := l.peekString(2)
			if len(v) == 2 && v[1] >= '0' && v[1] <= '9' {
				return lexNumber
			}
			// if immediately followed by a number, this is actually a DNUMBER
			return lexPhpOperator
		case '<':
			if l.hasPrefix("<<<") {
				return lexPhpHeredoc
			}
			return lexPhpOperator
		case '\'', '`':
			return lexPhpStringConst
		case '"':
			return lexPhpStringConst
		case '\\': // T_NS_SEPARATOR
			l.next()
			l.emit(T_NS_SEPARATOR)
			return l.base
		case eof:
			l.emit(T_EOF)
			return nil
		default:
			// check for potential label start
			switch {
			case '0' <= c && c <= '9':
				return lexNumber
			case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', c == '_', 0x80 <= c:
				return lexPhpString
			}
			if c < 0x20 || c == 0x7f {
				return l.error("syntax error, unexpected character 0x%02X", c)
			}
			return l.error("syntax error, unexpected character %c", c)
		}
	}
}
