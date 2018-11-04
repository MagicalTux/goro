package tokenizer

import "strings"

func lexText(l *Lexer) lexState {
	for {
		if strings.HasPrefix(l.input[l.pos:], "<?") {
			if l.pos > l.start {
				l.emit(T_INLINE_HTML)
			}
			return lexPhpOpen
		}
		if l.next() == eof {
			break
		}
	}

	// reached eof
	if l.pos > l.start {
		l.emit(T_INLINE_HTML)
	}
	l.emit(itemEOF)
	return nil
}

func lexPhpOpen(l *Lexer) lexState {
	l.pos += 2
	l.acceptFixed("php")
	if !l.acceptSpace() {
		return l.error("php tag should be followed by a whitespace")
	}
	l.emit(T_OPEN_TAG)
	return lexPhp
}
