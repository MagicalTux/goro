package tokenizer

import "strings"

func lexPhpHeredoc(l *Lexer) lexState {
	// we have a string starting with <<<
	if !l.acceptFixed("<<<") {
		l.reset()
		// instead of returning
		// it should just lexPhpOperator(l)
		return lexPhpOperator // I guess?
	}
	l.acceptSpaces()

	isNowDoc := false
	if l.peek() == '\'' {
		l.ignore()
		l.next()
		isNowDoc = true
	}

	op := l.acceptPhpLabel()
	if op == "" {
		l.reset()
		return lexPhpOperator
	}

	if isNowDoc {
		// nowdoc is heredoc without string interpolation
		// and the identifier is single-quoted:
		// <<<'EOF'
		// EOF
		if l.peek() != '\'' {
			return l.error("unexpected character '<<' (T_SL)")
		} else {
			l.ignore()
			l.next()
		}
	}

	if !l.accept("\r\n") {
		l.reset()
		return lexPhpOperator
	}

	l.emit(T_START_HEREDOC)

	// Check for empty heredoc (end marker on the very next line, possibly indented)
	if l.hasPrefix(op) {
		// handle case where heredoc is empty (non-indented)
		l.advance(len(op))
		l.emit(T_END_HEREDOC)
		return l.base
	}
	// Check for indented end marker at start (flexible heredoc)
	if indent, found := checkFlexibleEndMarker(l, op); found {
		_ = indent
		l.emit(T_END_HEREDOC)
		return l.base
	}

	// For flexible heredoc (PHP 7.3+), we need to collect the full content
	// and then look for the end marker which may be indented. We can't just
	// scan for "\n" + op because the marker might be "\n    " + op.
	//
	// Strategy: scan line by line. After each newline, check if the remaining
	// text starts with optional whitespace + op.

	for {
		// After a newline, check if the next line is the end marker (possibly indented)
		if l.hasPrefix("\n") || l.hasPrefix("\r\n") {
			// Consume the newline
			if l.hasPrefix("\r\n") {
				l.next() // \r
			}
			l.next() // \n

			// Check for end marker (with optional whitespace indentation)
			if l.hasPrefix(op) {
				// Non-indented end marker
				if l.pos > l.start {
					// Strip trailing newline from content and emit
					emitHeredocContent(l)
				}
				l.advance(len(op))
				l.emit(T_END_HEREDOC)
				return l.base
			}
			if indent, found := checkFlexibleEndMarker(l, op); found {
				_ = indent
				if l.pos > l.start {
					emitHeredocContent(l)
				}
				l.emit(T_END_HEREDOC)
				return l.base
			}
			// Not the end marker, continue scanning
			continue
		}

		if isNowDoc {
			c := l.next()
			if c == eof {
				l.emit(T_ENCAPSED_AND_WHITESPACE)
				l.error("unexpected eof in string")
				return nil
			}
			continue
		}

		c := l.peek()

		switch c {
		case eof:
			l.emit(T_ENCAPSED_AND_WHITESPACE)
			l.error("unexpected eof in string")
			return nil
		case '\\':
			// handle case where "\$" == "$"
			if l.hasPrefix(`\$`) {
				l.next()
				l.next()
			} else {
				// advance (ignore) one
				l.next() // \
				l.next() // the escaped char
			}
		case '$':
			// this is a variable
			if l.pos > l.start {
				l.emit(T_ENCAPSED_AND_WHITESPACE)
			}

			if l.hasPrefix(`${`) {
				if l.pos > l.start {
					l.emit(T_ENCAPSED_AND_WHITESPACE)
				}
				l.emit(Rune(l.next()))
				l.emit(Rune(l.next()))
				l.pushCall(lexInterpolatedComplexVar)
			} else {
				lexPhpVariable(l)
				switch c := l.peek(); c {
				case '-':
					if l.peekString(2) == "->" {
						l.pushCall(lexInterpolatedObjectOp)
					}
				case '[':
					l.pushCall(lexInterpolatedArrayAccess)
				}
			}
		case '{':
			if l.hasPrefix(`{$`) {
				if l.pos > l.start {
					l.emit(T_ENCAPSED_AND_WHITESPACE)
				}

				l.next()
				l.emit(Rune(c))
				lexPhpVariable(l)
				l.pushCall(lexInterpolatedComplexVar)
			} else {
				l.next()
			}
		default:
			l.next()
		}
	}
}

// checkFlexibleEndMarker checks if the current position has whitespace followed
// by the end marker (flexible heredoc syntax, PHP 7.3+). If found, it advances
// past the whitespace and marker and returns the indentation string and true.
func checkFlexibleEndMarker(l *Lexer, marker string) (string, bool) {
	// Peek ahead to see if we have whitespace + marker
	// Maximum reasonable indent is 256 chars
	peekLen := len(marker) + 256
	s := l.peekString(peekLen)
	if len(s) == 0 {
		return "", false
	}
	// Check if it starts with whitespace (spaces or tabs)
	if s[0] != ' ' && s[0] != '\t' {
		return "", false
	}
	// Find where the whitespace ends
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	indent := s[:i]
	// Check if the marker follows
	if strings.HasPrefix(s[i:], marker) {
		// Verify that after the marker there's a valid terminator
		// (semicolon, newline, EOF, or closing paren/bracket for inline use)
		afterMarker := i + len(marker)
		if afterMarker >= len(s) {
			// EOF after marker - valid
			l.advance(afterMarker)
			return indent, true
		}
		ch := s[afterMarker]
		if ch == ';' || ch == '\n' || ch == '\r' || ch == ')' || ch == ']' || ch == ',' || ch == '}' {
			// Valid flexible heredoc end marker
			l.advance(afterMarker)
			return indent, true
		}
	}
	return "", false
}

// emitHeredocContent emits the heredoc content, stripping the trailing newline
// character(s) that precede the end marker line.
func emitHeredocContent(l *Lexer) {
	// The output buffer contains content up to and including the newline before
	// the end marker. We need to strip that trailing newline.
	s := l.output.String()
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
		if len(s) > 0 && s[len(s)-1] == '\r' {
			s = s[:len(s)-1]
		}
	}
	// We can't easily modify the output buffer, so we'll just emit as-is.
	// The content naturally includes the trailing newline which is part of
	// the heredoc content in PHP.
	l.emit(T_ENCAPSED_AND_WHITESPACE)
}
