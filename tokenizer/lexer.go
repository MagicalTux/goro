package tokenizer

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const eof = rune(-1)

type lexState func(l *Lexer) lexState

type Lexer struct {
	input      string
	start, pos int
	width      int
	items      chan *Item

	sLine, sChar int // start line/char
	cLine, cChar int // current line/char

	state []*lexerState
}

type lexerState struct {
	start, pos int
	width      int

	sLine, sChar int // start line/char
	cLine, cChar int // current line/char
}

func NewLexer(i []byte) *Lexer {
	res := &Lexer{
		input: string(i),
		items: make(chan *Item, 2),
		sLine: 1,
		cLine: 1,
	}

	go res.run()

	return res
}

func (l *Lexer) pushState() {
	s := &lexerState{
		start: l.start,
		pos:   l.pos,
		width: l.width,
		sLine: l.sLine,
		sChar: l.sChar,
		cLine: l.cLine,
		cChar: l.cChar,
	}

	l.state = append(l.state, s)
}

func (l *Lexer) popState() {
	s := l.state[len(l.state)-1]
	l.state = l.state[:len(l.state)-1]

	l.start, l.pos = s.start, s.pos
	l.width = s.width
	l.sLine, l.sChar = s.sLine, s.sChar
	l.cLine, l.cChar = s.cLine, l.cChar
}

func (l *Lexer) NextItem() (*Item, error) {
	i := <-l.items
	if i == nil {
		// mh?
		return nil, io.EOF
	}
	if i.Type == itemError {
		return nil, errors.New(i.Data)
	}
	if i.Type == itemEOF {
		return nil, io.EOF
	}
	return i, nil
}

func (l *Lexer) hasPrefix(s string) bool {
	if len(s) > len(l.input)-l.pos {
		return false
	}

	return l.input[l.pos:l.pos+len(s)] == s
}

func (l *Lexer) run() {
	for state := lexText; state != nil; {
		state = state(l)
	}
	close(l.items)
}

func (l *Lexer) value() string {
	return l.input[l.start:l.pos]
}

func (l *Lexer) emit(t ItemType) {
	l.items <- &Item{t, l.input[l.start:l.pos], l.sLine, l.sChar}
	l.start = l.pos
	l.sLine, l.sChar = l.cLine, l.cChar
	l.state = nil
}

func (l *Lexer) next() rune {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	var r rune
	r, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	l.cChar += 1 // char counts in characters, not in bytes
	if r == '\n' {
		l.cLine += 1
		l.cChar = 0
	}
	return r
}

func (l *Lexer) ignore() {
	l.start = l.pos
	l.sLine, l.sChar = l.cLine, l.cChar
}

func (l *Lexer) backup() {
	l.pos -= l.width
	l.cChar -= 1 // could end at pos -1 (unlikely)
	l.width = 0
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.input) {
		return eof
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.pos:])
	return r
}

func (l *Lexer) peekString(ln int) string {
	if l.pos+ln >= len(l.input) {
		return ""
	}
	return l.input[l.pos : l.pos+ln]
}

func (l *Lexer) advance(c int) {
	for i := 0; i < c; i += 1 {
		// we do that for two purposes:
		// 1. correctly skip utf-8 characters
		// 2. detect linebreaks so we count these correctly
		l.next()
	}
}

func (l *Lexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.next()) >= 0 {
		return true
	}
	l.backup()
	return false
}

func (l *Lexer) acceptFixed(s string) bool {
	if !l.hasPrefix(s) {
		return false
	}
	l.advance(len([]rune(s))) // CL 108985 (May 2018, for Go 1.11)
	return true
}

func (l *Lexer) acceptSpace() bool {
	return l.accept(" \t\r\n")
}

func (l *Lexer) acceptSpaces() {
	l.acceptRun(" \t\r\n")
}

func (l *Lexer) acceptRun(valid string) {
	for strings.IndexRune(valid, l.next()) >= 0 {
	}
	l.backup()
}

func (l *Lexer) acceptUntil(s string) {
	for strings.IndexRune(s, l.next()) == -1 {
	}
}

func (l *Lexer) acceptPhpLabel() string {
	// accept a php label, first char is _ or alpha, next chars are are alphanumeric or _
	labelStart := l.pos
	c := l.next()
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', c == '_', 0x7f <= c:
	default:
		l.backup()
		// we didn't read a single char
		return ""
	}

	for {
		c := l.next()
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', c == '_', 0x7f <= c:
		default:
			l.backup()
			return l.input[labelStart:l.pos]
		}
	}
}

func (l *Lexer) error(format string, args ...interface{}) lexState {
	l.items <- &Item{
		itemError,
		fmt.Sprintf(format, args...),
		l.sLine, l.sChar,
	}
	return nil
}
