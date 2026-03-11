package pcre

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func mixed preg_replace ( mixed $pattern , mixed $replacement , mixed $subject [, int $limit = -1 [, int &$count ]] )
func pregReplace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pattern, replacement, subject *phpv.ZVal
	var limit *phpv.ZInt
	var count *phpv.ZInt

	_, err := core.Expand(ctx, args, &pattern, &replacement, &subject, &limit, &count)
	if err != nil {
		return nil, err
	}

	if limit == nil {
		limit = new(phpv.ZInt)
		*limit = -1
	}
	if count == nil {
		count = new(phpv.ZInt)
	}

	return doPregReplace(ctx, pattern, replacement, subject, *limit, count)
}

func prepareRegexp(pattern string) (*regexp.Regexp, error) {
	// When using the PCRE functions, it is required that the pattern is enclosed by delimiters.
	if len(pattern) < 2 {
		return nil, errors.New("pattern is too short")
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

	// find next instance of delimiter not prefixed by a '\'
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
		return nil, errors.New("end delimiter missing from pattern")
	}

	flags := pattern[pos+utf8.RuneLen(end_delimiter):]
	pattern = pattern[:pos]

	// Convert PHP regex flags to Go regexp flags
	var goFlags strings.Builder
	goFlags.WriteString("(?")
	hasFlags := false
	for _, f := range flags {
		switch f {
		case 'i': // case insensitive
			goFlags.WriteRune('i')
			hasFlags = true
		case 'm': // multiline
			goFlags.WriteRune('m')
			hasFlags = true
		case 's': // dotall (. matches newline)
			goFlags.WriteRune('s')
			hasFlags = true
		case 'x': // extended (ignore whitespace and comments)
			// Go regexp doesn't support 'x' flag directly, skip for now
		case 'u': // UTF-8 mode (Go regexp handles UTF-8 by default)
		case 'U': // ungreedy (swap greedy/non-greedy)
			goFlags.WriteRune('U')
			hasFlags = true
		case 'A': // anchored
			// Prepend ^ to pattern if not already anchored
			if len(pattern) == 0 || pattern[0] != '^' {
				pattern = "^" + pattern
			}
		case 'D': // dollar end only
			// In Go regexp, $ already only matches end of string by default (without m flag)
		}
	}

	var finalPattern string
	if hasFlags {
		goFlags.WriteRune(')')
		finalPattern = goFlags.String() + pattern
	} else {
		finalPattern = pattern
	}

	re, err := regexp.Compile(finalPattern)
	if err != nil {
		return nil, fmt.Errorf("preg: compilation failed: %s", err)
	}

	return re, nil
}

func doPregReplace(ctx phpv.Context, pattern, replacement, subject *phpv.ZVal, limit phpv.ZInt, count *phpv.ZInt) (*phpv.ZVal, error) {
	pattern, err := pattern.As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	re, err := prepareRegexp(string(pattern.AsString(ctx)))
	if err != nil {
		return nil, err
	}

	repl := []byte(replacement.AsString(ctx))
	in := []byte(subject.AsString(ctx))

	var r []byte
	n := 0
	maxReplacements := int(limit)

	for {
		if maxReplacements >= 0 && n >= maxReplacements {
			break
		}

		loc := re.FindSubmatchIndex(in)
		if loc == nil {
			break
		}

		// Extract submatches for backreference expansion
		var matches [][]byte
		for i := 0; i < len(loc); i += 2 {
			if loc[i] < 0 {
				matches = append(matches, nil)
			} else {
				matches = append(matches, in[loc[i]:loc[i+1]])
			}
		}

		r = append(r, in[:loc[0]]...)
		r = append(r, pcreExpand(matches, repl)...)
		in = in[loc[1]:]
		n++

		// Prevent infinite loop on zero-length matches
		if loc[0] == loc[1] {
			if len(in) == 0 {
				break
			}
			r = append(r, in[0])
			in = in[1:]
		}
	}
	r = append(r, in...)

	*count = phpv.ZInt(n)

	return phpv.ZString(r).ZVal(), nil
}
