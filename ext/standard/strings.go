package standard

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

var (
	addSlashesReplacer = strings.NewReplacer(
		`'`, `\'`,
		`"`, `\"`,
		`\`, `\\`,
		"\000", "\\\000",
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
)

// > func string addslashes( string $string )
func fncStrAddSlashes(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString

	_, err := core.Expand(ctx, args, &str)
	if err != nil {
		return nil, err
	}

	result := addSlashesReplacer.Replace(string(str))

	return phpv.ZString(result).ZVal(), nil
}

// > func string chr( int $codepoint )
func fncStrChr(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var codepoint phpv.ZInt

	_, err := core.Expand(ctx, args, &codepoint)
	if err != nil {
		return nil, err
	}

	codepoint &= 255

	return phpv.ZString(byte(codepoint)).ZVal(), nil
}

// > func string chunk_split( string $string, int $length = 76, string $separator = "\r\n" )
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

// >  func array|string count_chars( string $string, int $mode = 0 )
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

// >  func array explode( string $separator, string $string, int $limit = PHP_INT_MAX )
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

// >  func string implode( string $separator, array $array )
func fncStrImplode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var sep phpv.ZString
	var array *phpv.ZArray

	_, err := core.Expand(ctx, args, &sep, &array)
	if err != nil {
		return nil, err
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

// > func string lcfirst(string $string)
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

// > func string ltrim( string $string, string $characters = " \n\r\t\v\x00" )
func fncStrLtrim(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var charsArg *phpv.ZString

	_, err := core.Expand(ctx, args, &str, &charsArg)
	if err != nil {
		return nil, err
	}

	chars := " \n\r\t\v\x00"
	if charsArg != nil {
		chars = string(*charsArg)
	}

	result := strings.TrimLeft(string(str), chars)
	return phpv.ZString(result).ZVal(), nil
}

// > func string rtrim( string $string, string $characters = " \n\r\t\v\x00" )
func fncStrRtrim(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var charsArg *phpv.ZString

	_, err := core.Expand(ctx, args, &str, &charsArg)
	if err != nil {
		return nil, err
	}

	chars := " \n\r\t\v\000"
	if charsArg != nil {
		chars = string(*charsArg)
	}

	result := strings.TrimRight(string(str), chars)
	return phpv.ZString(result).ZVal(), nil
}

// > func string md5( string $string, bool $binary = false )
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

// > func string|false md5_file( string $filename, bool $binary = false )
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

// > func string nl2br( string $string, bool $use_xhtml = true )
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

	for {
		base := math.Floor(math.Log10(n))
		x := int(n / math.Pow10(int(base)))

		buf.WriteString(strconv.Itoa(x))
		if int(base)%3 == 0  && base != 0{
			buf.WriteString(thousandsSep)
		}

		n = float64(int(n) % int(math.Pow10(int(base))))

		if n == 0 {
			break
		}
	}

	if fac > 0 && decimals > 0 {
		n := math.Round(fac * math.Pow10(decimals))
		buf.WriteString(decimalSep)
		buf.WriteString(strconv.Itoa(int(n)))
	}

	return phpv.ZStr(buf.String()), nil
}

// > func mixed str_replace ( mixed $search , mixed $replace , mixed $subject [, int &$count ] )
func stdStrReplace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var search, replace, subject *phpv.ZVal
	var count *phpv.ZInt

	_, err := core.Expand(ctx, args, &search, &replace, &subject, &count)
	if err != nil {
		return nil, err
	}

	if count == nil {
		// avoid crash
		count = new(phpv.ZInt)
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

			vs, err = doStrReplace(ctx, vs, search, replace, count)
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

	vs, err = doStrReplace(ctx, vs, search, replace, count)
	if err != nil {
		return nil, err
	}

	return vs.ZVal(), nil
}

func doStrReplace(ctx phpv.Context, subject phpv.ZString, search, replace *phpv.ZVal, count *phpv.ZInt) (phpv.ZString, error) {
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

				cnt := bytes.Count([]byte(subject), from_b)
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
				subject = phpv.ZString(bytes.Replace([]byte(subject), from_b, to_b, cnt))
				*count += phpv.ZInt(cnt)

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

			cnt := bytes.Count([]byte(subject), from_b)
			if cnt == 0 {
				// nothing to replace, skip
				it1.Next(ctx)
				continue
			}

			subject = phpv.ZString(bytes.Replace([]byte(subject), from_b, to_b, cnt))
			*count += phpv.ZInt(cnt)

			it1.Next(ctx)
		}
	}

	search, err := search.As(ctx, phpv.ZtString)
	if err != nil {
		return subject, err
	}

	from_b := []byte(search.AsString(ctx))

	cnt := bytes.Count([]byte(subject), from_b)
	if cnt == 0 {
		return subject, nil
	}

	replace, err = replace.As(ctx, phpv.ZtString)
	if err != nil {
		return subject, err
	}

	to_b := []byte(replace.AsString(ctx))
	subject = phpv.ZString(bytes.Replace([]byte(subject), from_b, to_b, cnt))
	*count += phpv.ZInt(cnt)

	return subject, err
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

// > func string strtolower ( string $string )
func fncStrToLower(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var s phpv.ZString
	_, err := core.Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	return s.ToLower().ZVal(), nil
}
