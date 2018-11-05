package tokenizer

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const eof = rune(-1)

type lexState func(l *Lexer) lexState

type Lexer struct {
	input      *bufio.Reader
	start, pos int
	width      int
	items      chan *Item
	base       lexState

	output strings.Builder

	sLine, sChar int // start line/char
	cLine, cChar int // current line/char
	pLine, pChar int // previous line/char (for backup)

	baseStack []lexState
}

func NewLexer(i io.Reader) *Lexer {
	res := &Lexer{
		input: bufio.NewReader(i),
		items: make(chan *Item, 2),
		sLine: 1,
		cLine: 1,
	}

	go res.run()

	return res
}

func (l *Lexer) push(s lexState) {
	l.baseStack = append(l.baseStack, l.base)
	l.base = s
}

func (l *Lexer) pop() {
	l.base = l.baseStack[len(l.baseStack)-1]
	l.baseStack = l.baseStack[:len(l.baseStack)-1]
}

func (l *Lexer) write(s string) (int, error) {
	return l.output.Write([]byte(s))
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
	v, _ := l.input.Peek(len(s))
	return string(v) == s
}

func (l *Lexer) run() {
	l.push(lexText)
	for state := l.base; state != nil; {
		state = state(l)
	}
	close(l.items)
}

//func (l *Lexer) value() string {
//	return l.output.String()
//}

func (l *Lexer) emit(t ItemType) {
	l.items <- &Item{t, l.output.String(), l.sLine, l.sChar}
	l.start = l.pos
	l.sLine, l.sChar = l.cLine, l.cChar
	l.output.Reset()
}

func (l *Lexer) next() rune {
	var r rune
	var err error
	r, l.width, err = l.input.ReadRune()
	if err != nil {
		if err == io.EOF {
			return eof
		}
		panic(err) // TODO FIXME
	}
	l.pos += l.width
	l.pLine, l.pChar = l.cLine, l.cChar
	l.output.WriteRune(r)
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
	l.output.Reset()
}

func (l *Lexer) backup() {
	// update buffers
	l.input.UnreadRune()
	tmp := l.output.String()
	tmp = tmp[:len(tmp)-l.width] // remove
	l.output.Reset()
	l.output.WriteString(tmp)

	l.pos -= l.width
	l.cLine, l.cChar = l.pLine, l.pChar
	l.width = 0
}

func (l *Lexer) peek() rune {
	s, _ := l.input.Peek(utf8.UTFMax)
	if len(s) == 0 {
		return eof
	}
	r, _ := utf8.DecodeRune(s)
	return r
}

func (l *Lexer) peekString(ln int) string {
	s, _ := l.input.Peek(ln)
	return string(s)
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

func (l *Lexer) acceptSpaces() string {
	return l.acceptRun(" \t\r\n")
}

func (l *Lexer) acceptRun(valid string) string {
	b := &strings.Builder{}
	for {
		v := l.next()
		if strings.IndexRune(valid, v) >= 0 {
			b.WriteRune(v)
			continue
		}
		l.backup()
		return b.String()
	}
}

func (l *Lexer) acceptUntil(s string) {
	for strings.IndexRune(s, l.next()) == -1 {
	}
}

func (l *Lexer) acceptUntilFixed(s string) {
	var p int
	s2 := []rune(s)
	for {
		if p >= len(s2) {
			return // ok
		}
		c := l.next()
		if c == eof {
			return
		}
		if rune(c) == s2[p] {
			p += 1
		} else {
			p = 0
		}
	}
}

func (l *Lexer) acceptPhpLabel() string {
	// accept a php label, first char is _ or alpha, next chars are are alphanumeric or _
	labelStart := l.output.Len()
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
			return l.output.String()[labelStart:]
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
