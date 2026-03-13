package tokenizer

func lexText(l *Lexer) lexState {
	// Skip shebang line (#!) at the very start of the file
	if l.pos == 0 && l.start == 0 && l.hasPrefix("#!") {
		for {
			c := l.next()
			if c == '\n' || c == eof {
				break
			}
		}
		l.ignore()
	}
	for {
		if l.hasPrefix("<?") {
			ahead := l.peekString(6)
			isPhpTag := false
			if len(ahead) >= 3 && ahead[2] == '=' {
				// <?= echo tag - always valid
				isPhpTag = true
			} else if len(ahead) >= 5 && (ahead[2] == 'p' || ahead[2] == 'P') && (ahead[3] == 'h' || ahead[3] == 'H') && (ahead[4] == 'p' || ahead[4] == 'P') {
				// <?php - always valid
				isPhpTag = true
			} else if l.ShortOpenTag {
				// <? short tag - only valid when short_open_tag is on
				isPhpTag = true
			}
			if isPhpTag {
				if l.pos > l.start {
					l.emit(T_INLINE_HTML)
				}
				return lexPhpOpen
			}
			// short_open_tag=off and <? not followed by php or = : treat as text
			l.next() // consume '<'
			continue
		}
		if l.next() == eof {
			break
		}
		if l.output.Len() >= 8192 {
			l.emit(T_INLINE_HTML)
		}
	}

	// reached eof
	if l.pos > l.start {
		l.emit(T_INLINE_HTML)
	}
	l.emit(T_EOF)
	return nil
}

func lexPhpOpen(l *Lexer) lexState {
	l.advance(2)
	if l.peek() == '=' {
		l.next()
		l.emit(T_OPEN_TAG_WITH_ECHO)
		l.push(lexPhp)
		return l.base
	}

	l.acceptFixedI("php")
	readSpaces := l.acceptSpace()
	if !readSpaces && l.peek() > 0 && l.peekString(2) != "?>" {
		return l.error("php tag should be followed by a whitespace")
	}

	l.emit(T_OPEN_TAG)
	l.push(lexPhp)
	return l.base
}
