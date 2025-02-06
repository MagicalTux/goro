package standard

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

const trimChars = " \n\r\t\v\000\x0B"

// > const
var (
	STR_PAD_LEFT  = phpv.ZInt(0)
	STR_PAD_RIGHT = phpv.ZInt(1)
	STR_PAD_BOTH  = phpv.ZInt(2)
)

var (
	addSlashesReplacer = strings.NewReplacer(
		`'`, `\'`,
		`"`, `\"`,
		`\`, `\\`,
		"\000", "\\0",
	)
	nl2brReplacer = strings.NewReplacer(
		"\r\n", "<br>\r\n",
		"\n\r", "<br>\n\r",
		"\r", "<br>",
		"\n", "<br>",
	)
	nl2brReplacerXHTML = strings.NewReplacer(
		"\r\n", "<br />\r\n",
		"\n\r", "<br />\n\r",
		"\r", "<br />",
		"\n", "<br />",
	)
	quoteMetaReplacer = strings.NewReplacer(
		`.`, `\.`,
		`\`, `\\`,
		`+`, `\+`,
		`*`, `\*`,
		`?`, `\?`,
		`[`, `\[`,
		`^`, `\^`,
		`]`, `\]`,
		`(`, `\(`,
		`$`, `\$`,
		`)`, `\)`,
	)
)

// > func string addcslashes ( string $string, string $charlist )
func fncStrAddCSlashes(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var strArg phpv.ZString
	var charlistArg phpv.ZString
	_, err := core.Expand(ctx, args, &strArg, &charlistArg)
	if err != nil {
		return nil, err
	}

	str := []byte(strArg)
	escaped := map[byte]struct{}{}

	charlist := []byte(stripCSlashes(string(charlistArg)))
	for i := 0; i < len(charlist); i++ {
		c := charlist[i]

		c2 := safeIndex(charlist, i+1)
		c3 := safeIndex(charlist, i+2)
		c4 := safeIndex(charlist, i+3, c)

		if c2 == '.' && c3 == '.' {
			if i+3 >= len(charlist) {
				// TODO: show warning: addcslashes(): Invalid '..'-range, no character to the right of '..'
				escaped['.'] = struct{}{}
			}
			if c4 < c {
				// TODO: show warning: addcslashes(): Invalid '..'-range, no character to the right of '..'
				escaped[c] = struct{}{}
			} else {
				for ch := c; ch <= c4; ch++ {
					escaped[ch] = struct{}{}
				}
				i += 3
			}

		} else {
			escaped[c] = struct{}{}
		}
	}

	var buf bytes.Buffer
	for _, c := range str {
		if _, ok := escaped[c]; ok {
			buf.Write(escapeByte(c))
		} else {
			buf.WriteByte(c)
		}
	}

	return phpv.ZString(buf.String()).ZVal(), nil
}

// > func string addslashes ( string $string )
func fncStrAddSlashes(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	_, err := core.Expand(ctx, args, &str)
	if err != nil {
		return nil, err
	}

	result := addSlashesReplacer.Replace(string(str))
	return phpv.ZString(result).ZVal(), nil
}

// > func string chr ( int $codepoint )
func fncChr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var codepoint phpv.ZInt

	_, err := core.Expand(ctx, args, &codepoint)
	if err != nil {
		return nil, err
	}

	b := uint8(codepoint)
	return phpv.ZString([]byte{b}).ZVal(), nil
}

// > func int ord ( string $string )
func fncOrd(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str []byte

	_, err := core.Expand(ctx, args, &str)
	if err != nil {
		return nil, err
	}

	if len(str) == 0 {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(str[0]).ZVal(), nil
}

// > func string chunk_split ( string $string, int $length = 76, string $separator = "\r\n" )
func fncStrChunkSplit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var strArg phpv.ZString
	var lengthArg *phpv.ZInt
	var sepArg *phpv.ZString

	_, err := core.Expand(ctx, args, &strArg, &lengthArg, &sepArg)
	if err != nil {
		return nil, err
	}

	str := string(strArg)
	length := 76
	sep := "\r\n"

	if sepArg != nil {
		sep = string(*sepArg)
	}
	if lengthArg != nil {
		length = int(*lengthArg)
		if length <= 0 {
			return nil, errors.New("Argument #2 ($length) must be greater than 0")
		}
	}

	var buf bytes.Buffer
	for index := 0; index < len(str); {
		j := min(len(str), index+length)
		s := str[index:j]
		buf.WriteString(s)
		buf.WriteString(sep)
		index += len(s)
	}

	return phpv.ZString(buf.String()).ZVal(), nil
}

// >  func array|string count_chars ( string $string, int $mode = 0 )
func fncStrCountChars(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var strArg phpv.ZString
	var modeArg *phpv.ZInt

	_, err := core.Expand(ctx, args, &strArg, &modeArg)
	if err != nil {
		return nil, err
	}

	mode := 0
	if modeArg != nil {
		mode = int(*modeArg)
	}

	switch mode {
	case 0: // return an array with the byte-value as key and the frequency of every byte as value.
		tally := make([]int, 256)
		for _, n := range []byte(strArg) {
			tally[n]++
		}
		result := phpv.NewZArray()
		for _, n := range tally {
			result.OffsetSet(ctx, nil, phpv.ZInt(n).ZVal())
		}
		return result.ZVal(), nil

	case 1: // return same as 0 but only byte-values with a frequency greater than zero are listed.
		tally := make([]int, 256)
		for _, n := range []byte(strArg) {
			tally[n]++
		}

		result := phpv.NewZArray()
		for i := 0; i < 256; i++ {
			n := tally[byte(i)]
			if n > 0 {
				k := phpv.ZInt(i).ZVal()
				v := phpv.ZInt(n).ZVal()
				result.OffsetSet(ctx, k, v)
			}
		}

		return result.ZVal(), nil

	case 2: // return same as 0 but only byte-values with a frequency equal to zero are listed.
		tally := make([]int, 256)
		for _, n := range []byte(strArg) {
			tally[n]++
		}

		result := phpv.NewZArray()
		for i := 0; i < 256; i++ {
			if n := tally[byte(i)]; n == 0 {
				k := phpv.ZInt(byte(i)).ZVal()
				v := phpv.ZInt(0).ZVal()
				result.OffsetSet(ctx, k, v)
			}
		}

		return result.ZVal(), nil

	case 3: // a string containing all unique characters is returned.
		tally := make([]int, 256)
		for _, n := range []byte(strArg) {
			tally[n]++
		}

		var buf bytes.Buffer
		for i, n := range tally {
			if n > 0 {
				buf.WriteByte(byte(i))
			}
		}
		return phpv.ZStr(buf.String()), nil

	case 4: // a string containing all unique characters is returned.
		tally := make([]int, 256)
		for _, n := range []byte(strArg) {
			tally[n]++
		}

		var buf bytes.Buffer
		for i, n := range tally {
			if n == 0 {
				buf.WriteByte(byte(i))
			}
		}
		return phpv.ZStr(buf.String()), nil

	default:
		return nil, errors.New(`Argument #2 ($mode) must be between 0 and 4 (inclusive)`)
	}
}

// >  func array explode ( string $separator, string $string, int $limit = PHP_INT_MAX )
func fncStrExplode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var sep, str phpv.ZString
	var limitArg *phpv.ZInt

	_, err := core.Expand(ctx, args, &sep, &str, &limitArg)
	if err != nil {
		return nil, err
	}

	if sep == "" {
		// TODO: on PHP8+, throw an error
		return phpv.ZBool(false).ZVal(), nil
	}

	limit := math.MaxInt64
	if limitArg != nil {
		limit = int(*limitArg)
	}

	if limit == 0 {
		limit = 1
	}

	fields := strings.SplitN(string(str), string(sep), limit)
	if limit < 0 {
		// return all elements except the last -limit items
		fields = fields[0:max(0, len(fields)+limit)]
	}

	result := phpv.NewZArray()
	for _, item := range fields {
		result.OffsetSet(ctx, nil, phpv.ZStr(item))
	}

	return result.ZVal(), nil
}

// > func string implode ( string $separator, array $array )
// > alias join
func fncStrImplode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var arg1 *phpv.ZVal
	var arg2 **phpv.ZVal

	_, err := core.Expand(ctx, args, &arg1, &arg2)
	if err != nil {
		return nil, err
	}

	var sep phpv.ZString
	var array *phpv.ZArray

	if arg2 != nil {
		sep = arg1.AsString(ctx)
		array = (*arg2).AsArray(ctx)
	} else {
		array = arg1.AsArray(ctx)
		sep = phpv.ZString("")
	}

	var buf bytes.Buffer
	it := array.NewIterator()

	i := 0
	for ; it.Valid(ctx); it.Next(ctx) {
		v, err := it.Current(ctx)
		if err != nil {
			return nil, err
		}

		if i > 0 {
			buf.WriteString(string(sep))
		}
		buf.WriteString(string(v.AsString(ctx)))
		i++
	}

	return phpv.ZStr(buf.String()), nil
}

// > func string lcfirst ( string $string )
func fncStrLcFirst(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString

	_, err := core.Expand(ctx, args, &str)
	if err != nil {
		return nil, err
	}

	if len(str) == 0 {
		return str.ZVal(), nil
	}
	if str[0] < 0x41 || str[0] > 0x5a {
		return str.ZVal(), nil
	}

	chars := []byte(str)
	chars[0] = chars[0] + 32

	return phpv.ZStr(string(chars)), nil

}

// > func string ltrim ( string $str [, string $character_mask ] )
func fncStrLtrim(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var charsArg core.Optional[phpv.ZString]
	_, err := core.Expand(ctx, args, &str, &charsArg)
	if err != nil {
		return nil, err
	}

	chars := expandCharacterRanges(string(charsArg.GetOrDefault(trimChars)))
	result := strings.TrimLeft(string(str), chars)
	return phpv.ZString(result).ZVal(), nil
}

// > func string rtrim ( string $str [, string $character_mask ] )
// > alias chop
func fncStrRtrim(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var charsArg core.Optional[phpv.ZString]
	_, err := core.Expand(ctx, args, &str, &charsArg)
	if err != nil {
		return nil, err
	}

	chars := expandCharacterRanges(string(charsArg.GetOrDefault(trimChars)))
	result := strings.TrimRight(string(str), chars)
	return phpv.ZString(result).ZVal(), nil
}

// > func string trim ( string $str [, string $character_mask ] )
func fncStrTrim(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var charsArg core.Optional[phpv.ZString]
	_, err := core.Expand(ctx, args, &str, &charsArg)
	if err != nil {
		return nil, err
	}

	chars := expandCharacterRanges(string(charsArg.GetOrDefault(trimChars)))
	result := strings.Trim(string(str), chars)
	return phpv.ZString(result).ZVal(), nil
}

// > func string md5 ( string $string, bool $binary = false )
func fncStrMd5(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var binaryArg *phpv.ZBool

	_, err := core.Expand(ctx, args, &str, &binaryArg)
	if err != nil {
		return nil, err
	}

	binary := false
	if binaryArg != nil {
		binary = bool(*binaryArg)
	}

	sum := md5.Sum([]byte(str))

	var result string
	if binary {
		result = string(sum[:])
	} else {
		result = fmt.Sprintf("%x", sum)
	}

	return phpv.ZStr(result), nil
}

// > func string|false md5_file ( string $filename, bool $binary = false )
func fncStrMd5File(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var binaryArg *phpv.ZBool

	_, err := core.Expand(ctx, args, &filename, &binaryArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	binary := false
	if binaryArg != nil {
		binary = bool(*binaryArg)
	}

	f, err := os.Open(string(filename))
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}

	sum := h.Sum(nil)

	var result string
	if binary {
		result = string(sum[:])
	} else {
		result = fmt.Sprintf("%x", sum)
	}

	return phpv.ZStr(result), nil
}

// > func string nl2br ( string $string, bool $use_xhtml = true )
func fncStrNewLine2Br(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var useXHTMLArg *phpv.ZBool

	_, err := core.Expand(ctx, args, &str, &useXHTMLArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	useXHTML := true
	if useXHTMLArg != nil {
		useXHTML = bool(*useXHTMLArg)
	}

	var result string
	if useXHTML {
		result = nl2brReplacerXHTML.Replace(string(str))
	} else {
		result = nl2brReplacer.Replace(string(str))
	}

	return phpv.ZStr(result), nil

}

// > func string number_format ( float $num, int $decimals = 0, ?string $decimal_separator = ".", ?string $thousands_separator = "," )
func fncStrNumberFormat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var num phpv.ZFloat
	var decimalsArg *phpv.ZInt
	var decimalSepArg *phpv.ZString
	var thousandsSepArg *phpv.ZString

	_, err := core.Expand(ctx, args, &num, &decimalsArg, &decimalSepArg, &thousandsSepArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	decimals := 0
	decimalSep := "."
	thousandsSep := ","

	if decimalsArg != nil {
		decimals = int(*decimalsArg)
	}
	if decimalSepArg != nil {
		decimalSep = string(*decimalSepArg)
	}
	if thousandsSepArg != nil {
		thousandsSep = string(*thousandsSepArg)
	}

	var buf bytes.Buffer

	if num < 0 {
		num = -num
		buf.WriteRune('-')
	}

	n, fac := math.Modf(float64(num))

	if decimals == 0 {
		n += math.Round(fac)
	}

	if n == 0 {
		buf.WriteString("0")
	} else {
		for {
			base := int(math.Log10(n))
			x := int(n / math.Pow10(base))

			buf.WriteString(strconv.Itoa(x))
			if base%3 == 0 && base != 0 {
				buf.WriteString(thousandsSep)
			}

			n = float64(int(n) % int(math.Pow10(base)))
			nextBase := int(math.Log10(n))
			if base-nextBase > 1 {
				for b := base - 1; b > nextBase; b-- {
					buf.WriteString("0")
					if b%3 == 0 {
						buf.WriteString(thousandsSep)
					}
				}
			}
			if n == 0 {
				break
			}
		}
	}

	if decimals > 0 {
		if fac > 0 {
			n := math.Round(fac * math.Pow10(decimals))
			buf.WriteString(decimalSep)
			buf.WriteString(strconv.Itoa(int(n)))
		} else {
			buf.WriteString(decimalSep)
			buf.WriteString(strings.Repeat("0", decimals))
		}
	}

	return phpv.ZStr(buf.String()), nil
}

// > fun int ord(string $character)
func fncStrOrd(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var ch phpv.ZString

	_, err := core.Expand(ctx, args, &ch)
	if err != nil {
		return nil, err
	}

	if len(ch) == 0 {
		return phpv.ZInt(0).ZVal(), nil
	}

	fc := []byte(ch)[0]
	return phpv.ZInt(int(fc)).ZVal(), nil
}

// > func void parse_str ( string $encoded_string [, array &$result ] )
func fncStrParseStr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var arg core.OptionalRef[*phpv.ZVal]

	_, err := core.Expand(ctx, args, &str, &arg)
	if err != nil {
		return nil, err
	}

	if !arg.HasArg() {
		if err := ctx.Deprecated("Calling parse_str() without the result argument is deprecated"); err != nil {
			return nil, err
		}
	}

	var array phpv.ZArrayAccess
	if !arg.HasArg() {
		array = ctx.Parent(1)
	} else if arg.Get().GetType() != phpv.ZtArray {
		array = phpv.NewZArray()
	} else {
		arr := arg.Get().AsArray(ctx)
		arr.Clear(ctx)
		array = arr
	}

	err = parseQuery(ctx, string(str), array)
	if err != nil {
		return nil, err
	}

	if arg.HasArg() {
		arr := array.(*phpv.ZArray)
		arg.Set(ctx, arr.ZVal())
	}

	return nil, nil
}

// > func string quotemeta ( string $string )
func fncStrQuoteMeta(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString

	_, err := core.Expand(ctx, args, &str)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	result := quoteMetaReplacer.Replace(string(str))
	return phpv.ZStr(result), nil

}

// > func string sha1 ( string $string, bool $binary = false )
func fncStrSha1(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var binaryArg *phpv.ZBool

	_, err := core.Expand(ctx, args, &str, &binaryArg)
	if err != nil {
		return nil, err
	}

	binary := false
	if binaryArg != nil {
		binary = bool(*binaryArg)
	}

	sum := sha1.Sum([]byte(str))

	var result string
	if binary {
		result = string(sum[:])
	} else {
		result = fmt.Sprintf("%x", sum)
	}

	return phpv.ZStr(result), nil
}

// > func string|false sha1_file ( string $filename, bool $binary = false )
func fncStrSha1File(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var binaryArg *phpv.ZBool

	_, err := core.Expand(ctx, args, &filename, &binaryArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	binary := false
	if binaryArg != nil {
		binary = bool(*binaryArg)
	}

	f, err := os.Open(string(filename))
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}

	sum := h.Sum(nil)

	var result string
	if binary {
		result = string(sum[:])
	} else {
		result = fmt.Sprintf("%x", sum)
	}

	return phpv.ZStr(result), nil
}

// > func bool str_contains ( string $haystack, string $needle )
func fncStrContains(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString
	_, err := core.Expand(ctx, args, &haystack, &needle)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	result := strings.Contains(string(haystack), string(needle))
	return phpv.ZBool(result).ZVal(), nil
}

// > func bool str_ends_with ( string $haystack, string $needle )
func fncStrEndsWith(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString

	_, err := core.Expand(ctx, args, &haystack, &needle)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	result := strings.HasSuffix(string(haystack), string(needle))
	return phpv.ZBool(result).ZVal(), nil
}

// > func array str_getcsv ( string $string, string $separator = ",", string $enclosure = "\"", string $escape = "\\" )
func fncStrGetCsv(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var sepArg, encArg, escArg *phpv.ZString

	_, err := core.Expand(ctx, args, &str, &sepArg, &encArg, &escArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	if str == "" {
		// return an array with only a NULL element
		result := phpv.NewZArray()
		result.OffsetSet(ctx, nil, nil)
		return result.ZVal(), nil
	}

	sep := ","
	enc := "\""
	esc := "\\"

	if sepArg != nil {
		sep = string(*sepArg)
	}
	if encArg != nil {
		enc = string(*encArg)
	}
	if escArg != nil {
		esc = string(*escArg)
	}

	result := phpv.NewZArray()

	var buf bytes.Buffer

	escapeIndex := -1
	skippedSpcs := 0
	inserted := false
	enclosed := false

	// Not explicitly documented, but str_getcsv (weirdly) behaves as follows:
	// - enclosure only applies right after comma, or at beginning of the string
	//     so  'a,"b,c",d' == array('a', 'b,c', d)
	//     but 'a,b",c",d' == array('a', 'b"','c"', d)
	// - spaces before a valid enclosure is discarded
	//     so  'a,  "b,c",d' == array('a', 'b,c', d)
	//     but 'a,  b",c",d' == array('a', '  b"','c"', d)
	// - if enclosure is not valid, it should be added as part of the string
	// - it wasn't clearly stated, but escape only applies to enclosure, not to separators
	//     so  'a,"b\",c",d' == array('a', 'b",c', d)
	//     but 'a\,b,c,d' == array('a\', 'b', 'c', 'd')

	for i, b := range []byte(str) {
		ch := string(b)
		if ch == sep && !enclosed {
			result.OffsetSet(ctx, nil, phpv.ZStr(buf.String()))
			buf.Reset()
			inserted = false
		} else if ch == enc && !inserted && escapeIndex != i {
			enclosed = true
			skippedSpcs = 0
		} else if ch == enc && enclosed && escapeIndex != i {
			enclosed = false
		} else if ch == esc {
			escapeIndex = i + 1
		} else {
			if !inserted && ch == " " {
				skippedSpcs++
			} else {
				for i := 0; i < skippedSpcs; i++ {
					buf.WriteRune(' ')
				}
				buf.WriteString(ch)
				inserted = true
				skippedSpcs = 0
			}

		}
	}

	if buf.Len() > 0 || !inserted {
		result.OffsetSet(ctx, nil, phpv.ZStr(buf.String()))
	}

	return result.ZVal(), nil
}

// > func mixed str_ireplace ( mixed $search , mixed $replace , mixed $subject [, int &$count ] )
func stdStrIReplace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return strReplaceCommon(ctx, args, false)
}

// > func mixed str_replace ( mixed $search , mixed $replace , mixed $subject [, int &$count ] )
func stdStrReplace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return strReplaceCommon(ctx, args, true)
}

// > func string str_rot13 ( string $str )
func fncStrRot13(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	obuf := make([]byte, len(s))
	for i, v := range []byte(s) {
		if v >= 'a' && v <= 'z' {
			v = 'a' + ((v - 'a' + 13) % 26)
		} else if v >= 'A' && v <= 'Z' {
			v = 'A' + ((v - 'A' + 13) % 26)
		}
		obuf[i] = v
	}

	return phpv.ZString(obuf).ZVal(), nil
}

// > func string str_pad ( string $string, int $length, string $pad_string = " ", int $pad_type = STR_PAD_RIGHT )
func fncStrPad(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var length phpv.ZInt
	var padStrArg *phpv.ZString
	var padTypeArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &str, &length, &padStrArg, &padTypeArg)
	if err != nil {
		return nil, err
	}

	if len(str) >= int(length) {
		return str.ZVal(), nil
	}

	padStr := " "
	padType := STR_PAD_LEFT

	if padStrArg != nil {
		padStr = string(*padStrArg)
	}
	if padTypeArg != nil {
		padType = *padTypeArg
	}

	var buf bytes.Buffer
	switch padType {
	case STR_PAD_LEFT:
		buf.WriteString(strings.Repeat(padStr, int(length)-len(str)))
		buf.WriteString(string(str))
	case STR_PAD_RIGHT:
		buf.WriteString(string(str))
		buf.WriteString(strings.Repeat(padStr, int(length)-len(str)))
	case STR_PAD_BOTH:
		n := (int(length) - len(str))
		right := n / 2
		if n&1 == 1 {
			right++
		}
		left := n - right
		buf.WriteString(strings.Repeat(padStr, left))
		buf.WriteString(string(str))
		buf.WriteString(strings.Repeat(padStr, right))
	}

	return phpv.ZStr(buf.String()), nil
}

// > func string str_repeat ( string $string, int $times )
func fncStrRepeat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var times phpv.ZInt
	_, err := core.Expand(ctx, args, &str, &times)
	if err != nil {
		return nil, err
	}

	if times < 0 {
		return nil, errors.New("Argument #2 ($times) must be greater than or equal to 0")
	}

	return phpv.ZStr(strings.Repeat(string(str), int(times))), nil
}

// > func string strtolower ( string $string )
func fncStrToLower(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	return s.ToLower().ZVal(), nil
}

// > func string strtoupper ( string $string )
func fncStrToUpper(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	return s.ToUpper().ZVal(), nil
}

// > func string str_shuffle ( string $string )
func fncStrShuffle(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	_, err := core.Expand(ctx, args, &str)
	if err != nil {
		return nil, err
	}

	chars := []byte(str)
	sort.Slice(chars, func(i, j int) bool {
		return rand.IntN(2) == 1
	})

	return phpv.ZStr(string(chars)), nil
}

// > func array str_split ( string $string, int $length = 1 )
func fncStrSplit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var lengthArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &str, &lengthArg)
	if err != nil {
		return nil, err
	}

	length := 1
	if lengthArg != nil {
		length = int(*lengthArg)
	}

	if length < 1 {
		return nil, errors.New("Argument #2 ($length) must be greater than 0")
	}

	result := phpv.NewZArray()
	for i := 0; i < len(str); i += length {
		sub := str[i:min(i+length, len(str))]
		result.OffsetSet(ctx, nil, sub.ZVal())
	}

	return result.ZVal(), nil
}

// > func bool str_starts_with (string $haystack, string $needle)
func fncStrStartsWith(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle phpv.ZString

	_, err := core.Expand(ctx, args, &haystack, &needle)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	result := strings.HasPrefix(string(haystack), string(needle))
	return phpv.ZBool(result).ZVal(), nil
}

// > func array|int str_word_count ( string $string, int $format = 0, ?string $characters = null )
func fncStrWordCount(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var formatArg *phpv.ZInt
	var charsArg *phpv.ZString
	_, err := core.Expand(ctx, args, &str, &formatArg, &charsArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	format := 0
	chars := ""

	if formatArg != nil {
		format = int(*formatArg)
	}
	if charsArg != nil {
		chars = string(*charsArg)
	}

	switch format {
	case 0:
		wordCount := 0
		inWord := false
		for _, c := range str {
			isWord := unicode.IsLetter(c) || strings.ContainsRune(chars, c)
			if !inWord && isWord {
				wordCount++
			}
			inWord = isWord
		}
		return phpv.ZInt(wordCount).ZVal(), nil

	case 1:
		fallthrough
	case 2:
		words := phpv.NewZArray()
		inWord := false
		j := 0
		for i, c := range str {
			isWord := unicode.IsLetter(c) || strings.ContainsRune(chars, c)

			if isWord && !inWord {
				j = i
			} else if !isWord && inWord {
				word := str[j:i]
				if format == 2 {
					words.OffsetSet(ctx, phpv.ZInt(j), word.ZVal())
				} else {
					words.OffsetSet(ctx, nil, word.ZVal())
				}
			}

			inWord = isWord
		}
		if inWord {
			word := str[j:]
			if format == 2 {
				words.OffsetSet(ctx, phpv.ZInt(j), word.ZVal())
			} else {
				words.OffsetSet(ctx, nil, word.ZVal())
			}
		}

		return words.ZVal(), nil
	default:
		return nil, errors.New("Argument #2 ($format) must be a valid format value")
	}
}

// > func int strcasecmp ( string $string1, string $string2 )
func fncStrCaseCmp(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str1, str2 phpv.ZString
	_, err := core.Expand(ctx, args, &str1, &str2)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	result := strcmpCommon([]byte(str1), []byte(str2), false)
	return phpv.ZInt(result).ZVal(), nil
}

// > func int strncasecmp ( string $string1, string $string2 )
func fncStrNCaseCmp(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str1, str2 []byte
	var length phpv.ZInt
	_, err := core.Expand(ctx, args, &str1, &str2, &length)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	str1 = str1[0:min(int(length), len(str1))]
	str2 = str2[0:min(int(length), len(str2))]
	result := strcmpCommon(str1, str2, false)
	return phpv.ZInt(result).ZVal(), nil
}

// > func int strcmp ( string $string1, string $string2 )
func fncStrCmp(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str1, str2 phpv.ZString
	_, err := core.Expand(ctx, args, &str1, &str2)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	result := strcmpCommon([]byte(str1), []byte(str2), true)
	return phpv.ZInt(result).ZVal(), nil
}

// > func int strcoll ( string $string1, string $string2 )
func fncStrColl(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str1, str2 phpv.ZString
	_, err := core.Expand(ctx, args, &str1, &str2)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	return phpv.ZInt(strings.Compare(string(str1), string(str2))).ZVal(), nil
}

// > func int strcspn ( string $string, string $characters, int $offset = 0, ?int $length = null )
func fncStrCspn(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var strArg phpv.ZString
	var charsArg phpv.ZString
	var offsetArg *phpv.ZInt
	var lengthArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &strArg, &charsArg, &offsetArg, &lengthArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	offset := 0
	length := len(strArg)
	if offsetArg != nil {
		offset = int(*offsetArg)
	}
	if lengthArg != nil {
		length = int(*lengthArg)
	}

	str := substr([]byte(strArg), offset, length)
	chars := []byte(charsArg)

	segmentLen := 0
	for _, b := range []byte(str) {
		if bytes.ContainsRune(chars, rune(b)) {
			break
		}
		segmentLen++
	}

	return phpv.ZInt(segmentLen).ZVal(), nil
}

// > func string|false stristr ( string $haystack, string $needle, bool $before_needle = false )
func fncStrIStr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystackArg phpv.ZString
	// TODO: maybe handle deprecated case where needle not a string
	var needleArg phpv.ZString
	var beforeArg *phpv.ZBool
	_, err := core.Expand(ctx, args, &haystackArg, &needleArg, &beforeArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	haystack := bytesLowerCaseArray([]byte(haystackArg))
	needle := bytesLowerCaseArray([]byte(needleArg))

	beforeNeedle := false
	if beforeArg != nil {
		beforeNeedle = bool(*beforeArg)
	}

	i := bytes.Index(haystack, needle)
	if i < 0 {
		return phpv.ZBool(false).ZVal(), nil
	}

	var result []byte
	if beforeNeedle {
		result = []byte(haystackArg)[0:i]
	} else {
		result = []byte(haystackArg)[i:]
	}

	return phpv.ZStr(string(result)), nil
}

// > func string|false strstr ( string $haystack, string $needle, bool $before_needle = false )
// > alias strchr
func fncStrStr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystackArg phpv.ZString
	var needleArg *phpv.ZVal
	var beforeArg *phpv.ZBool
	_, err := core.Expand(ctx, args, &haystackArg, &needleArg, &beforeArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	haystack := []byte(haystackArg)
	beforeNeedle := false
	var needle []byte

	if needleArg.GetType() == phpv.ZtInt {
		n := byte(needleArg.AsInt(ctx))
		needle = []byte{n}
	} else {
		needle = []byte(needleArg.AsString(ctx))
	}

	if beforeArg != nil {
		beforeNeedle = bool(*beforeArg)
	}

	i := bytes.Index(haystack, needle)
	if i < 0 {
		return phpv.ZBool(false).ZVal(), nil
	}

	var result []byte
	if beforeNeedle {
		result = haystack[0:i]
	} else {
		result = haystack[i:]
	}

	return phpv.ZStr(string(result)), nil
}

// > func string|false strrchr ( string $haystack, string $needle )
func fncStrRChr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystackArg phpv.ZString
	var needleArg *phpv.ZVal
	_, err := core.Expand(ctx, args, &haystackArg, &needleArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	haystack := []byte(haystackArg)
	var needle byte

	if needleArg.GetType() == phpv.ZtInt {
		needle = byte(needleArg.AsInt(ctx))
	} else {
		s := []byte(needleArg.AsString(ctx))
		if len(s) == 0 {
			return phpv.ZFalse.ZVal(), nil
		}
		needle = s[0]
	}

	i := bytes.LastIndexByte(haystack, needle)
	if i < 0 {
		return phpv.ZBool(false).ZVal(), nil
	}

	result := haystack[i:]
	return phpv.ZStr(string(result)), nil
}

// > func string strtr ( string $str , string $from , string $to )
// > func string strtr ( string $str , array $replace_pairs )
func fncStrTr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	nargs := len(args)
	if nargs >= 3 {
		var strArg phpv.ZString
		var from phpv.ZString
		var to phpv.ZString
		_, err := core.Expand(ctx, args, &strArg, &from, &to)
		if err != nil {
			return phpv.ZBool(false).ZVal(), err
		}

		str := []byte(strArg)
		replacer := map[byte]byte{}

		for i := 0; i < min(len(from), len(to)); i++ {
			replacer[from[i]] = to[i]
		}

		for i, c := range str {
			if rc, ok := replacer[c]; ok {
				str[i] = rc
			}
		}

		return phpv.ZStr(string(str)), nil
	} else if nargs >= 2 {
		var strArg phpv.ZString
		var replacePairs *phpv.ZArray
		_, err := core.Expand(ctx, args, &strArg, &replacePairs)
		if err != nil {
			return phpv.ZBool(false).ZVal(), err
		}

		str := []byte(strArg)

		// // The longest keys will be tried first, so sort by length
		keys := replacePairs.ByteArrayKeys(ctx)
		sort.Slice(keys, func(i, j int) bool {
			if len(keys[i]) > len(keys[j]) {
				return true
			}
			return false
		})

		// str will be only iterated only once,
		// so previous replacement will not be overriden
		// by other matches
		var buf bytes.Buffer
		for len(str) > 0 {
			found := false
			for _, key := range keys {
				if !bytes.HasPrefix(str, key) {
					continue
				}

				val, err := replacePairs.OffsetGet(ctx, phpv.ZString(key))
				if err != nil {
					return nil, err
				}

				replacement := []byte(val.String())
				buf.Write(replacement)
				str = str[len(key):]
				found = true
				break
			}
			if !found {
				buf.WriteByte(str[0])
				str = str[1:]
			}
		}

		return phpv.ZStr(buf.String()), nil
	}

	return nil, fmt.Errorf("strtr() expects at least 2 arguments, %d given", nargs)
}

// > func string strip_tags ( string $string, array|string|null $allowed_tags = null )
func fncStripTags(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var allowedTagsArg **phpv.ZVal
	_, err := core.Expand(ctx, args, &str, &allowedTagsArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	allowedTags := map[string]struct{}{}

	if allowedTagsArg != nil && !phpv.IsNull(*allowedTagsArg) {
		arg := *allowedTagsArg
		switch arg.GetType() {
		case phpv.ZtString:
			s := string(arg.AsString(ctx))
			re := regexp.MustCompile(`\<(\w*)>`)
			for _, m := range re.FindAllStringSubmatch(s, -1) {
				if len(m) < 2 {
					continue
				}
				tag := m[1]
				allowedTags[tag] = struct{}{}
			}
		case phpv.ZtArray:
			it := arg.NewIterator()
			for ; it.Valid(ctx); it.Next(ctx) {
				item, err := it.Current(ctx)
				if err != nil {
					return nil, err
				}
				allowedTags[item.String()] = struct{}{}
			}
		}
	}

	// NOTE: This doesn't quite replicate the original
	// strip_tag, as that one has more complex tag stripping state machine.

	tagIndex := -1
	inTag := false
	var buf bytes.Buffer
	for i, c := range str {
		if c == '<' {
			inTag = true
			tagIndex = i
		} else if !inTag {
			buf.WriteRune(c)
		} else if c == '>' {
			inTag = false
			start := min(tagIndex+1, len(str)-1)
			if str[start] == '/' {
				start++
			}
			sub := string(str[start : i+1])
			end := strings.IndexFunc(sub, isNotLetter)

			tagName := sub[0:end]
			_, includeTag := allowedTags[tagName]
			if includeTag {
				buf.Write([]byte(str[tagIndex : i+1]))
			}
		}
	}

	return phpv.ZStr(buf.String()), nil
}

// > func string stripcslashes ( string $string )
func fncStripCSlashes(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	_, err := core.Expand(ctx, args, &str)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	return phpv.ZStr(stripCSlashes(string(str))), nil
}

// > func int|false stripos ( string $haystack, string $needle, int $offset = 0 )
func fncStrIPos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystackArg, needleArg phpv.ZString
	var offsetArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &haystackArg, &needleArg, &offsetArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	offset := 0
	if offsetArg != nil {
		offset = int(*offsetArg)
	}

	haystack := bytesLowerCaseArray([]byte(haystackArg))
	needle := bytesLowerCaseArray([]byte(needleArg))

	if offset >= len(haystack) {
		return phpv.ZBool(false).ZVal(), nil
	}
	if offset > 0 {
		haystack = haystack[offset:]
	}

	result := bytes.Index(haystack, needle)
	if result < 0 {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZInt(result + offset).ZVal(), nil
}

// > func int|false strpos ( string $haystack, string $needle, int $offset = 0 )
func fncStrPos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle []byte
	var offsetArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &haystack, &needle, &offsetArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	// TODO: handle case where needle is int

	offset := 0
	if offsetArg != nil {
		offset = int(*offsetArg)
	}

	if offset >= len(haystack) {
		return phpv.ZBool(false).ZVal(), nil
	}
	if offset > 0 {
		haystack = haystack[offset:]
	}

	result := bytes.Index(haystack, needle)
	if result < 0 {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZInt(result + offset).ZVal(), nil
}

// > func int|false strrpos ( string $haystack, string $needle, int $offset = 0 )
func fncStrRPos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystack, needle []byte
	var offsetArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &haystack, &needle, &offsetArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	// TODO: handle case where needle is int

	offset := 0
	if offsetArg != nil {
		offset = int(*offsetArg)
	}

	if offset >= len(haystack) {
		return phpv.ZBool(false).ZVal(), nil
	}
	if offset >= 0 {
		haystack = haystack[offset:]
		result := bytes.LastIndex(haystack, needle)
		if result < 0 {
			return phpv.ZBool(false).ZVal(), nil
		}

		return phpv.ZInt(result + offset).ZVal(), nil
	} else {
		n := len(haystack) + offset
		result := bytes.LastIndex(haystack[:n], needle)
		if result < 0 {
			return phpv.ZBool(false).ZVal(), nil
		}

		return phpv.ZInt(result).ZVal(), nil
	}
}

// > func string stripslashes ( string $string )
func fncStripSlashes(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	_, err := core.Expand(ctx, args, &str)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}
	result := bytes.ReplaceAll([]byte(str), []byte(`\`), nil)
	return phpv.ZStr(string(result)), nil
}

// > func int strnatcasecmp ( string $string1, string $string2 )
func fncStrNatCaseCmp(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var string1, string2 phpv.ZString
	_, err := core.Expand(ctx, args, &string1, &string2)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	result := natCmp([]byte(string1), []byte(string2), false)

	return phpv.ZInt(result).ZVal(), nil
}

// > func int strnatcmp ( string $string1, string $string2 )
func fncStrNatCmp(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var string1, string2 phpv.ZString
	_, err := core.Expand(ctx, args, &string1, &string2)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	result := natCmp([]byte(string1), []byte(string2), true)

	return phpv.ZInt(result).ZVal(), nil
}

// > func string|false strpbrk ( string $string, string $characters )
func fncStrPbrk(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str, chars phpv.ZString
	_, err := core.Expand(ctx, args, &str, &chars)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	i := bytes.IndexAny([]byte(str), string(chars))
	if i < 0 {
		return phpv.ZBool(false).ZVal(), nil
	}

	return str[i:].ZVal(), nil
}

// > func string strrev ( string $string )
func fncStrRev(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	_, err := core.Expand(ctx, args, &str)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	data := []byte(str)
	for i := 0; i < len(data)/2; i++ {
		j := len(data) - i - 1
		c := data[i]
		data[i] = data[j]
		data[j] = c
	}

	return phpv.ZStr(string(data)), nil
}

const (
	STRTOK_ARG phpv.ZInt = iota
	STRTOK_LAST_INDEX
)

type strTokStateType struct {
	lastString *phpv.ZString
	lastIndex  int
}

// TODO: move to a context state instead of global state
var strTokTempState = strTokStateType{
	lastString: nil,
	lastIndex:  -1,
}

// > func string|false strtok ( string $string, string $token )
func fncStrtok(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var strArg phpv.ZString
	var tokenArg *phpv.ZString
	_, err := core.Expand(ctx, args, &strArg, &tokenArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	index := 0
	startIndex := 0
	var str []byte
	var token []byte
	if tokenArg != nil {
		str = []byte(strArg)
		token = []byte(*tokenArg)
		strTokTempState.lastString = &strArg
	} else {
		token = []byte(strArg)
		if strTokTempState.lastString == nil {
			println("WARN: Both arguments must be provided when starting tokenization")
			return phpv.ZBool(false).ZVal(), nil
		}
		str = []byte(*strTokTempState.lastString)
		startIndex = max(strTokTempState.lastIndex, 0)
	}

	// skip token delimeters
	for ; startIndex < len(str); startIndex++ {
		if !bytes.ContainsRune(token, rune(str[startIndex])) {
			break
		}
	}
	// read until token delimeter
	for index = startIndex; index < len(str); index++ {
		if bytes.ContainsRune(token, rune(str[index])) {
			break
		}
	}

	result := string(str[startIndex:index])

	strTokTempState.lastIndex = index + 1
	if index >= len(str) && result == "" {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZStr(result), nil
}

// > func string substr ( string $string, int $offset, ?int $length = null )
func fncSubstr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var offsetArg phpv.ZInt
	var lengthArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &str, &offsetArg, &lengthArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	offset := int(offsetArg)
	length := len(str)
	if lengthArg != nil {
		length = int(*lengthArg)
	}

	result := substr([]byte(str), offset, length)
	return phpv.ZStr(string(result)), nil
}

// > func int substr_compare ( string $haystack, string $needle, int $offset, ?int $length = null, bool $case_insensitive = false )
func fncSubstrCompare(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystackArg, needleArg phpv.ZString
	var offsetArg phpv.ZInt
	var lengthArg *phpv.ZInt
	var caseInsensitiveArg *phpv.ZBool
	_, err := core.Expand(ctx, args, &haystackArg, &needleArg, &offsetArg, &lengthArg, &caseInsensitiveArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	haystack := []byte(haystackArg)
	needle := []byte(needleArg)
	offset := int(offsetArg)
	haystackLen := len(haystack)
	needleLen := len(needle)
	caseInsensitive := false

	if lengthArg != nil {
		haystackLen = int(*lengthArg)
		needleLen = haystackLen
	}
	if caseInsensitiveArg != nil {
		caseInsensitive = bool(*caseInsensitiveArg)
	}

	str1 := substr([]byte(haystack), offset, haystackLen)
	str2 := substr([]byte(needle), 0, needleLen)
	result := strcmpCommon(str1, str2, !caseInsensitive)

	// strcmp returns the difference between two bytes
	// so explicitly clamp it to -1 to 1
	if result < 0 {
		result = -1
	} else if result > 0 {
		result = 1
	}

	return phpv.ZInt(result).ZVal(), nil
}

// > func int substr_cont ( string $haystack, string $needle, int $offset, ?int $length = null, bool $case_insensitive = false )
func fncSubstrCount(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var haystackArg, needleArg phpv.ZString
	var offsetArg *phpv.ZInt
	var lengthArg *phpv.ZInt
	var caseInsensitiveArg *phpv.ZBool
	_, err := core.Expand(ctx, args, &haystackArg, &needleArg, &offsetArg, &lengthArg, &caseInsensitiveArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	if len(needleArg) == 0 {
		return nil, errors.New("Argument #2 ($needle) cannot be empty")
	}

	haystack := []byte(haystackArg)
	needle := []byte(needleArg)
	offset := 0
	length := len(haystack)
	if offsetArg != nil {
		offset = int(*offsetArg)
	}
	if lengthArg != nil {
		length = int(*lengthArg)
	}

	count := 0
	haystack = substr(haystack, offset, length)
	for len(haystack) > 0 {
		if bytes.Index(haystack, needle) == 0 {
			count++
			haystack = haystack[len(needle):]
		} else {
			haystack = haystack[1:]
		}
	}

	return phpv.ZInt(count).ZVal(), nil
}

// > func mixed substr_replace ( mixed $string , mixed $replacement , mixed $start [, mixed $length ] )
func fncSubstrReplace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var inputArg, replacementArg *phpv.ZVal
	var startArg phpv.ZInt
	var lengthArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &inputArg, &replacementArg, &startArg, &lengthArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	var buf bytes.Buffer
	start := int(startArg)

	getReplacement := func(index int) ([]byte, error) {
		var replacement []byte
		if replacementArg.GetType() == phpv.ZtString {
			replacement = []byte(replacementArg.AsString(ctx))
		} else {
			val, err := replacementArg.Array().OffsetGet(ctx, phpv.ZInt(index).Value())
			if err != nil {
				return nil, err
			}
			replacement = []byte(val.String())
		}
		return replacement, nil
	}

	if inputArg.GetType() == phpv.ZtString {
		input := []byte(inputArg.AsString(ctx))
		length := len(input)
		if lengthArg != nil {
			length = int(*lengthArg)
		}

		replacement, err := getReplacement(0)
		if err != nil {
			return nil, err
		}

		left, _, right := segment(input, start, length)
		buf.Write(left)
		buf.Write(replacement)
		buf.Write(right)

		return phpv.ZStr(buf.String()), nil
	}

	res := inputArg.Dup()

	i := 0
	it := res.NewIterator()
	for ; it.Valid(ctx); it.Next(ctx) {
		v, err := it.Current(ctx)
		if err != nil {
			return nil, err
		}

		input := []byte(v.String())
		length := len(input)
		if lengthArg != nil {
			length = int(*lengthArg)
		}

		replacement, err := getReplacement(i)
		if err != nil {
			return nil, err
		}

		left, _, right := segment(input, start, length)
		buf.Write(left)
		buf.Write(replacement)
		buf.Write(right)
		v.Set(phpv.ZStr(buf.String()))
		buf.Reset()

		i++
	}

	return res, nil
}

// > func string ucfirst ( string $str )
func fncUcFirst(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var strArg phpv.ZString
	_, err := core.Expand(ctx, args, &strArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	str := []rune(strArg)
	var buf bytes.Buffer
	if len(str) > 0 {
		buf.WriteString(strings.ToUpper(string(str[0:1])))
		buf.WriteString(string(str[1:]))
	}

	return phpv.ZStr(buf.String()), nil
}

// > func string ucwords ( string $str [, string $delimiters = " \t\r\n\f\v" ] )
func fncUcWords(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var strArg phpv.ZString
	var delimetersArg *phpv.ZString
	_, err := core.Expand(ctx, args, &strArg, &delimetersArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	str := []byte(strArg)
	delimeters := []byte(" \t\r\n\f\v")
	if delimetersArg != nil {
		delimeters = []byte(*delimetersArg)
	}

	i := 0
	var buf bytes.Buffer

	for i < len(str) {
		// skip delimeters
		for i < len(str) && bytes.ContainsRune(delimeters, rune(str[i])) {
			buf.WriteByte(str[i])
			i++
		}

		if i < len(str) {
			buf.WriteRune(unicode.ToUpper(rune(str[i])))
			i++
		}

		// word chars
		for i < len(str) && !bytes.ContainsRune(delimeters, rune(str[i])) {
			buf.WriteByte(str[i])
			i++
		}
	}

	return phpv.ZStr(buf.String()), nil
}

// > func string wordwrap ( string $str [, int $width = 75 [, string $break = "\n" [, bool $cut = FALSE ]]] )
func fncWordWrap(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var strArg phpv.ZString
	var widthArg *phpv.ZInt
	var breakStrArg *phpv.ZString
	var cutArg *phpv.ZBool
	_, err := core.Expand(ctx, args, &strArg, &widthArg, &breakStrArg, &cutArg)
	if err != nil {
		return phpv.ZBool(false).ZVal(), err
	}

	str := []byte(strArg)
	width := 75
	breakStr := "\n"
	cut := false
	if widthArg != nil {
		width = int(*widthArg)
	}
	if breakStrArg != nil {
		breakStr = string(*breakStrArg)
	}
	if cutArg != nil {
		cut = bool(*cutArg)
	}

	// It wasn't explicitly defined, but word here means
	// anything separated only by " " 0x20 (doesn't include other whitespaces)

	// Beyond the seemingly simple wordwrap
	// as described by the documentation, lies
	// a really quirky, untuitive behaviour when
	// it comes to edge cases, particularly with
	// strings with erratic long sequences of whitespaces.
	// This is the simplest implementation I could
	// muster that faithfully reproduces the original
	// wordwrap implementation. Modify or optimize
	// at your own peril. Just kidding. Or not.
	var buf bytes.Buffer
	if cut {
		i := 0
		for i < len(str) {
			j := min(i+width, len(str)-1)

			for str[j] != ' ' && j > i {
				j--
			}

			if i == j {
				j = i + 1
				for j < min(i+width, len(str)) && str[j] != ' ' {
					j++
				}
				buf.Write(str[i:j])
				if j < len(str)-1 {
					buf.WriteString(breakStr)
				}
				i = j
				continue
			}

			buf.Write(str[i:j])

			if j >= len(str) {
				break
			}
			if j < len(str)-1 {
				buf.WriteString(breakStr)
			}
			i = j
			if str[i] == ' ' {
				i++
			}
		}
	} else {
		i := 0
		lastBreak := -1
		for i < len(str) {
			j := min(i+width, len(str))
			if j >= len(str) {
				buf.Write(str[i:j])
				break
			}

			for str[j] != ' ' && j > i {
				j--
			}

			if i == j {
				for j < len(str) && str[j] != ' ' {
					j++
				}
			}

			buf.Write(str[i:j])
			if j >= len(str) {
				break
			}

			if str[j] == ' ' && j-lastBreak > 1 {
				buf.WriteString(breakStr)
				lastBreak = j
			} else {
				buf.WriteByte(' ')
			}

			i = j + 1
		}
	}

	return phpv.ZStr(buf.String()), nil
}

func strReplaceCommon(ctx phpv.Context, args []*phpv.ZVal, caseSensitive bool) (*phpv.ZVal, error) {
	var search, replace, subject *phpv.ZVal
	var count core.Ref[*phpv.ZInt]
	_, err := core.Expand(ctx, args, &search, &replace, &subject, &count)
	if err != nil {
		return nil, err
	}

	if count.Value == nil {
		// avoid crash
		var n phpv.ZInt
		count.Set(ctx, &n)
	}

	if subject.GetType() == phpv.ZtArray {
		res := subject.Dup()
		it := res.NewIterator()

		for {
			if !it.Valid(ctx) {
				break
			}

			v, err := it.Current(ctx)
			if err != nil {
				return nil, err
			}

			err = v.CastTo(ctx, phpv.ZtString)
			if err != nil {
				return nil, err
			}

			vs := v.Value().(phpv.ZString)

			vs, err = doStrReplace(ctx, vs, search, replace, count, caseSensitive)
			if err != nil {
				return nil, err
			}

			v.Set(vs.ZVal())
			it.Next(ctx)
		}

		return res, nil
	}

	subject, err = subject.As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	vs := subject.Value().(phpv.ZString)

	vs, err = doStrReplace(ctx, vs, search, replace, count, caseSensitive)
	if err != nil {
		return nil, err
	}

	return vs.ZVal(), nil
}

func doStrReplace(
	ctx phpv.Context,
	subject phpv.ZString,
	search, replace *phpv.ZVal,
	count core.Ref[*phpv.ZInt],
	caseSensitive bool,
) (phpv.ZString, error) {
	if search.GetType() == phpv.ZtArray {
		if replace.GetType() == phpv.ZtArray {
			it1 := search.NewIterator()
			it2 := replace.NewIterator()

			for {
				if !it1.Valid(ctx) {
					// end of source,
					return subject, nil
				}
				from, err := it1.Current(ctx)
				if err != nil {
					return subject, err
				}

				from, err = from.As(ctx, phpv.ZtString)
				if err != nil {
					return subject, err
				}

				from_b := []byte(from.AsString(ctx))

				cnt := bytesCount([]byte(subject), from_b, caseSensitive)
				if cnt == 0 {
					// nothing to replace, skip
					it1.Next(ctx)
					it2.Next(ctx)
					continue
				}

				var to *phpv.ZVal
				if it2.Valid(ctx) {
					to, err = it2.Current(ctx)
					if err != nil {
						return subject, err
					}
				}

				to, err = to.As(ctx, phpv.ZtString)
				if err != nil {
					return subject, err
				}

				to_b := []byte(to.AsString(ctx))
				subject = phpv.ZString(bytesReplace([]byte(subject), from_b, to_b, cnt, caseSensitive))

				n := phpv.ZInt(cnt)
				count.Set(ctx, &n)

				it1.Next(ctx)
				it2.Next(ctx)
			}
		}

		var err error
		replace, err = replace.As(ctx, phpv.ZtString)
		if err != nil {
			return subject, err
		}

		to_b := []byte(replace.AsString(ctx))

		it1 := search.NewIterator()

		for {
			if !it1.Valid(ctx) {
				// end of source,
				return subject, nil
			}
			from, err := it1.Current(ctx)
			if err != nil {
				return subject, err
			}

			from, err = from.As(ctx, phpv.ZtString)
			if err != nil {
				return subject, err
			}

			from_b := []byte(from.AsString(ctx))

			cnt := bytesCount([]byte(subject), from_b, caseSensitive)
			if cnt == 0 {
				// nothing to replace, skip
				it1.Next(ctx)
				continue
			}

			subject = phpv.ZString(bytesReplace([]byte(subject), from_b, to_b, cnt, caseSensitive))

			n := phpv.ZInt(cnt)
			count.Set(ctx, &n)

			it1.Next(ctx)
		}
	}

	search, err := search.As(ctx, phpv.ZtString)
	if err != nil {
		return subject, err
	}

	from_b := []byte(search.AsString(ctx))

	cnt := bytesCount([]byte(subject), from_b, caseSensitive)
	if cnt == 0 {
		return subject, nil
	}

	replace, err = replace.As(ctx, phpv.ZtString)
	if err != nil {
		return subject, err
	}

	to_b := []byte(replace.AsString(ctx))
	subject = phpv.ZString(bytesReplace([]byte(subject), from_b, to_b, cnt, caseSensitive))
	n := phpv.ZInt(cnt)
	count.Set(ctx, &n)

	return subject, err
}

// TODO: move to another file or package
func bytesReplace(s, old, new []byte, count int, caseSensitive bool) []byte {
	if caseSensitive {
		return bytes.Replace(s, old, new, count)
	}

	if len(s) == 0 || len(old) == 0 {
		return s
	}

	replaced := 0
	var buf bytes.Buffer
	for i := 0; i < len(s)-len(old)+1; i++ {
		if count > 0 && replaced >= count {
			buf.Write(s[i:])
			break
		}

		match := true
		for j := 0; j < len(old); j++ {
			c1 := bytesLowerCase(s[i+j])
			c2 := bytesLowerCase(old[j])
			if c1 != c2 {
				match = false
				break
			}
		}

		if match {
			buf.Write(new)
			replaced++
			i += len(old) - 1
		} else {
			buf.WriteByte(s[i])
		}
	}

	return buf.Bytes()
}

func bytesUpperCase(b byte) byte {
	return byte(unicode.ToUpper(rune(b)))
}

func bytesLowerCase(b byte) byte {
	return byte(unicode.ToLower(rune(b)))
}

func bytesLowerCaseArray(bs []byte) []byte {
	var result []byte
	for _, b := range bs {
		b = bytesLowerCase(b)
		result = append(result, b)
	}
	return result
}

func bytesCount(s, sep []byte, caseSensitive bool) int {
	if caseSensitive {
		return bytes.Count(s, sep)
	}

	if len(s) == 0 || len(sep) == 0 {
		return 0
	}

	replaced := 0
	for i := 0; i < len(s)-len(sep)+1; i++ {
		match := true
		for j := 0; j < len(sep); j++ {
			c1 := bytesLowerCase(s[i+j])
			c2 := bytesLowerCase(sep[j])
			if c1 != c2 {
				match = false
				break
			}
		}

		if match {
			replaced++
			i += len(sep) - 1
		}
	}

	return replaced
}

func substr(strArg []byte, offset, length int) []byte {
	var start, end int
	if offset < 0 {
		start = max(0, len(strArg)+offset)
	} else {
		start = min(offset, len(strArg))
	}
	if length < 0 {
		end = max(0, len(strArg)+length)
	} else {
		end = min(start+length, len(strArg))
	}

	if start == 0 && end == len(strArg) {
		return strArg
	}

	var result []byte
	if start <= end {
		result = strArg[start:end]
	}

	return result
}

func segment(strArg []byte, offset, length int) ([]byte, []byte, []byte) {
	var start, end int
	if offset < 0 {
		start = max(0, len(strArg)+offset)
	} else {
		start = min(offset, len(strArg))
	}
	if length < 0 {
		end = max(0, len(strArg)+length)
	} else {
		end = min(start+length, len(strArg))
	}

	if start == 0 && end == len(strArg) {
		return nil, strArg, nil
	}

	left := strArg[0:start]
	mid := strArg[start:end]
	right := strArg[end:]

	return left, mid, right
}

func isNotLetter(c rune) bool {
	return !unicode.IsLetter(c)
}

func strcmpCommon(str1, str2 []byte, caseSensitive bool) int {
	for i := 0; i < max(len(str1), len(str2)); i++ {
		var c1, c2 byte
		if i < len(str1) {
			c1 = str1[i]
		}
		if i < len(str2) {
			c2 = str2[i]
		}

		if !caseSensitive {
			c1 = bytesLowerCase(c1)
			c2 = bytesLowerCase(c2)
		}

		if c1 < c2 {
			return int(c1) - int(c2)
		}
		if c1 > c2 {
			return int(c1) - int(c2)
		}
	}
	return 0
}

func escapeByte(b byte) []byte {
	switch b {
	case '\n':
		return []byte(`\n`)
	case '\r':
		return []byte(`\r`)
	case '\a':
		return []byte(`\a`)
	case '\t':
		return []byte(`\t`)
	case '\v':
		return []byte(`\v`)
	case '\b':
		return []byte(`\b`)
	case '\f':
		return []byte(`\f`)
	}

	return []byte{'\\', b}
}

func stripCSlashes(str string) string {
	var buf bytes.Buffer
	for i := 0; i < len(str); i++ {
		if str[i] != '\\' {
			buf.WriteByte(str[i])
			continue
		}

		i++
		if i >= len(str) {
			break
		}

		hex := false
		unescaped := true

		switch str[i] {
		case 'n':
			buf.WriteString("\n")
		case 'r':
			buf.WriteString("\r")
		case 'a':
			buf.WriteString("\a")
		case 't':
			buf.WriteString("\t")
		case 'v':
			buf.WriteString("\v")
		case 'b':
			buf.WriteString("\b")
		case 'f':
			buf.WriteString("\f")
		default:
			unescaped = false
		}

		if unescaped {
			continue
		}

		if str[i] == 'x' {
			hex = true
			i++
			if i >= len(str) {
				buf.WriteByte('x')
				break
			}
		}

		readNum := unicode.IsNumber(rune(str[i]))
		if !readNum {
			buf.WriteByte(str[i])
		} else if readNum {
			base := 8
			length := 3
			if hex {
				base = 16
				length = 2
			}

			j := i
			for j-i <= length-1 && unicode.IsNumber(rune(str[j])) {
				j++
				if j >= len(str) {
					break
				}
			}

			if j > i {
				n, err := strconv.ParseInt(string(str[i:j]), base, 8)
				if err == nil {
					buf.WriteByte(byte(n))
				}
				i = j - 1
			}

		}
	}

	return buf.String()
}

func safeIndex[T any](xs []T, index int, defaultVal ...T) T {
	var x T
	if index >= 0 && index < len(xs) {
		x = xs[index]
	} else if len(defaultVal) > 0 {
		x = defaultVal[0]
	}
	return x
}

// example:
// expandCharacterRanges("abc..ghx..z") == "abcdefghxyz"
func expandCharacterRanges(str string) string {
	var buf bytes.Buffer
	bytes := []byte(str)
	for i := 0; i < len(bytes); i++ {
		if core.Idx(bytes, i+1) == '.' && core.Idx(bytes, i+2) == '.' && i+3 < len(bytes) {
			from := bytes[i]
			to := bytes[i+3]
			for c := from; c <= to; c++ {
				buf.WriteByte(c)
			}
		} else {
			buf.WriteByte(bytes[i])
		}
	}
	return buf.String()
}
