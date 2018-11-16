package pcre

import (
	"errors"
	"log"
	"unicode/utf8"

	"github.com/MagicalTux/gophp/core"
	gopcre "github.com/gijsbers/go-pcre"
)

//> func mixed preg_replace ( mixed $pattern , mixed $replacement , mixed $subject [, int $limit = -1 [, int &$count ]] )
func pregReplace(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var pattern, replacement, subject *core.ZVal
	var limit *core.ZInt
	var count *core.ZInt

	_, err := core.Expand(ctx, args, &pattern, &replacement, &subject, &limit, &count)
	if err != nil {
		return nil, err
	}

	if limit == nil {
		limit = new(core.ZInt)
		*limit = -1
	}
	if count == nil {
		count = new(core.ZInt)
	}

	return doPregReplace(ctx, pattern, replacement, subject, *limit, count)
}

func prepareRegexp(pattern string) (gopcre.Regexp, error) {
	// TODO: pattern cache

	// When using the PCRE functions, it is required that the pattern is enclosed by delimiters. A delimiter can be any non-alphanumeric, non-backslash, non-whitespace character.
	if len(pattern) < 2 { // can't be less than delimiter+delimiter
		return gopcre.Regexp{}, errors.New("pattern is too short")
	}

	delimiter, d_len := utf8.DecodeRuneInString(pattern)
	pattern = pattern[d_len:]
	end_delimiter := delimiter

	switch delimiter {
	case '(':
		end_delimiter = ')'
	case '{':
		end_delimiter = '}'
	case '[':
		end_delimiter = ']'
	case '<':
		end_delimiter = '>'
	}
	_ = delimiter

	// find next instance of delimiter not prefixed by a \
	var skip, found bool
	var stack, pos int
	for i, c := range pattern {
		if skip {
			skip = false
			continue
		}

		switch c {
		case '\\':
			skip = true
		case delimiter:
			if delimiter != end_delimiter {
				// brackets
				stack += 1
				break
			}
			fallthrough
		case end_delimiter:
			if stack > 0 {
				stack -= 1
			} else {
				found = true
				pos = i
			}
		}
		if found {
			break
		}
	}
	if !found {
		return gopcre.Regexp{}, errors.New("end delimiter missing from pattern")
	}
	mod := pattern[pos+d_len:]
	pattern = pattern[:pos]

	log.Printf("PCRE: pattern=%s modifier=%s", pattern, mod)

	return gopcre.Compile(pattern, 0) // TODO
}

func doPregReplace(ctx core.Context, pattern, replacement, subject *core.ZVal, limit core.ZInt, count *core.ZInt) (*core.ZVal, error) {
	pattern, err := pattern.As(ctx, core.ZtString)
	if err != nil {
		return nil, err
	}

	regexp, err := prepareRegexp(string(pattern.AsString(ctx)))
	if err != nil {
		return nil, err
	}

	repl := []byte(replacement.AsString(ctx))

	in := []byte(subject.AsString(ctx))
	m := regexp.Matcher(in, 0) // TODO flags
	var r []byte

	for m.Matches() {
		loc := m.Index()
		r = append(r, in[:loc[0]]...)
		r = append(r, pcreExpand(m.Extract(), repl)...) // TODO expand repl
		in = in[loc[1]:]
		m.Match(in, 0) // TODO flags
	}
	r = append(r, in...)

	// check repl for backreferences (\1 or $1 type of thing)

	return core.ZString(r).ZVal(), nil
}
