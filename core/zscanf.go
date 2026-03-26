package core

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type errBadScanChar struct {
	Code byte
}

func (err *errBadScanChar) Error() string {
	if err.Code == 0 {
		// End of format string after % → PHP outputs just an opening quote
		return `Bad scan conversion character "`
	}
	return fmt.Sprintf(`Bad scan conversion character "%c"`, err.Code)
}

type errArgIndexOutOfRange struct{}

func (err *errArgIndexOutOfRange) Error() string {
	return `"%n$" argument index out of range`
}

func skipWhitespaces(r *bufio.Reader) error {
	_, err := skipWhitespacesTracked(r)
	return err
}

func skipWhitespacesTracked(r *bufio.Reader) (int, error) {
	count := 0
	for {
		c, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				return count, nil
			}
			return count, err
		}
		switch c {
		case ' ', '\n', '\t', '\r', '\f', '\v':
			count++
		default:
			r.UnreadByte()
			return count, nil
		}
	}
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

func countRemainingScanCodes(format phpv.ZString, startIndex int) int {
	count := 0
	for i := startIndex; i < len(format); i++ {
		if format[i] == '%' {
			i++
			if i >= len(format) {
				break
			}
			if format[i] == '%' {
				continue // literal %
			}
			// skip position specifier like 1$
			j := i
			for j < len(format) && format[j] >= '0' && format[j] <= '9' {
				j++
			}
			if j < len(format) && j > i && format[j] == '$' {
				i = j + 1
				if i >= len(format) {
					break
				}
			}
			// check suppression (comes before width)
			suppressed := false
			if format[i] == '*' {
				suppressed = true
				i++
				if i >= len(format) {
					break
				}
			}
			// skip width
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				i++
			}
			if i >= len(format) {
				break
			}
			// skip length modifiers (h, l, L, hh, ll)
			if i < len(format) && (format[i] == 'h' || format[i] == 'l' || format[i] == 'L') {
				i++
				if i < len(format) && (format[i] == 'h' || format[i] == 'l') {
					i++
				}
			}
			if i >= len(format) {
				break
			}
			c := format[i]
			if c == '[' {
				// skip past character class
				i++
				if i < len(format) && format[i] == '^' {
					i++
				}
				if i < len(format) && format[i] == ']' {
					i++
				}
				for i < len(format) && format[i] != ']' {
					i++
				}
			}
			if !suppressed {
				count++
			}
		}
	}
	return count
}

// scanReadInt reads an integer from buf with optional width limit
func scanReadInt(buf *bufio.Reader, base int, width int) (int64, bool) {
	var s []byte
	// Read optional sign
	c, err := buf.ReadByte()
	if err != nil {
		return 0, false
	}
	if c == '-' || c == '+' {
		s = append(s, c)
		width--
	} else {
		buf.UnreadByte()
	}

	if width == 0 {
		return 0, false
	}

	// For hex, skip optional 0x prefix
	if base == 16 {
		if peeked, err := buf.Peek(2); err == nil && len(peeked) == 2 {
			if peeked[0] == '0' && (peeked[1] == 'x' || peeked[1] == 'X') {
				buf.ReadByte()
				buf.ReadByte()
				width -= 2
			}
		}
	}

	// For octal, skip optional 0 prefix
	if base == 8 {
		c, err := buf.ReadByte()
		if err == nil {
			if c == '0' {
				width--
				// just consume it
			} else {
				buf.UnreadByte()
			}
		}
	}

	gotDigit := false
	for width != 0 {
		c, err := buf.ReadByte()
		if err != nil {
			break
		}
		valid := false
		switch base {
		case 2:
			valid = c == '0' || c == '1'
		case 8:
			valid = c >= '0' && c <= '7'
		case 10:
			valid = c >= '0' && c <= '9'
		case 16:
			valid = (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		}
		if !valid {
			buf.UnreadByte()
			break
		}
		s = append(s, c)
		gotDigit = true
		width--
	}

	if !gotDigit {
		return 0, false
	}

	n, err2 := strconv.ParseInt(string(s), base, 64)
	if err2 != nil {
		// Try unsigned
		un, err3 := strconv.ParseUint(string(s), base, 64)
		if err3 != nil {
			return 0, false
		}
		return int64(un), true
	}
	return n, true
}

// scanReadFloat reads a float from buf with optional width limit
func scanReadFloat(buf *bufio.Reader, width int) (float64, bool) {
	var s []byte

	// Read optional sign
	c, err := buf.ReadByte()
	if err != nil {
		return 0, false
	}
	if c == '-' || c == '+' {
		s = append(s, c)
		width--
	} else {
		buf.UnreadByte()
	}

	gotDigit := false
	gotDot := false
	gotE := false

	for width != 0 {
		c, err := buf.ReadByte()
		if err != nil {
			break
		}
		if c >= '0' && c <= '9' {
			s = append(s, c)
			gotDigit = true
			width--
		} else if c == '.' && !gotDot && !gotE {
			s = append(s, c)
			gotDot = true
			width--
		} else if (c == 'e' || c == 'E') && !gotE && gotDigit {
			s = append(s, c)
			gotE = true
			width--
			// Read optional sign after e
			if width != 0 {
				c2, err2 := buf.ReadByte()
				if err2 == nil {
					if c2 == '+' || c2 == '-' {
						s = append(s, c2)
						width--
					} else {
						buf.UnreadByte()
					}
				}
			}
		} else {
			buf.UnreadByte()
			break
		}
	}

	if !gotDigit {
		return 0, false
	}

	f, err2 := strconv.ParseFloat(string(s), 64)
	if err2 != nil {
		// If the string ends with an incomplete exponent, strip and retry
		stripped := string(s)
		stripCount := 0
		if len(stripped) > 0 && (stripped[len(stripped)-1] == '+' || stripped[len(stripped)-1] == '-') {
			stripped = stripped[:len(stripped)-1]
			stripCount++
		}
		if len(stripped) > 0 && (stripped[len(stripped)-1] == 'e' || stripped[len(stripped)-1] == 'E') {
			stripped = stripped[:len(stripped)-1]
			stripCount++
		}
		if stripCount > 0 && len(stripped) > 0 {
			f2, err3 := strconv.ParseFloat(stripped, 64)
			if err3 == nil {
				for i := 0; i < stripCount; i++ {
					buf.UnreadByte()
				}
				return f2, true
			}
		}
		return 0, false
	}
	return f, true
}

// scanReadString reads a non-whitespace string with optional width limit
func scanReadString(buf *bufio.Reader, width int) (string, bool) {
	var s []byte
	for width != 0 {
		c, err := buf.ReadByte()
		if err != nil {
			break
		}
		if isWhitespace(c) {
			buf.UnreadByte()
			break
		}
		s = append(s, c)
		width--
	}
	if len(s) == 0 {
		return "", false
	}
	return string(s), true
}

// inputBytesConsumed is a counting reader wrapper to track bytes read
type countingReader struct {
	r     *bufio.Reader
	count int
}

func (cr *countingReader) ReadByte() (byte, error) {
	b, err := cr.r.ReadByte()
	if err == nil {
		cr.count++
	}
	return b, err
}

func (cr *countingReader) UnreadByte() error {
	err := cr.r.UnreadByte()
	if err == nil {
		cr.count--
	}
	return err
}

// validateScanFormat checks the format string for invalid conversion characters.
// PHP validates the entire format string before scanning and throws an error
// for any invalid specifier, even if earlier specifiers would fail first.
func validateScanFormat(format phpv.ZString) error {
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		i++
		if i >= len(format) {
			// Trailing "%" with nothing after → bad scan char (empty)
			return &errBadScanChar{0}
		}
		if format[i] == '%' {
			continue // literal %%
		}
		// skip position specifier like 1$
		j := i
		for j < len(format) && format[j] >= '0' && format[j] <= '9' {
			j++
		}
		if j > i && j < len(format) && format[j] == '$' {
			i = j + 1
			if i >= len(format) {
				return &errBadScanChar{0}
			}
		}
		// skip suppression
		if i < len(format) && format[i] == '*' {
			i++
			if i >= len(format) {
				return &errBadScanChar{0}
			}
		}
		// skip width
		for i < len(format) && format[i] >= '0' && format[i] <= '9' {
			i++
		}
		if i >= len(format) {
			return &errBadScanChar{0}
		}
		// skip length modifiers (h, l, L, hh, ll)
		if i < len(format) && (format[i] == 'h' || format[i] == 'l' || format[i] == 'L') {
			i++
			if i < len(format) && (format[i] == 'h' || format[i] == 'l') {
				i++
			}
		}
		if i >= len(format) {
			// Format string ended after length modifier (e.g., "%h") → bad scan char
			return &errBadScanChar{0}
		}
		c := format[i]
		switch c {
		case 'd', 'i', 'o', 'x', 'X', 'u', 'f', 'e', 'E', 'g', 's', 'c', 'n', '[':
			// Valid format specifiers
			if c == '[' {
				// skip past character class
				i++
				if i < len(format) && format[i] == '^' {
					i++
				}
				if i < len(format) && format[i] == ']' {
					i++
				}
				for i < len(format) && format[i] != ']' {
					i++
				}
			}
		default:
			return &errBadScanChar{c}
		}
	}
	return nil
}

// zscanRead returns: values, totalSpecifierCount, inputWasEmpty, scanFailed, error
func zscanRead(r io.Reader, format phpv.ZString) ([]*phpv.ZVal, int, bool, bool, error) {
	buf := bufio.NewReader(r)
	inputConsumed := 0
	failed := false
	result := []*phpv.ZVal{}

	// Check if input is truly empty (zero bytes).
	// PHP returns NULL when the input string is empty.
	inputEmpty := false
	if _, err := buf.Peek(1); err == io.EOF {
		inputEmpty = true
	}
	var pos int

Loop:
	for pos = 0; pos < len(format); pos++ {
		c := format[pos]

		switch c {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			// Whitespace in format: skip whitespace in input
			n, err := skipWhitespacesTracked(buf)
			if err != nil {
				return nil, 0, inputEmpty, false, err
			}
			inputConsumed += n
			continue

		case '%':
			// proceed below

		default:
			// Literal character: must match in input
			c2, err := buf.ReadByte()
			if err != nil {
				if err == io.EOF {
					break Loop
				}
				return nil, 0, inputEmpty, false, err
			}
			inputConsumed++
			if c != c2 {
				break Loop
			}
			continue
		}

		pos++
		if pos >= len(format) {
			break
		}

		// Check for literal %%
		if format[pos] == '%' {
			c2, err := buf.ReadByte()
			if err != nil {
				break Loop
			}
			inputConsumed++
			if c2 != '%' {
				break Loop
			}
			continue
		}

		// Parse position specifier like %1$s
		posSpec := -1
		{
			j := pos
			for j < len(format) && format[j] >= '0' && format[j] <= '9' {
				j++
			}
			if j > pos && j < len(format) && format[j] == '$' {
				n, _ := strconv.Atoi(string(format[pos:j]))
				if n > 100000 || n < 0 {
					// Argument index out of range - use errBadScanChar to trigger ValueError in caller
					return nil, 0, inputEmpty, false, &errArgIndexOutOfRange{}
				}
				posSpec = n
				pos = j + 1
				if pos >= len(format) {
					break
				}
			}
		}

		// Parse suppression flag
		suppress := false
		if pos < len(format) && format[pos] == '*' {
			suppress = true
			pos++
			if pos >= len(format) {
				break
			}
		}

		// Parse width
		width := -1 // -1 means unlimited
		{
			j := pos
			for j < len(format) && format[j] >= '0' && format[j] <= '9' {
				j++
			}
			if j > pos {
				w, _ := strconv.Atoi(string(format[pos:j]))
				width = w
				pos = j
			}
		}

		if pos >= len(format) {
			break
		}

		// Skip length modifiers (h, l, L, ll, hh) - they're no-ops in PHP
		if pos < len(format) && (format[pos] == 'h' || format[pos] == 'l' || format[pos] == 'L') {
			pos++
			// handle ll, hh
			if pos < len(format) && (format[pos] == 'h' || format[pos] == 'l') {
				pos++
			}
		}

		if pos >= len(format) {
			break
		}

		fChar := format[pos]

		var val *phpv.ZVal

		switch fChar {
		case 'n':
			// %n: number of characters consumed from input so far
			// Count actual input consumed
			consumed := inputConsumed
			// Also count what's buffered
			val = phpv.ZInt(consumed).ZVal()

		case 'c':
			// %c: In PHP's scanf, %c reads non-whitespace characters.
			// Without width: reads 1 non-whitespace char; returns "" if next char is whitespace.
			// With width N: reads up to N non-whitespace chars (stopping at whitespace).
			// If input is fully exhausted (EOF), %c fails like other specifiers.
			// But if there's still data (even whitespace), %c "succeeds" with empty string.
			count := 1
			if width > 0 {
				count = width
			}
			// Check if there's any input remaining (even whitespace)
			if _, peekErr := buf.Peek(1); peekErr != nil {
				// Input exhausted → fail
				failed = true
				break Loop
			}
			var s []byte
			for i := 0; i < count; i++ {
				b, err := buf.ReadByte()
				if err != nil {
					break
				}
				if isWhitespace(b) {
					buf.UnreadByte()
					break
				}
				s = append(s, b)
				inputConsumed++
			}
			val = phpv.ZStr(string(s))

		case 's':
			// %s: read non-whitespace string
			// Skip leading whitespace first
			wn, err := skipWhitespacesTracked(buf)
			if err != nil {
				return nil, 0, inputEmpty, false, err
			}
			inputConsumed += wn
			w := width
			var s []byte
			for w != 0 {
				b, err := buf.ReadByte()
				if err != nil {
					break
				}
				if isWhitespace(b) {
					buf.UnreadByte()
					break
				}
				s = append(s, b)
				inputConsumed++
				w--
			}
			if len(s) == 0 {
				failed = true
				break Loop
			}
			val = phpv.ZStr(string(s))

		case 'd':
			// %d: decimal integer
			wn, err := skipWhitespacesTracked(buf)
			if err != nil {
				return nil, 0, inputEmpty, false, err
			}
			inputConsumed += wn
			n, ok := scanReadIntTracked(buf, 10, width, &inputConsumed)
			if !ok {
				failed = true
				break Loop
			}
			val = phpv.ZInt(n).ZVal()

		case 'i':
			// %i: integer with auto-detected base
			wn, err := skipWhitespacesTracked(buf)
			if err != nil {
				return nil, 0, inputEmpty, false, err
			}
			inputConsumed += wn
			// Peek to detect base
			base := 10
			c1, err1 := buf.ReadByte()
			if err1 != nil {
				failed = true
				break Loop
			}
			if c1 == '0' {
				c2, err2 := buf.ReadByte()
				if err2 == nil {
					if c2 == 'x' || c2 == 'X' {
						base = 16
						buf.UnreadByte()
						buf.UnreadByte()
					} else if c2 >= '0' && c2 <= '7' {
						base = 8
						buf.UnreadByte()
						buf.UnreadByte()
					} else {
						buf.UnreadByte()
						buf.UnreadByte()
					}
				} else {
					buf.UnreadByte()
				}
			} else {
				buf.UnreadByte()
			}
			n, ok := scanReadIntTracked(buf, base, width, &inputConsumed)
			if !ok {
				failed = true
				break Loop
			}
			val = phpv.ZInt(n).ZVal()

		case 'o':
			// %o: octal integer
			wn, err := skipWhitespacesTracked(buf)
			if err != nil {
				return nil, 0, inputEmpty, false, err
			}
			inputConsumed += wn
			n, ok := scanReadIntTracked(buf, 8, width, &inputConsumed)
			if !ok {
				failed = true
				break Loop
			}
			val = phpv.ZInt(n).ZVal()

		case 'x', 'X':
			// %x: hexadecimal integer
			wn, err := skipWhitespacesTracked(buf)
			if err != nil {
				return nil, 0, inputEmpty, false, err
			}
			inputConsumed += wn
			n, ok := scanReadIntTracked(buf, 16, width, &inputConsumed)
			if !ok {
				failed = true
				break Loop
			}
			val = phpv.ZInt(n).ZVal()

		case 'u':
			// %u: unsigned decimal integer
			wn, err := skipWhitespacesTracked(buf)
			if err != nil {
				return nil, 0, inputEmpty, false, err
			}
			inputConsumed += wn
			n, ok := scanReadUintTracked(buf, width, &inputConsumed)
			if !ok {
				failed = true
				break Loop
			}
			// If it fits in int64 (non-negative), return as int
			if n <= 9223372036854775807 {
				val = phpv.ZInt(int64(n)).ZVal()
			} else {
				// Return as string for values > max int64
				val = phpv.ZStr(strconv.FormatUint(n, 10))
			}

		case 'f', 'e', 'E', 'g':
			// %f/%e/%E/%g: float
			wn, err := skipWhitespacesTracked(buf)
			if err != nil {
				return nil, 0, inputEmpty, false, err
			}
			inputConsumed += wn
			f, ok := scanReadFloatTracked(buf, width, &inputConsumed)
			if !ok {
				failed = true
				break Loop
			}
			val = phpv.ZFloat(f).ZVal()

		case '[':
			// %[...]: character class
			pos++ // skip '['
			negate := false
			if pos < len(format) && format[pos] == '^' {
				negate = true
				pos++
			}
			// Build set of characters
			var charSet []byte
			// First character can be ']' and is literal
			if pos < len(format) && format[pos] == ']' {
				charSet = append(charSet, ']')
				pos++
			}
			for pos < len(format) && format[pos] != ']' {
				if pos+2 < len(format) && format[pos+1] == '-' && format[pos+2] != ']' {
					// range like a-z
					for c := format[pos]; c <= format[pos+2]; c++ {
						charSet = append(charSet, c)
					}
					pos += 3
				} else {
					charSet = append(charSet, format[pos])
					pos++
				}
			}
			// pos now points at ']' (or past end)

			w := width
			var s []byte
			for w != 0 {
				b, err := buf.ReadByte()
				if err != nil {
					break
				}
				inSet := false
				for _, sc := range charSet {
					if b == sc {
						inSet = true
						break
					}
				}
				if negate {
					inSet = !inSet
				}
				if !inSet {
					buf.UnreadByte()
					break
				}
				s = append(s, b)
				inputConsumed++
				w--
			}
			if len(s) == 0 {
				// %[...] always "succeeds" — on empty match, the value is NULL
				// (unlike %d/%f/%s which cause the entire result to be NULL)
				val = phpv.ZNULL.ZVal()
			} else {
				val = phpv.ZStr(string(s))
			}

		default:
			return nil, 0, inputEmpty, false, &errBadScanChar{fChar}
		}

		if suppress {
			// Don't store the value
			continue
		}

		if posSpec > 0 {
			// Position specifier: store at specific index
			for len(result) < posSpec {
				result = append(result, nil)
			}
			result[posSpec-1] = val
		} else {
			result = append(result, val)
		}
	}

	// Count total expected fields (for null-filling)
	totalFields := countRemainingScanCodes(format, 0)
	if failed && totalFields > len(result) {
		// Fill remaining with nil
	}

	return result, totalFields, inputEmpty, failed, nil
}

// scanReadIntTracked reads an int and tracks inputConsumed
func scanReadIntTracked(buf *bufio.Reader, base int, width int, consumed *int) (int64, bool) {
	var s []byte

	// Read optional sign
	c, err := buf.ReadByte()
	if err != nil {
		return 0, false
	}
	if c == '-' || c == '+' {
		s = append(s, c)
		*consumed++
		if width > 0 {
			width--
		}
	} else {
		buf.UnreadByte()
	}

	if width == 0 {
		return 0, false
	}

	// For hex, skip optional 0x prefix
	if base == 16 {
		// Peek at the first 2 bytes to check for 0x/0X prefix
		// We use Peek to avoid the double-UnreadByte problem
		if peeked, err := buf.Peek(2); err == nil && len(peeked) == 2 {
			if peeked[0] == '0' && (peeked[1] == 'x' || peeked[1] == 'X') {
				// Consume the 0x prefix
				buf.ReadByte()
				buf.ReadByte()
				*consumed += 2
				if width > 0 {
					width -= 2
				}
			}
		}
	}

	gotDigit := false
	for width != 0 {
		c, err := buf.ReadByte()
		if err != nil {
			break
		}
		valid := false
		switch base {
		case 2:
			valid = c == '0' || c == '1'
		case 8:
			valid = c >= '0' && c <= '7'
		case 10:
			valid = c >= '0' && c <= '9'
		case 16:
			valid = (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		}
		if !valid {
			buf.UnreadByte()
			break
		}
		s = append(s, c)
		gotDigit = true
		*consumed++
		if width > 0 {
			width--
		}
	}

	if !gotDigit {
		// If we had a sign but no digits, unread the sign
		if len(s) > 0 && !gotDigit {
			// Can't easily unread multiple bytes, just return false
		}
		return 0, false
	}

	n, err2 := strconv.ParseInt(string(s), base, 64)
	if err2 != nil {
		un, err3 := strconv.ParseUint(strings.TrimPrefix(string(s), "+"), base, 64)
		if err3 != nil {
			return 0, false
		}
		return int64(un), true
	}
	return n, true
}

// scanReadUintTracked reads an unsigned decimal integer and tracks inputConsumed.
// It reads a signed decimal number and interprets the result as unsigned (like PHP's %u).
func scanReadUintTracked(buf *bufio.Reader, width int, consumed *int) (uint64, bool) {
	// Read the value as a signed int64 first (handles sign)
	n, ok := scanReadIntTracked(buf, 10, width, consumed)
	if !ok {
		return 0, false
	}
	// Reinterpret as unsigned (matching PHP behavior: -1 becomes 18446744073709551615)
	return uint64(n), true
}

// scanReadFloatTracked reads a float and tracks inputConsumed
func scanReadFloatTracked(buf *bufio.Reader, width int, consumed *int) (float64, bool) {
	var s []byte

	// Read optional sign
	c, err := buf.ReadByte()
	if err != nil {
		return 0, false
	}
	if c == '-' || c == '+' {
		s = append(s, c)
		*consumed++
		if width > 0 {
			width--
		}
	} else {
		buf.UnreadByte()
	}

	gotDigit := false
	gotDot := false
	gotE := false

	for width != 0 {
		c, err := buf.ReadByte()
		if err != nil {
			break
		}
		if c >= '0' && c <= '9' {
			s = append(s, c)
			gotDigit = true
			*consumed++
			if width > 0 {
				width--
			}
		} else if c == '.' && !gotDot && !gotE {
			s = append(s, c)
			gotDot = true
			*consumed++
			if width > 0 {
				width--
			}
		} else if (c == 'e' || c == 'E') && !gotE && gotDigit {
			s = append(s, c)
			gotE = true
			*consumed++
			if width > 0 {
				width--
			}
			// Read optional sign after e
			if width != 0 {
				c2, err2 := buf.ReadByte()
				if err2 == nil {
					if c2 == '+' || c2 == '-' {
						s = append(s, c2)
						*consumed++
						if width > 0 {
							width--
						}
					} else {
						buf.UnreadByte()
					}
				}
			}
		} else {
			buf.UnreadByte()
			break
		}
	}

	if !gotDigit {
		return 0, false
	}

	f, err2 := strconv.ParseFloat(string(s), 64)
	if err2 != nil {
		// If the string ends with an incomplete exponent (e.g., "1.0E" or "1.0E+"),
		// strip the exponent part and unread those characters, then try again.
		stripped := string(s)
		stripCount := 0
		if len(stripped) > 0 && (stripped[len(stripped)-1] == '+' || stripped[len(stripped)-1] == '-') {
			stripped = stripped[:len(stripped)-1]
			stripCount++
		}
		if len(stripped) > 0 && (stripped[len(stripped)-1] == 'e' || stripped[len(stripped)-1] == 'E') {
			stripped = stripped[:len(stripped)-1]
			stripCount++
		}
		if stripCount > 0 && len(stripped) > 0 {
			f2, err3 := strconv.ParseFloat(stripped, 64)
			if err3 == nil {
				// Unread the stripped characters
				*consumed -= stripCount
				for i := 0; i < stripCount; i++ {
					buf.UnreadByte()
				}
				return f2, true
			}
		}
		return 0, false
	}
	return f, true
}

func zscanfIntoArray(ctx phpv.Context, r io.Reader, format phpv.ZString) (*phpv.ZVal, error) {
	values, count, inputEmpty, scanFailed, err := zscanRead(r, format)
	if err != nil {
		if bsc, ok := err.(*errBadScanChar); ok {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, bsc.Error())
		}
		if air, ok := err.(*errArgIndexOutOfRange); ok {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, air.Error())
		}
		return nil, err
	}

	// Return NULL in the following cases (matching PHP behavior):
	// 1. Input is truly empty (zero bytes) → always NULL
	// 2. Scan failed (a specifier couldn't match) and no values were produced
	//    This covers whitespace-only input with %d/%s etc. where whitespace is skipped
	//    but no actual value could be read.
	if inputEmpty && len(values) == 0 {
		return phpv.ZNULL.ZVal(), nil
	}
	if scanFailed && len(values) == 0 {
		return phpv.ZNULL.ZVal(), nil
	}

	// If no values matched but input was non-empty, return array of NULLs
	result := phpv.NewZArray()
	for _, v := range values {
		result.OffsetSet(ctx, nil, v)
	}

	for d := count - len(values); d > 0; d-- {
		result.OffsetSet(ctx, nil, phpv.ZNULL.ZVal())
	}

	return result.ZVal(), nil
}

func zscanfIntoRef(ctx phpv.Context, r io.Reader, format phpv.ZString, args ...*phpv.ZVal) (*phpv.ZVal, error) {
	values, count, _, _, err := zscanRead(r, format)
	if err != nil {
		if bsc, ok := err.(*errBadScanChar); ok {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, bsc.Error())
		}
		if air, ok := err.(*errArgIndexOutOfRange); ok {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, air.Error())
		}
		return nil, err
	}

	_ = count

	// Count total specifiers (excluding suppressed ones)
	totalSpecs := 0
	for i := 0; i < len(format); i++ {
		if format[i] == '%' {
			i++
			if i >= len(format) {
				break
			}
			if format[i] == '%' {
				continue
			}
			// skip position specifier
			j := i
			for j < len(format) && format[j] >= '0' && format[j] <= '9' {
				j++
			}
			if j > i && j < len(format) && format[j] == '$' {
				i = j + 1
				if i >= len(format) {
					break
				}
			}
			// check for suppression
			if i < len(format) && format[i] == '*' {
				// Skip width and format char
				i++
				for i < len(format) && format[i] >= '0' && format[i] <= '9' {
					i++
				}
				if i < len(format) && format[i] == '[' {
					i++
					if i < len(format) && format[i] == '^' {
						i++
					}
					if i < len(format) && format[i] == ']' {
						i++
					}
					for i < len(format) && format[i] != ']' {
						i++
					}
				}
				continue // suppressed, don't count
			}
			// skip width
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				i++
			}
			if i >= len(format) {
				break
			}
			if format[i] == '[' {
				i++
				if i < len(format) && format[i] == '^' {
					i++
				}
				if i < len(format) && format[i] == ']' {
					i++
				}
				for i < len(format) && format[i] != ']' {
					i++
				}
			}
			totalSpecs++
		}
	}

	if totalSpecs < len(args) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Variable is not assigned by any conversion specifiers")
	}

	if totalSpecs > len(args) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Different numbers of variable names and field specifiers")
	}

	for i, val := range values {
		if i >= len(args) {
			break
		}
		if val == nil {
			continue
		}
		// Set the by-reference arg directly. If the arg has a Name (variable name),
		// use OffsetSet to modify the variable in the calling scope. Otherwise,
		// use Set() which handles ZVal references properly.
		if args[i].Name != nil {
			parent := ctx.Parent(1)
			if parent == nil {
				ctx.OffsetSet(ctx, *args[i].Name, val)
			} else {
				parent.OffsetSet(ctx, *args[i].Name, val)
			}
		} else {
			args[i].Set(val)
		}
	}

	return phpv.ZInt(len(values)).ZVal(), nil
}

func Zscanf(ctx phpv.Context, r io.Reader, format phpv.ZString, args ...*phpv.ZVal) (*phpv.ZVal, error) {
	// Validate the entire format string before scanning (PHP behavior:
	// the entire format is validated upfront before any scanning occurs)
	if err := validateScanFormat(format); err != nil {
		if bsc, ok := err.(*errBadScanChar); ok {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, bsc.Error())
		}
		return nil, err
	}

	if len(args) > 0 {
		return zscanfIntoRef(ctx, r, format, args...)
	} else {
		return zscanfIntoArray(ctx, r, format)
	}
}

// Helper to check if a character is in a set (used by %[ ])
func charInSet(c byte, set []byte) bool {
	for _, s := range set {
		if c == s {
			return true
		}
	}
	return false
}

// isDigit checks if a byte is an ASCII digit
func isDigitByte(c byte) bool {
	return c >= '0' && c <= '9'
}

// isHexDigit checks if a byte is a hex digit
func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// Ensure unused imports are satisfied
var _ = unicode.IsDigit
var _ = strings.TrimSpace
