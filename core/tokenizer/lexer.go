package tokenizer

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core/phpv"
)

const eof = rune(-1)

type lexState func(l *Lexer) lexState

type Lexer struct {
	input      *bufio.Reader
	fn         string
	start, pos int
	width      int
	items      chan *Item
	prevItem   *Item
	base       lexState
	done       chan struct{} // closed to signal the lexer goroutine to stop

	inputRst []byte
	output   bytes.Buffer

	sLine, sChar int // start line/char
	cLine, cChar int // current line/char
	pLine, pChar int // previous line/char (for backup)

	baseStack []lexState

	ShortOpenTag bool // when false, <? without php/= is not a PHP open tag
}

func NewLexer(i io.Reader, fn string) *Lexer {
	return NewLexerWithShortTag(i, fn, true)
}

func NewLexerWithShortTag(i io.Reader, fn string, shortOpenTag bool) *Lexer {
	res := &Lexer{
		input:        bufio.NewReader(i),
		fn:           fn, // filename, for position information
		items:        make(chan *Item, 2),
		done:         make(chan struct{}),
		sLine:        1,
		cLine:        1,
		ShortOpenTag: shortOpenTag,
	}

	go res.run(lexText)

	return res
}

func NewLexerPhp(i io.Reader, fn string) *Lexer {
	res := &Lexer{
		input: bufio.NewReader(i),
		fn:    fn, // filename, for position information
		items: make(chan *Item, 2),
		done:  make(chan struct{}),
		sLine: 1,
		cLine: 1,
		base:  lexText, // allow ?> to fall back to HTML mode
	}

	go res.run(lexPhp)

	return res
}

func (l *Lexer) pushCall(s lexState) {
	l.baseStack = append(l.baseStack, l.base)
	l.base = s
	s(l)
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
		return &Item{Type: T_EOF}, nil
	}
	if i.Type == itemError {
		return nil, &phpv.PhpError{
			Err:  fmt.Errorf("%s", i.Data),
			Code: phpv.E_PARSE,
			Loc:  i.Loc(),
		}
	}
	return i, nil
}

func (l *Lexer) hasPrefix(s string) bool {
	v := l.peekString(len(s))
	return string(v) == s
}

func (l *Lexer) hasPrefixI(s string) bool {
	v := l.peekString(len(s))
	return strings.ToLower(string(v)) == strings.ToLower(s)
}

func (l *Lexer) run(state lexState) {
	defer func() {
		if r := recover(); r != nil {
			// Recover from panics in the lexer goroutine and emit an error
			l.error("internal tokenizer error: %v", r)
		}
		close(l.items)
	}()
	l.push(state)
	for state = l.base; state != nil; {
		// Check if we've been told to stop
		select {
		case <-l.done:
			return
		default:
		}
		state = state(l)
	}
}

func (l *Lexer) emit(t ItemType) {
	item := &Item{t, l.output.String(), l.fn, l.sLine, l.sChar}
	select {
	case l.items <- item:
	case <-l.done:
		return
	}
	l.prevItem = item
	l.start = l.pos
	l.sLine, l.sChar = l.cLine, l.cChar
	l.output.Reset()
}

func (l *Lexer) isDone() bool {
	select {
	case <-l.done:
		return true
	default:
		return false
	}
}

func (l *Lexer) next() rune {
	if l.isDone() {
		return eof
	}

	var r rune
	var err error

	if len(l.inputRst) > 0 {
		r, l.width = utf8.DecodeRune(l.inputRst)
		if l.width == len(l.inputRst) {
			l.inputRst = nil
		} else {
			l.inputRst = l.inputRst[l.width:]
		}
	} else {
		r, l.width, err = l.input.ReadRune()
		if err != nil {
			if err == io.EOF {
				return eof
			}
			return eof // TODO FIXME error reporting?
		}
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

func (l *Lexer) reset() {
	tmp := []byte(l.output.String())

	if len(l.inputRst) == 0 {
		l.inputRst = tmp
	} else {
		l.inputRst = append(tmp, l.inputRst...)
	}

	l.output.Reset()
	l.pos -= len(tmp)
	l.cLine, l.cChar = l.sLine, l.sChar
}

func (l *Lexer) backup() {
	if l.width == 0 {
		return
	}

	// update buffers
	tmp := l.output.String()
	r := []byte(tmp[len(tmp)-l.width:]) // removed char
	tmp = tmp[:len(tmp)-l.width]        // remove
	l.output.Reset()
	l.output.WriteString(tmp)

	l.inputRst = append(r, l.inputRst...)

	l.pos -= l.width
	l.cLine, l.cChar = l.pLine, l.pChar
	l.width = 0
}

func (l *Lexer) peek() rune {
	s := []byte(l.peekString(utf8.UTFMax))
	if len(s) == 0 {
		return eof
	}
	r, _ := utf8.DecodeRune(s)
	return r
}

func (l *Lexer) peekString(ln int) string {
	if len(l.inputRst) > 0 {
		if len(l.inputRst) >= ln {
			return string(l.inputRst[:ln])
		}
		res := l.inputRst
		s, _ := l.input.Peek(ln - len(res))
		return string(append(res, s...))
	}
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

func (l *Lexer) acceptFixedI(s string) bool {
	if !l.hasPrefixI(s) {
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
	b := &bytes.Buffer{}
	for {
		v := l.next()
		if v == eof {
			return b.String()
		}
		if strings.IndexRune(valid, v) >= 0 {
			b.WriteRune(v)
			continue
		}
		l.backup()
		return b.String()
	}
}

func (l *Lexer) acceptUntil(s string) {
	for {
		c := l.next()
		if c == eof || strings.IndexRune(s, c) >= 0 {
			break
		}
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
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', c == '_', 0x80 <= c:
	default:
		l.backup()
		// we didn't read a single char
		return ""
	}

	for {
		c := l.next()
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', c == '_', 0x80 <= c:
		default:
			l.backup()
			return l.output.String()[labelStart:]
		}
	}
}

func (l *Lexer) error(format string, args ...interface{}) lexState {
	select {
	case l.items <- &Item{
		itemError,
		fmt.Sprintf(format, args...),
		l.fn,
		l.sLine, l.sChar,
	}:
	case <-l.done:
	}
	return nil
}

// Close signals the lexer goroutine to stop. This should be called
// when compilation is aborted (e.g., on timeout) to prevent goroutine leaks.
func (l *Lexer) Close() {
	select {
	case <-l.done:
		// already closed
	default:
		close(l.done)
	}
}
