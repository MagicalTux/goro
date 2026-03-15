package tokenizer

func lexPhpEolComment(l *Lexer) lexState {
	// this is a simple comment going until end of line
	// In PHP, ?> inside a // or # comment closes the PHP tag
	for {
		if l.hasPrefix("?>") {
			// Emit the comment collected so far, then let the operator
			// handler deal with the close tag
			l.emit(T_COMMENT)
			return l.base
		}
		c := l.next()
		if c == eof || c == '\r' || c == '\n' {
			break
		}
	}
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
