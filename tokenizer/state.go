package tokenizer

import "strings"

type lexState func(l *Lexer) lexState

func lexText(l *Lexer) lexState {
	for {
		if strings.HasPrefix(l.input[l.pos:], "<?") {
			if l.pos > l.start {
				l.emit(ItemText)
			}
			return lexPhpOpen
		}
		if l.next() == eof {
			break
		}
	}

	// reached eof
	if l.pos > l.start {
		l.emit(ItemText)
	}
	l.emit(ItemEOF)
	return nil
}

func lexPhpOpen(l *Lexer) lexState {
	return nil // TODO
}
