package tokenizer

func lexPhpEolComment(l *Lexer) lexState {
	// this is a simple comment going until end of line
	l.acceptUntil("\r\n")
	l.emit(T_COMMENT)
	return lexPhp
}
