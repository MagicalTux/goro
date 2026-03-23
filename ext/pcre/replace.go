package pcre

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func mixed preg_replace ( mixed $pattern , mixed $replacement , mixed $subject [, int $limit = -1 [, int &$count ]] )
func pregReplace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pattern, replacement, subject *phpv.ZVal
	var limit *phpv.ZInt
	var countRef core.OptionalRef[phpv.ZInt]

	_, err := core.Expand(ctx, args, &pattern, &replacement, &subject, &limit, &countRef)
	if err != nil {
		return nil, err
	}

	limitVal := phpv.ZInt(-1)
	if limit != nil {
		limitVal = *limit
	}

	// Type checking: if replacement is array, pattern must also be array
	if replacement != nil && replacement.GetType() == phpv.ZtArray {
		if pattern.GetType() != phpv.ZtArray {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "preg_replace(): Argument #1 ($pattern) must be of type array when argument #2 ($replacement) is an array, string given")
		}
	}

	// Check for invalid types
	if replacement != nil && replacement.GetType() == phpv.ZtObject {
		className := replacement.Value().(phpv.ZObject).GetClass().GetName()
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("preg_replace(): Argument #2 ($replacement) must be of type array|string, %s given", className))
	}
	if pattern != nil && pattern.GetType() == phpv.ZtObject {
		className := pattern.Value().(phpv.ZObject).GetClass().GetName()
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("preg_replace(): Argument #1 ($pattern) must be of type array|string, %s given", className))
	}

	count := new(phpv.ZInt)

	// Handle array patterns
	var result *phpv.ZVal
	if pattern.GetType() == phpv.ZtArray {
		result, err = doPregReplaceArrayPattern(ctx, pattern, replacement, subject, limitVal, count)
	} else if subject.GetType() == phpv.ZtArray {
		result, err = doPregReplaceArraySubject(ctx, pattern, replacement, subject, limitVal, count)
	} else {
		result, err = doPregReplace(ctx, pattern, replacement, subject, limitVal, count)
	}

	if err != nil {
		return nil, err
	}

	if countRef.HasArg() {
		countRef.Set(ctx, *count)
	}

	return result, nil
}

func doPregReplaceArrayPattern(ctx phpv.Context, pattern, replacement, subject *phpv.ZVal, limit phpv.ZInt, count *phpv.ZInt) (*phpv.ZVal, error) {
	patternArr := pattern.Value().(*phpv.ZArray)
	var replArr *phpv.ZArray
	if replacement != nil && replacement.GetType() == phpv.ZtArray {
		replArr = replacement.Value().(*phpv.ZArray)
	}

	// Handle array subject
	if subject.GetType() == phpv.ZtArray {
		subjectArr := subject.Value().(*phpv.ZArray)
		result := phpv.NewZArray()
		totalCount := phpv.ZInt(0)
		for k, v := range subjectArr.Iterate(ctx) {
			current := v
			elemCount := phpv.ZInt(0)
			idx := 0
			for _, pv := range patternArr.Iterate(ctx) {
				var replVal *phpv.ZVal
				if replArr != nil {
					rv, err := replArr.OffsetGet(ctx, phpv.ZInt(idx).ZVal())
					if err == nil && rv != nil && rv.GetType() != phpv.ZtNull {
						replVal = rv
					} else {
						replVal = phpv.ZString("").ZVal()
					}
				} else if replacement != nil {
					replVal = replacement
				} else {
					replVal = phpv.ZString("").ZVal()
				}
				c := phpv.ZInt(0)
				var err error
				current, err = doPregReplace(ctx, pv, replVal, current, limit, &c)
				if err != nil {
					return nil, err
				}
				if current == nil {
					// Error occurred (warning already emitted)
					current = phpv.ZNULL.ZVal()
					break
				}
				elemCount += c
				idx++
			}
			totalCount += elemCount
			result.OffsetSet(ctx, k, current)
		}
		*count = totalCount
		return result.ZVal(), nil
	}

	// Single subject, array pattern
	current := subject
	totalCount := phpv.ZInt(0)
	idx := 0
	for _, pv := range patternArr.Iterate(ctx) {
		var replVal *phpv.ZVal
		if replArr != nil {
			rv, err := replArr.OffsetGet(ctx, phpv.ZInt(idx).ZVal())
			if err == nil && rv != nil && rv.GetType() != phpv.ZtNull {
				replVal = rv
			} else {
				replVal = phpv.ZString("").ZVal()
			}
		} else if replacement != nil {
			replVal = replacement
		} else {
			replVal = phpv.ZString("").ZVal()
		}
		c := phpv.ZInt(0)
		var err error
		current, err = doPregReplace(ctx, pv, replVal, current, limit, &c)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return phpv.ZNULL.ZVal(), nil
		}
		totalCount += c
		idx++
	}
	*count = totalCount
	return current, nil
}

func doPregReplaceArraySubject(ctx phpv.Context, pattern, replacement, subject *phpv.ZVal, limit phpv.ZInt, count *phpv.ZInt) (*phpv.ZVal, error) {
	subjectArr := subject.Value().(*phpv.ZArray)
	result := phpv.NewZArray()
	totalCount := phpv.ZInt(0)

	for k, v := range subjectArr.Iterate(ctx) {
		c := phpv.ZInt(0)
		replaced, err := doPregReplace(ctx, pattern, replacement, v, limit, &c)
		if err != nil {
			return nil, err
		}
		totalCount += c
		result.OffsetSet(ctx, k, replaced)
	}
	*count = totalCount
	return result.ZVal(), nil
}

// prepareRegexp parses a PHP-style regex pattern with delimiters and flags,
// and returns a compiled Go regexp. On error, it returns a *pcreError that
// should be turned into a PHP warning (not a fatal error).
func prepareRegexp(pattern string) (*regexp.Regexp, *pcreError) {
	// Check empty pattern
	if len(pattern) == 0 {
		return nil, &pcreError{kind: pcreErrEmpty}
	}

	// Trim leading whitespace for delimiter detection (PHP does this)
	trimmed := strings.TrimLeftFunc(pattern, unicode.IsSpace)
	if len(trimmed) == 0 {
		return nil, &pcreError{kind: pcreErrEmpty}
	}

	delimiter, d_len := utf8.DecodeRuneInString(trimmed)

	// Check for invalid delimiters
	if delimiter == 0 || delimiter == '\\' || unicode.IsLetter(delimiter) || unicode.IsDigit(delimiter) {
		return nil, &pcreError{kind: pcreErrAlphanumeric}
	}

	rest := trimmed[d_len:]
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

	// find next instance of end_delimiter not prefixed by a '\'
	var skip, found bool
	var stack, pos int
	for i, c := range rest {
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
		if delimiter == end_delimiter {
			return nil, &pcreError{kind: pcreErrNoEndDelim, delimiter: delimiter}
		}
		return nil, &pcreError{kind: pcreErrNoEndDelimMatch, delimiter: end_delimiter}
	}

	flags := rest[pos+utf8.RuneLen(end_delimiter):]
	regexBody := rest[:pos]

	// Check for NUL in flags
	for _, f := range flags {
		if f == 0 {
			return nil, &pcreError{kind: pcreErrNulModifier}
		}
	}

	// Strip \r from flags (PHP ignores \r in modifier section)
	flags = strings.ReplaceAll(flags, "\r", "")

	// Convert PHP regex syntax to Go
	// 1. Convert (?'name'...) to (?P<name>...)
	regexBody = convertNamedCaptures(regexBody)

	// Convert PHP regex flags to Go regexp flags
	var goFlags strings.Builder
	goFlags.WriteString("(?")
	hasFlags := false
	useExtended := false
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
			useExtended = true
		case 'u': // UTF-8 mode (Go regexp handles UTF-8 by default)
		case 'U': // ungreedy (swap greedy/non-greedy)
			goFlags.WriteRune('U')
			hasFlags = true
		case 'A': // anchored
			// Prepend ^ to pattern if not already anchored
			if len(regexBody) == 0 || regexBody[0] != '^' {
				regexBody = "^" + regexBody
			}
		case 'D': // dollar end only
			// In Go regexp, $ already only matches end of string by default (without m flag)
			// But if 'm' flag is present, we need to handle this differently.
			// For now, $ in Go matches end of text by default, so D is effectively the default.
		case 'S': // study - optimization hint, ignored
		case 'X': // extra - PCRE_EXTRA, mostly ignored in modern PHP
		case 'J': // allow duplicate named groups - not supported in Go
			// Just ignore for now
		case 'n': // non-capture modifier - convert unnamed groups to non-capturing
			regexBody = convertUnnamedToNonCapture(regexBody)
		case ' ', '\t', '\n': // whitespace in modifiers is ignored by PHP
			// Ignore whitespace in modifier section
		default:
			return nil, &pcreError{kind: pcreErrUnknownModifier, modifier: f}
		}
	}

	if useExtended {
		regexBody = stripExtendedWhitespace(regexBody)
	}

	var finalPattern string
	if hasFlags {
		goFlags.WriteRune(')')
		finalPattern = goFlags.String() + regexBody
	} else {
		finalPattern = regexBody
	}

	// Validate named groups: PCRE requires names to start with a non-digit
	if err := validateNamedGroups(finalPattern); err != nil {
		return nil, err
	}

	re, err := regexp.Compile(finalPattern)
	if err != nil {
		return nil, &pcreError{kind: pcreErrCompile, compileErr: err}
	}

	return re, nil
}

// validateNamedGroups checks for PCRE-invalid named group names (e.g., starting with a digit).
func validateNamedGroups(pattern string) *pcreError {
	i := 0
	for i < len(pattern) {
		// Skip escaped characters
		if pattern[i] == '\\' && i+1 < len(pattern) {
			i += 2
			continue
		}
		// Skip character classes
		if pattern[i] == '[' {
			i++
			for i < len(pattern) && pattern[i] != ']' {
				if pattern[i] == '\\' && i+1 < len(pattern) {
					i += 2
				} else {
					i++
				}
			}
			if i < len(pattern) {
				i++
			}
			continue
		}
		// Check for named groups: (?P<name>), (?<name>), (?'name')
		if i+3 < len(pattern) && pattern[i] == '(' && pattern[i+1] == '?' {
			nameStart := -1
			if pattern[i+2] == 'P' && i+4 < len(pattern) && pattern[i+3] == '<' {
				nameStart = i + 4
			} else if pattern[i+2] == '<' && pattern[i+3] != '=' && pattern[i+3] != '!' {
				nameStart = i + 3
			}
			if nameStart >= 0 && nameStart < len(pattern) {
				ch := pattern[nameStart]
				if ch >= '0' && ch <= '9' {
					return &pcreError{kind: pcreErrCompile, compileErr: fmt.Errorf("subpattern name must start with a non-digit at offset %d", nameStart-1)}
				}
			}
		}
		i++
	}
	return nil
}

// convertNamedCaptures converts PHP named capture syntaxes to Go-compatible format.
// PHP supports: (?P<name>...), (?<name>...), (?'name'...)
// Go supports: (?P<name>...) and (?<name>...)
// So we only need to convert (?'name'...) → (?P<name>...)
func convertNamedCaptures(pattern string) string {
	// Look for (?'name' and convert to (?P<name>
	var result strings.Builder
	i := 0
	for i < len(pattern) {
		if i+2 < len(pattern) && pattern[i] == '(' && pattern[i+1] == '?' && pattern[i+2] == '\'' {
			// Find the closing '
			end := strings.IndexByte(pattern[i+3:], '\'')
			if end >= 0 {
				name := pattern[i+3 : i+3+end]
				result.WriteString("(?P<")
				result.WriteString(name)
				result.WriteString(">")
				i = i + 3 + end + 1
				continue
			}
		}
		// Handle backslash escapes to avoid false matches inside escaped sequences
		if pattern[i] == '\\' && i+1 < len(pattern) {
			result.WriteByte(pattern[i])
			result.WriteByte(pattern[i+1])
			i += 2
			continue
		}
		result.WriteByte(pattern[i])
		i++
	}
	return result.String()
}

// stripExtendedWhitespace removes unescaped whitespace and #-comments from a pattern (PCRE_EXTENDED).
// Whitespace inside character classes [...] is preserved.
func stripExtendedWhitespace(pattern string) string {
	var result strings.Builder
	inCharClass := false
	i := 0

	for i < len(pattern) {
		ch := pattern[i]

		// Handle escape sequences
		if ch == '\\' && i+1 < len(pattern) {
			result.WriteByte(ch)
			result.WriteByte(pattern[i+1])
			i += 2
			continue
		}

		// Track character classes
		if ch == '[' && !inCharClass {
			inCharClass = true
			result.WriteByte(ch)
			i++
			continue
		}
		if ch == ']' && inCharClass {
			inCharClass = false
			result.WriteByte(ch)
			i++
			continue
		}

		if !inCharClass {
			// Skip whitespace
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\f' || ch == '\v' {
				i++
				continue
			}
			// Skip comments (# to end of line)
			if ch == '#' {
				for i < len(pattern) && pattern[i] != '\n' {
					i++
				}
				continue
			}
		}

		result.WriteByte(ch)
		i++
	}

	return result.String()
}

// convertUnnamedToNonCapture converts unnamed capture groups (...) to non-capture
// groups (?:...) for the PCRE /n modifier. Named groups (?P<name>...), (?<name>...),
// and (?'name'...) are left intact. Other special groups (?:...), (?=...), (?!...), etc.
// are also left intact.
func convertUnnamedToNonCapture(pattern string) string {
	var result strings.Builder
	i := 0
	for i < len(pattern) {
		// Handle backslash escapes
		if pattern[i] == '\\' && i+1 < len(pattern) {
			result.WriteByte(pattern[i])
			result.WriteByte(pattern[i+1])
			i += 2
			continue
		}
		// Skip character classes [...]
		if pattern[i] == '[' {
			result.WriteByte(pattern[i])
			i++
			// Handle negation
			if i < len(pattern) && pattern[i] == '^' {
				result.WriteByte(pattern[i])
				i++
			}
			// Handle ] as first char in class
			if i < len(pattern) && pattern[i] == ']' {
				result.WriteByte(pattern[i])
				i++
			}
			for i < len(pattern) && pattern[i] != ']' {
				if pattern[i] == '\\' && i+1 < len(pattern) {
					result.WriteByte(pattern[i])
					result.WriteByte(pattern[i+1])
					i += 2
				} else {
					result.WriteByte(pattern[i])
					i++
				}
			}
			if i < len(pattern) {
				result.WriteByte(pattern[i])
				i++
			}
			continue
		}
		if pattern[i] == '(' {
			if i+1 < len(pattern) && pattern[i+1] == '?' {
				// This is a special group (?...) - leave it as-is
				result.WriteByte(pattern[i])
				i++
			} else {
				// This is an unnamed capture group - convert to non-capturing
				result.WriteString("(?:")
				i++
			}
			continue
		}
		result.WriteByte(pattern[i])
		i++
	}
	return result.String()
}

func doPregReplace(ctx phpv.Context, pattern, replacement, subject *phpv.ZVal, limit phpv.ZInt, count *phpv.ZInt) (*phpv.ZVal, error) {
	patternStr, err := pattern.As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	re, pcreErr := prepareRegexp(string(patternStr.AsString(ctx)))
	if pcreErr != nil {
		ctx.Warn("%s", pcreErr.Warning("preg_replace"))
		return phpv.ZNULL.ZVal(), nil
	}

	repl := []byte(replacement.AsString(ctx))
	in := []byte(subject.AsString(ctx))

	maxReplacements := int(limit)
	if maxReplacements < 0 {
		maxReplacements = -1
	}

	// Find all matches at once on the original string to preserve anchor semantics
	allLocs := re.FindAllSubmatchIndex(in, maxReplacements)
	if allLocs == nil {
		*count = 0
		return phpv.ZString(in).ZVal(), nil
	}

	var r []byte
	pos := 0
	for _, loc := range allLocs {
		// Extract submatches for backreference expansion
		var matches [][]byte
		for i := 0; i < len(loc); i += 2 {
			if loc[i] < 0 {
				matches = append(matches, nil)
			} else {
				matches = append(matches, in[loc[i]:loc[i+1]])
			}
		}

		r = append(r, in[pos:loc[0]]...)
		r = append(r, pcreExpand(matches, repl)...)
		pos = loc[1]

		// For zero-length matches, advance by one rune to avoid infinite loop
		if loc[0] == loc[1] && pos < len(in) {
			_, size := utf8.DecodeRune(in[pos:])
			if size == 0 {
				size = 1
			}
			r = append(r, in[pos:pos+size]...)
			pos += size
		}
	}
	r = append(r, in[pos:]...)

	*count = phpv.ZInt(len(allLocs))

	return phpv.ZString(r).ZVal(), nil
}
