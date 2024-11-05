package standard

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

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
	var g = phpctx.NewGlobal(context.Background(), phpctx.NewProcess("test"))
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
	var g = phpctx.NewGlobal(context.Background(), phpctx.NewProcess("test"))
	val := must(fn(g, phpArgs(args...)))
	return val
}

func testOutput(fn phpFunc, args ...any) {
	var g = phpctx.NewGlobal(context.Background(), phpctx.NewProcess("test"))
	ret := testRun(fn, args...)
	switch ret.GetType() {
	case phpv.ZtArray:
		it := ret.NewIterator()
		print("[")
		for ; it.Valid(g); it.Next(g) {
			fmt.Printf("%v => %v  ", must(it.Key(g)), must(it.Current(g)))
		}
		println("]")
	default:
		s := fmt.Sprintf("%+v", ret)
		fmt.Printf("《%s》len=%v\n", s, len(s))
	}
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
}
