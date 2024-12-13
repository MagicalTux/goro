package core

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Ini struct {
	processSections bool // if false, everything will be in section "root"
}

type iniValue struct {
	s, k, v string
}

func NewIni() *Ini {
	return &Ini{}
}

func (i *Ini) Parse(r io.Reader) error {
	buf := bufio.NewReader(r)
	var s string // section
	var lineNo int
	var values []*iniValue

	for {
		lineNo += 1
		l, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		l = strings.TrimSpace(l)
		if l == "" {
			// empty line
			continue
		}
		if l[0] == ';' {
			// comment only line
			continue
		}

		if l[0] == '[' {
			// this is a section identifier

			// check for comments
			pos := strings.IndexByte(l, ';')
			if pos != -1 {
				l = strings.TrimSpace(l[:pos])
			}

			if l[len(l)-1] != ']' {
				// syntax error
				return fmt.Errorf("ini: unable to parse section declaration on line %d", lineNo)
			}

			s = l[1 : len(l)-1]
			continue
		}

		// l should be in the form of var_name=value
		pos := strings.IndexByte(l, '=')
		if pos == -1 {
			// lines without values are considered to be ignored by php
			continue
		}

		k := l[:pos]
		l = l[pos+1:]

		values = append(values, &iniValue{s, k, l})
	}

	return errors.New("todo parse values")
}
