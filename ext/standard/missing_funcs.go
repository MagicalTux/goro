package standard

import (
	"bytes"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/stream"
)

// > func void clearstatcache ([ bool $clear_realpath_cache = FALSE [, string $filename = "" ]] )
func fncClearstatcache(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// No-op in Go - we don't cache stat results
	return nil, nil
}

// > func mixed fscanf ( resource $handle , string $format [, mixed &$... ] )
func fncFscanf(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var fmt phpv.ZString
	n, err := core.Expand(ctx, args, &handle, &fmt)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	var file *stream.Stream
	if handle.GetResourceType() == phpv.ResourceStream {
		file, _ = handle.(*stream.Stream)
	}
	if file == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	// Read one line from the stream
	var buf []byte
	for {
		b, err := file.ReadByte()
		if err != nil {
			break
		}
		if b == '\n' {
			break
		}
		buf = append(buf, b)
	}

	if len(buf) == 0 && file.Eof() {
		return phpv.ZFalse.ZVal(), nil
	}

	line := string(buf)
	r := strings.NewReader(line)
	output, err := core.Zscanf(ctx, r, fmt, args[n:]...)
	if err != nil {
		return nil, err
	}

	return output, nil
}

// > func array get_included_files ( void )
func fncGetIncludedFiles(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	files := ctx.Global().GetIncludedFiles()
	result := phpv.NewZArray()
	for _, f := range files {
		result.OffsetSet(ctx, nil, phpv.ZString(f).ZVal())
	}
	return result.ZVal(), nil
}

// > func int|false readfile ( string $filename [, bool $use_include_path = FALSE [, resource $context ]] )
func fncReadfile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var useIncludePath *phpv.ZBool
	var contextResource core.Optional[phpv.Resource]
	_, err := core.Expand(ctx, args, &filename, &useIncludePath, &contextResource)
	if err != nil {
		return nil, err
	}

	usePath := useIncludePath != nil && bool(*useIncludePath)

	f, err := ctx.Global().Open(ctx, filename, "r", usePath)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("readfile(%s): Failed to open stream: %s", filename, err)
	}
	defer f.Close()

	n, err := io.Copy(ctx, f)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZInt(n).ZVal(), nil
}

// > func string set_include_path ( string $new_include_path )
func fncSetIncludePath(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var newPath phpv.ZString
	_, err := core.Expand(ctx, args, &newPath)
	if err != nil {
		return nil, err
	}

	// Empty string is not a valid include path
	if newPath == "" {
		return phpv.ZFalse.ZVal(), nil
	}

	old := ctx.GetConfig("include_path", phpv.ZString(".").ZVal())
	ctx.Global().SetLocalConfig("include_path", newPath.ZVal())
	return old, nil
}

// > func string get_include_path ( void )
func fncGetIncludePath(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return ctx.GetConfig("include_path", phpv.ZString(".").ZVal()), nil
}

// > func array|false fgetcsv ( resource $handle [, int $length = 0 [, string $separator = "," [, string $enclosure = '"' [, string $escape = "\\" ]]]] )
func fncFgetcsv(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var lengthArg *phpv.ZInt
	var sepArg, encArg, escArg *phpv.ZString
	_, err := core.Expand(ctx, args, &handle, &lengthArg, &sepArg, &encArg, &escArg)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	var file *stream.Stream
	if handle.GetResourceType() == phpv.ResourceStream {
		file, _ = handle.(*stream.Stream)
	}
	if file == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	sep := byte(',')
	enc := byte('"')
	esc := byte('\\')

	if sepArg != nil && len(*sepArg) > 0 {
		sep = (*sepArg)[0]
	}
	if encArg != nil && len(*encArg) > 0 {
		enc = (*encArg)[0]
	}
	if escArg != nil {
		if len(*escArg) > 0 {
			esc = (*escArg)[0]
		} else {
			esc = 0 // empty string means no escape
		}
	}

	maxLen := 0
	if lengthArg != nil && *lengthArg > 0 {
		maxLen = int(*lengthArg)
	}

	// PHP's fgetcsv reads across multiple lines when inside a quoted field.
	// We read bytes one at a time, tracking quote state, so newlines within
	// quoted fields are included in the data rather than terminating the record.
	var line []byte
	inQuotes := false
	totalRead := 0

	for {
		b, readErr := file.ReadByte()
		if readErr != nil {
			if len(line) == 0 {
				return phpv.ZFalse.ZVal(), nil
			}
			break
		}
		totalRead++

		if !inQuotes {
			if b == '\n' {
				break
			}
			if b == '\r' {
				// Check for \r\n
				nb, err := file.ReadByte()
				if err == nil && nb != '\n' {
					file.Seek(-1, io.SeekCurrent)
				}
				break
			}
			if b == enc {
				inQuotes = true
			}
			line = append(line, b)
		} else {
			// Inside quotes: newlines are part of the field
			if esc != 0 && esc != enc && b == esc {
				// Escape char: include it and the next char
				line = append(line, b)
				nb, readErr2 := file.ReadByte()
				if readErr2 == nil {
					line = append(line, nb)
					totalRead++
				}
				if maxLen > 0 && totalRead >= maxLen-1 {
					break
				}
				continue
			}
			if b == enc {
				line = append(line, b)
				// Check for doubled enclosure
				nb, readErr2 := file.ReadByte()
				if readErr2 != nil {
					// EOF after closing quote
					inQuotes = false
					break
				}
				if nb == enc {
					// Doubled enclosure - still inside quotes
					line = append(line, nb)
					totalRead++
				} else {
					// End of quoted field - push back next byte
					inQuotes = false
					file.Seek(-1, io.SeekCurrent)
				}
			} else {
				line = append(line, b)
			}
		}

		if maxLen > 0 && totalRead >= maxLen-1 {
			break
		}
	}

	// Parse CSV
	return ParseCsvLine(ctx, string(line), sep, enc, esc)
}

func ParseCsvLine(ctx phpv.Context, line string, sep, enc, esc byte) (*phpv.ZVal, error) {
	result := phpv.NewZArray()
	i := 0
	trailingSep := false
	for i <= len(line) {
		if i == len(line) {
			if trailingSep {
				// Trailing separator means empty final field
				result.OffsetSet(ctx, nil, phpv.ZString("").ZVal())
			}
			break
		}
		if line[i] == enc {
			// Enclosed field
			i++ // skip opening enclosure
			var field []byte
			for i < len(line) {
				if esc != 0 && esc != enc && line[i] == esc && i+1 < len(line) {
					// Escape character: the next char is escaped (not a field terminator if it's enc)
					// PHP keeps both the escape char and the escaped char in the output
					field = append(field, line[i])
					i++
					field = append(field, line[i])
					i++
				} else if line[i] == enc {
					if esc == enc && i+1 < len(line) && line[i+1] == enc {
						// Doubled enclosure used as escape = literal enclosure
						field = append(field, enc)
						i += 2
					} else if esc != enc && i+1 < len(line) && line[i+1] == enc {
						// Doubled enclosure = literal enclosure
						field = append(field, enc)
						i += 2
					} else {
						// End of enclosed field
						i++ // skip closing enclosure
						break
					}
				} else {
					field = append(field, line[i])
					i++
				}
			}
			// Skip to next separator
			for i < len(line) && line[i] != sep {
				i++
			}
			trailingSep = false
			if i < len(line) {
				i++ // skip separator
				trailingSep = true
			}
			result.OffsetSet(ctx, nil, phpv.ZString(field).ZVal())
		} else {
			// Unenclosed field
			start := i
			for i < len(line) && line[i] != sep {
				i++
			}
			field := line[start:i]
			trailingSep = false
			if i < len(line) {
				i++ // skip separator
				trailingSep = true
			}
			result.OffsetSet(ctx, nil, phpv.ZString(field).ZVal())
		}
	}

	return result.ZVal(), nil
}

// > func int|false fputcsv ( resource $handle , array $fields [, string $separator = "," [, string $enclosure = '"' [, string $escape = "\\" ]]] )
func fncFputcsv(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var fields *phpv.ZArray
	var sepArg, encArg, escArg *phpv.ZString
	_, err := core.Expand(ctx, args, &handle, &fields, &sepArg, &encArg, &escArg)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	var file *stream.Stream
	if handle.GetResourceType() == phpv.ResourceStream {
		file, _ = handle.(*stream.Stream)
	}
	if file == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	sep := byte(',')
	enc := byte('"')
	esc := byte('\\')

	if sepArg != nil && len(*sepArg) > 0 {
		sep = (*sepArg)[0]
	}
	if encArg != nil && len(*encArg) > 0 {
		enc = (*encArg)[0]
	}
	if escArg != nil && len(*escArg) > 0 {
		esc = (*escArg)[0]
	}

	lineBytes, err := BuildCsvLine(ctx, fields, sep, enc, esc)
	if err != nil {
		return nil, err
	}

	n, err := file.Write(lineBytes)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	return phpv.ZInt(n).ZVal(), nil
}

// BuildCsvLine builds a CSV line from a ZArray of fields. Returns the line as bytes (including trailing newline).
func BuildCsvLine(ctx phpv.Context, fields *phpv.ZArray, sep, enc, esc byte) ([]byte, error) {
	var buf bytes.Buffer
	first := true

	it := fields.NewIterator()
	for ; it.Valid(ctx); it.Next(ctx) {
		if !first {
			buf.WriteByte(sep)
		}
		first = false

		val, err := it.Current(ctx)
		if err != nil {
			return nil, err
		}

		// Convert non-string values to string, emit warning for arrays
		if val.GetType() == phpv.ZtArray {
			ctx.Warn("Array to string conversion")
		}
		field := val.String()

		// Check if enclosure is needed (matches PHP 8.5's php_fputcsv behavior)
		// PHP encloses fields containing separator, enclosure, escape, or whitespace chars.
		needsEnclose := strings.ContainsAny(field, string([]byte{sep, enc, '\n', '\r', '\t', ' '}))
		if esc != 0 && esc != enc {
			needsEnclose = needsEnclose || strings.ContainsRune(field, rune(esc))
		}

		if needsEnclose {
			buf.WriteByte(enc)
			for i := 0; i < len(field); i++ {
				c := field[i]
				if c == enc {
					// Double the enclosure unless preceded by escape char
					if esc != 0 && esc != enc && i > 0 && field[i-1] == esc {
						// Escape char already escapes this enclosure, write as-is
						buf.WriteByte(c)
					} else {
						buf.WriteByte(enc)
						buf.WriteByte(enc)
					}
				} else {
					// Write all other chars (including escape) as-is
					buf.WriteByte(c)
				}
			}
			buf.WriteByte(enc)
		} else {
			buf.WriteString(field)
		}
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// > func bool flock ( resource $handle , int $operation [, int &$wouldblock ] )
// Stub implementation - Go doesn't easily support flock across platforms.
func fncFlock(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handle phpv.Resource
	var operation phpv.ZInt
	var wouldblock core.OptionalRef[phpv.ZInt]
	_, err := core.Expand(ctx, args, &handle, &operation, &wouldblock)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	// Set wouldblock to 0 if passed by reference
	if wouldblock.HasArg() {
		wouldblock.Set(ctx, phpv.ZInt(0))
	}

	return phpv.ZTrue.ZVal(), nil
}

// > func int|bool version_compare ( string $version1, string $version2 [, string $operator ] )
func fncVersionCompare(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var v1, v2 phpv.ZString
	var op *phpv.ZString
	_, err := core.Expand(ctx, args, &v1, &v2, &op)
	if err != nil {
		return nil, err
	}

	cmp := compareVersions(string(v1), string(v2))
	
	if op == nil {
		if cmp < 0 { return phpv.ZInt(-1).ZVal(), nil }
		if cmp > 0 { return phpv.ZInt(1).ZVal(), nil }
		return phpv.ZInt(0).ZVal(), nil
	}

	var result bool
	switch string(*op) {
	case "<", "lt":
		result = cmp < 0
	case "<=", "le":
		result = cmp <= 0
	case ">", "gt":
		result = cmp > 0
	case ">=", "ge":
		result = cmp >= 0
	case "==", "=", "eq":
		result = cmp == 0
	case "!=", "ne", "<>":
		result = cmp != 0
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "version_compare(): Argument #3 ($operator) must be a valid comparison operator")
	}
	return phpv.ZBool(result).ZVal(), nil
}

func compareVersions(v1, v2 string) int {
	parts1 := splitVersion(v1)
	parts2 := splitVersion(v2)
	
	maxLen := len(parts1)
	if len(parts2) > maxLen { maxLen = len(parts2) }
	
	for i := 0; i < maxLen; i++ {
		var p1, p2 string
		if i < len(parts1) { p1 = parts1[i] }
		if i < len(parts2) { p2 = parts2[i] }
		
		cmp := compareVersionPart(p1, p2)
		if cmp != 0 { return cmp }
	}
	return 0
}

func splitVersion(v string) []string {
	// PHP splits on '.', '-', '_' and also on transitions between
	// digits and letters (e.g. "1a2" => ["1","a","2"]).
	var parts []string
	cur := ""
	prevIsDigit := false
	for i, c := range v {
		if c == '.' || c == '-' || c == '_' {
			if cur != "" {
				parts = append(parts, cur)
			}
			cur = ""
			continue
		}
		isDigit := c >= '0' && c <= '9'
		if i > 0 && cur != "" && isDigit != prevIsDigit {
			parts = append(parts, cur)
			cur = ""
		}
		cur += string(c)
		prevIsDigit = isDigit
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}

func compareVersionPart(a, b string) int {
	// Special version strings have specific ordering
	// In PHP: "dev" < "alpha" = "a" < "beta" = "b" < "RC" = "rc" < "#" < "pl" = "p"
	// Where "#" means any numeric string. Empty string is treated as 0 (numeric).
	specials := map[string]int{"dev": 0, "alpha": 1, "a": 1, "beta": 2, "b": 2, "rc": 3, "#": 4, "p": 5, "pl": 5}

	aLower := strings.ToLower(a)
	bLower := strings.ToLower(b)

	aNum, aIsNum := isVersionNum(a)
	bNum, bIsNum := isVersionNum(b)

	if aIsNum && bIsNum {
		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
		return 0
	}

	// Map to canonical weight
	aSpec, aIsSpec := specials[aLower]
	if aIsNum {
		aSpec = specials["#"]
		aIsSpec = true
	}
	bSpec, bIsSpec := specials[bLower]
	if bIsNum {
		bSpec = specials["#"]
		bIsSpec = true
	}

	if aIsSpec && bIsSpec {
		if aSpec < bSpec {
			return -1
		}
		if aSpec > bSpec {
			return 1
		}
		return 0
	}

	// Number > special string, special string > unknown
	if aIsSpec && !bIsSpec {
		return 1
	}
	if !aIsSpec && bIsSpec {
		return -1
	}

	// Fallback to string comparison
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func isVersionNum(s string) (int, bool) {
	if s == "" { return 0, true }
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' { return 0, false }
		n = n*10 + int(c-'0')
	}
	return n, true
}

// > func string quoted_printable_encode ( string $string )
func fncQuotedPrintableEncode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}
	var result []byte
	lineLen := 0
	for _, b := range []byte(s) {
		if (b >= 33 && b <= 126 && b != '=') || b == '\t' || b == ' ' {
			result = append(result, b)
			lineLen++
		} else if b == '\r' || b == '\n' {
			result = append(result, b)
			lineLen = 0
		} else {
			result = append(result, '=')
			result = append(result, "0123456789ABCDEF"[b>>4])
			result = append(result, "0123456789ABCDEF"[b&0xf])
			lineLen += 3
		}
		if lineLen >= 75 {
			result = append(result, '=', '\r', '\n')
			lineLen = 0
		}
	}
	return phpv.ZString(result).ZVal(), nil
}

// > func string utf8_decode ( string $string )
func fncUtf8Decode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}
	ctx.Deprecated("Function utf8_decode() is deprecated since 8.2, visit the php.net documentation for various alternatives", logopt.NoFuncName(true))
	// Convert UTF-8 to ISO-8859-1 (Latin-1)
	result := make([]byte, 0, len(s))
	for _, r := range string(s) {
		if r <= 0xFF {
			result = append(result, byte(r))
		} else {
			result = append(result, '?')
		}
	}
	return phpv.ZString(result).ZVal(), nil
}

// > func string utf8_encode ( string $string )
func fncUtf8Encode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}
	ctx.Deprecated("Function utf8_encode() is deprecated since 8.2, visit the php.net documentation for various alternatives", logopt.NoFuncName(true))
	// Convert ISO-8859-1 to UTF-8
	result := make([]rune, len(s))
	for i, b := range []byte(s) {
		result[i] = rune(b)
	}
	return phpv.ZStr(string(result)), nil
}

// > func string metaphone ( string $string [, int $max_phonemes = 0 ] )
func fncMetaphone(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}
	var maxPhonemes core.Optional[phpv.ZInt]
	core.Expand(ctx, args[1:], &maxPhonemes)
	maxP := int(maxPhonemes.GetOrDefault(0))
	result := metaphone(strings.ToUpper(string(s)), maxP)
	return phpv.ZString(result).ZVal(), nil
}

func metaphone(word string, maxPhonemes int) string {
	if len(word) == 0 {
		return ""
	}
	w := []byte(word)
	n := len(w)
	var result []byte
	isV := func(c byte) bool { return c == 'A' || c == 'E' || c == 'I' || c == 'O' || c == 'U' }
	add := func(c byte) {
		if maxPhonemes <= 0 || len(result) < maxPhonemes {
			result = append(result, c)
		}
	}
	at := func(i int) byte {
		if i < 0 || i >= n {
			return 0
		}
		return w[i]
	}

	i := 0
	// Skip initial silent letters
	if n >= 2 {
		switch string(w[:2]) {
		case "AE", "GN", "KN", "PN", "WR":
			i = 1
		}
	}

	// Track effective start for vowel handling
	startPos := i

	for i < n {
		if maxPhonemes > 0 && len(result) >= maxPhonemes {
			break
		}
		c := w[i]
		if c < 'A' || c > 'Z' {
			i++
			continue
		}
		// Skip doubled letters (except C)
		if c != 'C' && i > 0 && at(i-1) == c {
			i++
			continue
		}
		// Vowels: only emit if at effective start of word
		if isV(c) {
			if i == startPos {
				add(c)
			}
			i++
			continue
		}
		switch c {
		case 'B':
			if !(i > 0 && at(i-1) == 'M' && i+1 >= n) {
				add('B')
			}
		case 'C':
			nx := at(i + 1)
			if nx == 'I' || nx == 'E' || nx == 'Y' {
				if nx == 'I' && at(i+2) == 'A' {
					add('X')
					i += 2
				} else {
					add('S')
					i++
				}
			} else {
				add('K')
			}
		case 'D':
			if at(i+1) == 'G' {
				nx2 := at(i + 2)
				if nx2 == 'E' || nx2 == 'I' || nx2 == 'Y' {
					add('J')
					i += 2
				} else {
					add('T')
				}
			} else {
				add('T')
			}
		case 'F':
			add('F')
		case 'G':
			nx := at(i + 1)
			if i+1 < n && nx == 'H' {
				nx2 := at(i + 2)
				if nx2 != 0 && !isV(nx2) {
					// GH before non-vowel: silent
					i++
				} else if i == 0 {
					if nx2 == 'O' {
						// GHO: silent G
						i++
					} else {
						add('K')
						i++
					}
				} else {
					// GH after vowel: silent
					i++
				}
			} else if nx == 'N' {
				if i+2 >= n || (i+2 < n && at(i+2) == 'E' && i+3 >= n) {
					// GN at end or GNE at end: silent
				} else if i == 0 {
					// Initial GN: silent G
				} else {
					add('K')
				}
			} else if i > 0 && at(i-1) == 'G' {
				add('K')
			} else {
				pv := at(i - 1)
				if i > 0 && (nx == 'E' || nx == 'I' || nx == 'Y') && pv != 'G' {
					add('J')
				} else if i == 0 || pv != 'G' {
					add('K')
				}
			}
		case 'H':
			if isV(at(i + 1)) {
				pv := at(i - 1)
				if i == 0 || (pv != 'C' && pv != 'G' && pv != 'P' && pv != 'S' && pv != 'T') {
					add('H')
				}
			}
		case 'J':
			add('J')
		case 'K':
			if i == 0 || at(i-1) != 'C' {
				add('K')
			}
		case 'L':
			add('L')
		case 'M':
			add('M')
		case 'N':
			add('N')
		case 'P':
			if at(i+1) == 'H' {
				add('F')
				i++
			} else {
				add('P')
			}
		case 'Q':
			add('K')
		case 'R':
			add('R')
		case 'S':
			nx := at(i + 1)
			if nx == 'H' || (nx == 'I' && (at(i+2) == 'A' || at(i+2) == 'O')) {
				add('X')
				if nx == 'H' {
					i++
				} else {
					i += 2
				}
			} else if nx == 'C' && at(i+2) == 'H' {
				add('S')
				add('K')
				i += 2
			} else {
				add('S')
			}
		case 'T':
			nx := at(i + 1)
			if nx == 'H' {
				add('0')
				i++
			} else if nx == 'C' && at(i+2) == 'H' {
				// TCH -> X (like "scratch", "match")
				add('X')
				i += 2
			} else if nx == 'I' && (at(i+2) == 'A' || at(i+2) == 'O') {
				add('X')
				i++
			} else {
				add('T')
			}
		case 'V':
			add('F')
		case 'W':
			if at(i+1) == 'H' {
				// WH -> W (treat like W + vowel)
				if isV(at(i + 2)) {
					add('W')
					i++ // skip H
				} else if i == 0 {
					add('W')
					i++ // skip H
				}
			} else if isV(at(i + 1)) {
				add('W')
			}
		case 'Y':
			if isV(at(i + 1)) {
				add('Y')
			}
		case 'X':
			if i == 0 {
				// Initial X sounds like S
				add('S')
			} else {
				add('K')
				add('S')
			}
		case 'Z':
			add('S')
		}
		i++
	}
	return string(result)
}

// > func string|false crypt ( string $string , string $salt )
// fncCrypt delegates to the CGo-based implementation in crypt.go
func fncCrypt(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return fncCryptImpl(ctx, args)
}
