package tokenizer

func lexPhpEolComment(l *Lexer) lexState {
	// this is a simple comment going until end of line
	l.acceptUntil("\r\n")
	l.emit(T_COMMENT)
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
