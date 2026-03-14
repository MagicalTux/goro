package tokenizer

func lexPhpEolComment(l *Lexer) lexState {
	// this is a simple comment going until end of line
	l.acceptUntil("\r\n")
	l.emit(T_COMMENT)
	return l.base
}

func lexPhpAttribute(l *Lexer) lexState {
	// Emit #[ as a T_ATTRIBUTE token (PHP 8.0)
	l.next() // consume '#'
	l.next() // consume '['
	l.emit(T_ATTRIBUTE)
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
