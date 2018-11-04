package tokenizer

import "strings"

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

	p := strings.Index(l.input[l.pos:], "*/")
	if p == -1 {
		l.pos = len(l.input)
		l.emit(t)
		return l.base
	}

	l.advance(p + 2)
	l.emit(t)

	return l.base
}
