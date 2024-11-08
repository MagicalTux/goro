package standard

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

var g = phpctx.NewGlobal(context.Background(), phpctx.NewProcess("test"))

func must[T any](x T, err error) T {
	if err != nil {
		panic(err)
	}
	return x
}

type phpFunc func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error)

func phpArgs(args ...any) []*phpv.ZVal {
	var vals []*phpv.ZVal
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			vals = append(vals, phpv.ZStr(v))
		case int:
			vals = append(vals, phpv.ZInt(v).ZVal())
		case bool:
			vals = append(vals, phpv.ZBool(v).ZVal())

		case float32:
			vals = append(vals, phpv.ZFloat(v).ZVal())
		case float64:
			vals = append(vals, phpv.ZFloat(v).ZVal())

		case *phpv.ZArray:
			vals = append(vals, v.ZVal())
		case *phpv.ZVal:
			vals = append(vals, v)
		default:
			panic(fmt.Sprintf("unhandled type: %+v", reflect.TypeOf(arg)))
		}
	}
	return vals
}

func phpArray(args ...any) *phpv.ZArray {
	arr := phpv.NewZArray()
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			arr.OffsetSet(g, nil, phpv.ZStr(v))
		case int:
			arr.OffsetSet(g, nil, phpv.ZInt(v).ZVal())
		case bool:
			arr.OffsetSet(g, nil, phpv.ZBool(v).ZVal())
		case float32:
			arr.OffsetSet(g, nil, phpv.ZFloat(v).ZVal())
		case float64:
			arr.OffsetSet(g, nil, phpv.ZFloat(v).ZVal())

		case *phpv.ZVal:
			arr.OffsetSet(g, nil, v)
		default:
			panic(fmt.Sprintf("unhandled array item: %+v", arg))
		}
	}
	return arr
}

func testRun(fn phpFunc, args ...any) *phpv.ZVal {
	val := must(fn(g, phpArgs(args...)))
	return val
}

func testOutput(fn phpFunc, args ...any) *phpctx.Global {
	ret := testRun(fn, args...)
	switch ret.GetType() {
	case phpv.ZtArray:
		stdFuncVarDump(g, phpArgs(ret))
	default:
		s := fmt.Sprintf("%+v", ret)
		fmt.Printf("《%s》len=%v\n", s, len(s))
	}
	return g
}

func TestStrFunctions(t *testing.T) {
	println("# add_slashes")
	testOutput(fncStrAddSlashes, `it's a  string: "test"`)

	println("# chr")
	testOutput(fncStrChr, 97)
	testOutput(fncStrChr, 65)

	println("# chunk_split")
	testOutput(fncStrChunkSplit, "123456789", 3, "x")

	println("# str_count mode 0")
	testOutput(fncStrCountChars, "abcdeabc")
	println("# str_count mode 1")
	testOutput(fncStrCountChars, "\00012abcdeabc", 1)
	println("# str_count mode 2")
	testOutput(fncStrCountChars, "abcdeabc", 2)
	println("# str_count mode 3")
	testOutput(fncStrCountChars, "abcdeabc", 3)
	println("# str_count mode 4")
	testOutput(fncStrCountChars, "abcdeabc", 4)

	println("# explode")
	testOutput(fncStrExplode, ",", "a,b,c,d", -3)

	println("# implode")
	testOutput(fncStrImplode, ",", phpArray("a", "b", "c"))
	testOutput(fncStrImplode, "", phpArray("a", "b", "c"))
	testOutput(fncStrImplode, "x", phpArray())
	testOutput(fncStrImplode, "x", phpArray("1"))

	println("# lcFirst")
	testOutput(fncStrLcFirst, "test")
	testOutput(fncStrLcFirst, "Test")
	testOutput(fncStrLcFirst, "TEST")
	testOutput(fncStrLcFirst, "124")
	testOutput(fncStrLcFirst, "ZzzZ")

	println("# ltrim")
	testOutput(fncStrLtrim, "\x00\000  \t\ntrim left ")
	println("# rtrim")
	testOutput(fncStrRtrim, "\x00\000  \ttrim right   \x00\r\n")

	println("# md5")
	testOutput(fncStrMd5, "hello world")
	testOutput(fncStrMd5, "hello world", true)

	println("# md5_file")
	testOutput(fncStrMd5File, "base.go", false)
	testOutput(fncStrMd5File, "base.go", true)

	println("# nl2br")
	testOutput(fncStrNewLine2Br, "one\ntwo\r\nthree\n\rfour\n")
	println()
	testOutput(fncStrNewLine2Br, "one\ntwo\r\nthree\n\rfour\n", false)

	println("# number_format")
	// 123 456 789
	testOutput(fncStrNumberFormat, 123456789.12345, 2)
	testOutput(fncStrNumberFormat, 123456789.12345, 3)
	testOutput(fncStrNumberFormat, 123456789.12345, 0)
	testOutput(fncStrNumberFormat, 23456789.12345, 0)
	testOutput(fncStrNumberFormat, 3456789.12345, 0)
	testOutput(fncStrNumberFormat, 456789.12345, 0)
	testOutput(fncStrNumberFormat, 89.12345, 0)
	testOutput(fncStrNumberFormat, 9.12345, 2)
	testOutput(fncStrNumberFormat, 123456789.12345, 2, "#", "@")

	println("# ord")
	testOutput(fncStrOrd, "a")
	testOutput(fncStrOrd, "A")
	testOutput(fncStrOrd, "")
	testOutput(fncStrOrd, "👍")

	println("# parse_str")
	testOutput(fncStrParseStr, "first=value&arr[]=foo+bar&arr[]=baz")
	testOutput(fncStrParseStr, "first[]=value&arr[]=foo+bar&arr[]=baz")
	testOutput(fncStrParseStr, "first=value&first=value2&arr[]=foo+bar&arr[]=baz")
	testOutput(fncStrParseStr, "")
	testOutput(fncStrParseStr, "x")

	println("# quote_meta")
	testOutput(fncStrQuoteMeta, `quote these: .\()*?$`)

	println("# sha1")
	testOutput(fncStrSha1, `hello world`)
	testOutput(fncStrSha1, `hello world`, true)

	println("# sha1_file")
	testOutput(fncStrSha1, `base.go`, false)
	testOutput(fncStrSha1, `base.go`, true)

	println("# str_contains")
	testOutput(fncStrContains, "the widdle fox flew over the electric fence", "fox")
	testOutput(fncStrContains, "the widdle fox flew over the electric fence", "little")

	println("# str_ends_with")
	testOutput(fncStrEndsWith, "aaabbb", "a")
	testOutput(fncStrEndsWith, "aaabbb", "b")
	testOutput(fncStrEndsWith, "aaabbb", "bb")

	println(" #str_getcsv")
	testOutput(fncStrGetCsv, `a,b,c,,d,`)
	testOutput(fncStrGetCsv, `ab,  "bx",cx","de`)
	testOutput(fncStrGetCsv, `xy,"ab,cd"`)
	testOutput(fncStrGetCsv, `"ab,xy`)
	testOutput(fncStrGetCsv, `\"ab,xy`)
	testOutput(fncStrGetCsv, `ab,"xy\",we",ef`)
	testOutput(fncStrGetCsv, ``)
	testOutput(fncStrGetCsv, `x,`)
	testOutput(fncStrGetCsv, `x,y`)
	testOutput(fncStrGetCsv, `,`)

	println(" #str_replace")
	testOutput(stdStrReplace, `cde`, `x`, `AbcdeF`)
	testOutput(stdStrReplace, `cde`, `x`, `AbCdeF`)
	testOutput(stdStrReplace, `cdef`, `x`, `Abcdef`)

	println(" #str_ireplace")
	testOutput(stdStrIReplace, `cde`, `x`, `AbcdeF`)
	testOutput(stdStrIReplace, `ef`, `x`, `AbcdeF`)

	println(" #str_pad")
	testOutput(fncStrPad, `fo`, 9, `x`)
	testOutput(fncStrPad, `fo`, 9, `x`, STR_PAD_RIGHT)
	testOutput(fncStrPad, `fo`, 9, `x`, STR_PAD_BOTH)

	println(" #str_repeat")
	testOutput(fncStrRepeat, `wa`, 5)
	testOutput(fncStrRepeat, `wa`, 5)
	testOutput(fncStrRepeat, `a`, 0)
	testOutput(fncStrRepeat, `a`, 1)
	// testOutput(fncStrRepeat, `a`, -1)

	println(" #str_shuffle")
	testOutput(fncStrShuffle, `abcdef`)

	println(" #str_split")
	testOutput(fncStrSplit, `abcdefghijk`, 1)
	testOutput(fncStrSplit, `abcdefghijk`, 3)
	testOutput(fncStrSplit, `abcdefghijk`, 100)

	println(" #str_starts_with")
	testOutput(fncStrStartsWith, `abcdefghijk`, `abc`)

	println(" #str_word_count")
	testOutput(fncStrWordCount, `one two three`)
	testOutput(fncStrWordCount, `one two three`, 1)
	testOutput(fncStrWordCount, `one two three`, 2)
	testOutput(fncStrWordCount, `one two three 112`, 2, "1")

	println(" #strcasecmp")
	testOutput(fncStrCaseCmp, `a`, `A`)
	testOutput(fncStrCaseCmp, `a`, `Aa`)
	testOutput(fncStrCaseCmp, `aAa`, `Aa`)
	testOutput(fncStrCaseCmp, `aAa`, `b`)
	testOutput(fncStrCaseCmp, `b`, `aaab`)
	testOutput(fncStrCaseCmp, ``, ``)

	println(" #strcoll")
	testOutput(fncStrColl, `a`, `A`)
	testOutput(fncStrColl, `a`, `Aa`)
	testOutput(fncStrColl, `aAa`, `Aa`)
	testOutput(fncStrColl, `aAa`, `b`)
	testOutput(fncStrColl, `b`, `aaab`)
	testOutput(fncStrColl, ``, ``)

	println(" #substr")
	testOutput(fncSubstr, `abcde`, -1, 1)
	testOutput(fncSubstr, `abcde`, 1, -1)
	testOutput(fncSubstr, `abcde`, 10000, 1000)

	println(" #strcspn")
	testOutput(fncStrCspn, `banana`, `a`)
	testOutput(fncStrCspn, `banana`, `abcd`)
	testOutput(fncStrCspn, `banana`, `z`)
	testOutput(fncStrCspn, `abcdhelloabcd`, `a`, -9)
	testOutput(fncStrCspn, `abcdhelloabcd`, `a`, -9, -5)

	println(" #strstr")
	testOutput(fncStrStr, `banana@gorilla.com`, `@`)
	testOutput(fncStrStr, `banana@gorilla.com`, `@`, true)

	println(" #strip_tags")
	testOutput(fncStripTags, `>test`, `<p>`)
	testOutput(fncStripTags, `<foo>test<p  blah><a><b>`, `<p><a>`)
	testOutput(fncStripTags, `<hello><em>there`, phpArray("em"))

	println(" #stripcslashes")
	testOutput(fncStripCSlashes, `test \x6159 \1334 \' \" \z \w \x`)
	testOutput(fncStripCSlashes, `\1`)
	testOutput(fncStripCSlashes, `\0`)
	testOutput(fncStripCSlashes, `I\'d have a coffee.\nNot a problem.`)

	println(" #stripos")
	testOutput(fncStrIPos, `abcdefab`, `ab`)
	testOutput(fncStrIPos, `Abcdefab`, `ab`)
	testOutput(fncStrIPos, `Abcdefab`, `aB`)
	testOutput(fncStrIPos, `abcdefab`, `ab`, 3)
	testOutput(fncStrIPos, `abcdefab`, `AB`, 3)
	testOutput(fncStrIPos, `abcdefab`, `AB`, 10000)

	println(" #strpos")
	testOutput(fncStrPos, `abcdefab`, `ab`)
	testOutput(fncStrPos, `Abcdefab`, `ab`)
	testOutput(fncStrPos, `Abcdefab`, `aB`)
	testOutput(fncStrPos, `abcdefab`, `ab`, 3)
	testOutput(fncStrPos, `abcdefab`, `AB`, 3)
	testOutput(fncStrPos, `abcdefab`, `AB`, 3, 1000)

	println(" #stripslashes")
	testOutput(fncStripSlashes, `foo\' \bar 000`)

	println(" #stristr")
	testOutput(fncStrIStr, `banana@gOrilla.com`, `@GO`)
	testOutput(fncStrIStr, `banana@GOrilla.com`, `@Go`, true)

	println(" #strnatcasecmp")
	testOutput(fncStrNatCaseCmp, `Apple`, `Banana`)
	testOutput(fncStrNatCaseCmp, `Banana`, `Apple`)
	testOutput(fncStrNatCaseCmp, `apple`, `Apple`)

	println(" #strpbrk")
	testOutput(fncStrPbrk, `This is a Simple text.`, `mi`)
	testOutput(fncStrPbrk, `This is a Simple text.`, `S`)

	println(" #strrev")
	testOutput(fncStrRev, ``)
	testOutput(fncStrRev, `a`)
	testOutput(fncStrRev, `ab`)
	testOutput(fncStrRev, `abc`)
	testOutput(fncStrRev, `abcdef`)
	testOutput(fncStrRev, `:D 😭`)

	println(" #strtok")
	token := `/d`
	testOutput(fncStrtok, `abc/def/ghi`, token)
	testOutput(fncStrtok, token)
	testOutput(fncStrtok, token)
	testOutput(fncStrtok, token)
	testOutput(fncStrtok, token)
	testOutput(fncStrtok, token)

	println("# substr_compare")
	testOutput(fncSubstrCompare, "abcde", "bc", 1, 2); // 0
	testOutput(fncSubstrCompare, "abcde", "de", -2, 2); // 0
	testOutput(fncSubstrCompare, "abcde", "bcg", 1, 2, false); // 0
	testOutput(fncSubstrCompare, "abcde", "BC", 1, 2, true); // 0
	testOutput(fncSubstrCompare, "abcde", "bc", 1, 3); // 1
	testOutput(fncSubstrCompare, "abcde", "cd", 1, 2); // -1
	testOutput(fncSubstrCompare, "abcde", "abc", 5, 1); // warning
}
