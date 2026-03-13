package tokenizer

func lexPhpEolComment(l *Lexer) lexState {
	// this is a simple comment going until end of line
	l.acceptUntil("\r\n")
	l.emit(T_COMMENT)
	return l.base
}

func lexPhpAttribute(l *Lexer) lexState {
	// Skip #[...] attribute syntax (PHP 8.0)
	// Handle nested brackets: #[Foo(arg1, [1,2])]
	l.next() // consume '#'
	l.next() // consume '['
	depth := 1
	for depth > 0 {
		c := l.next()
		switch c {
		case '[':
			depth++
		case '(':
			depth++ // count parens too for nested expressions
		case ')':
			depth--
		case ']':
			depth--
		case eof:
			break
		}
	}
	l.emit(T_COMMENT) // emit as comment so the parser ignores it
	return l.base
}

func lexPhpBlockComment(l *Lexer) lexState {
	t := T_COMMENT
	if l.hasPrefix("/**") {
		t = T_DOC_COMMENT
	}

	l.acceptUntilFixed("*/")
	l.emit(t)

	return l.base
}
