package tokenizer

import (
	"fmt"
	"log"
	"strings"
	"unicode/utf8"
)

const eof = rune(-1)

type lexState func(l *Lexer) lexState

type Lexer struct {
	input      string
	start, pos int
	width      int
	items      chan *item
}

func NewLexer(i []byte) *Lexer {
	res := &Lexer{
		input: string(i),
		items: make(chan *item, 2),
	}

	go res.run()

	return res
}

func (l *Lexer) NextItem() (ItemType, string) {
	i := <-l.items
	if i == nil {
		// mh?
		return ItemError, "unable to read from lexer"
	}
	return i.t, i.data
}

func (l *Lexer) run() {
	for state := lexText; state != nil; {
		state = state(l)
	}
	close(l.items)
}

func (l *Lexer) emit(t ItemType) {
	log.Printf("emit %s", t)
	l.items <- &item{t, l.input[l.start:l.pos]}
	l.start = l.pos
}

func (l *Lexer) next() rune {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	var r rune
	r, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return r
}

func (l *Lexer) ignore() {
	l.start = l.pos
}

func (l *Lexer) backup() {
	l.pos -= l.width
	l.width = 0
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.input) {
		return eof
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.pos:])
	return r
}

func (l *Lexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.next()) >= 0 {
		return true
	}
	l.backup()
	return false
}

func (l *Lexer) acceptFixed(s string) bool {
	if !strings.HasPrefix(l.input[l.pos:], s) {
		return false
	}
	l.pos += len(s)
	return true
}

func (l *Lexer) acceptSpace() bool {
	switch l.next() {
	case ' ', '\t', '\f', '\r', '\n':
		return true
	default:
		l.backup()
		return false
	}
}

func (l *Lexer) acceptRun(valid string) {
	for strings.IndexRune(valid, l.next()) >= 0 {
	}
	l.backup()
}

func (l *Lexer) error(format string, args ...interface{}) lexState {
	l.items <- &item{
		ItemError,
		fmt.Sprintf(format, args...),
	}
	return nil
}
