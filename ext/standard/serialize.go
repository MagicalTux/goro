package standard

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type deserializer struct {
	allowedClasses  map[phpv.ZString]struct{}
	allowAllClasses bool
}

// > func string serialize ( mixed $value )
func fncSerialize(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var value *phpv.ZVal
	_, err := core.Expand(ctx, args, &value)
	if err != nil {
		return nil, err
	}
	result, err := serialize(ctx, value)
	return phpv.ZStr(result), err
}

// > func mixed unserialize ( string $str [, array $options ] )
func fncUnserialize(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var optionsArg core.Optional[*phpv.ZVal]
	_, err := core.Expand(ctx, args, &str, &optionsArg)
	if err != nil {
		return nil, err
	}

	deserializer := &deserializer{
		allowAllClasses: true,
		allowedClasses:  map[phpv.ZString]struct{}{},
	}
	if optionsArg.HasArg() {
		options := optionsArg.Get().AsArray(ctx)
		arg, _ := options.OffsetGet(ctx, phpv.ZString("allowed_classes"))
		switch arg.GetType() {
		case phpv.ZtArray:
			deserializer.allowAllClasses = false
			for _, className := range arg.AsArray(ctx).Iterate(ctx) {
				deserializer.allowedClasses[className.AsString(ctx)] = struct{}{}
			}
		default:
			deserializer.allowAllClasses = bool(arg.AsBool(ctx))
		}
	}

	result, _, err := deserializer.parse(ctx, string(str))
	return result, err
}

func serialize(ctx phpv.Context, value *phpv.ZVal) (string, error) {
	var result string
	switch value.GetType() {
	case phpv.ZtNull:
		result = "N;"
	case phpv.ZtBool:
		switch value.AsBool(ctx) {
		case true:
			result = "b:1;"
		case false:
			result = "b:0;"
		}
	case phpv.ZtInt:
		n := value.AsInt(ctx)
		result = "i:" + strconv.FormatInt(int64(n), 10) + ";"
	case phpv.ZtFloat:
		n := value.AsFloat(ctx)
		p := ctx.GetConfig("serialize_precision", phpv.ZInt(14).ZVal()).AsInt(ctx)
		result = "d:" + strconv.FormatFloat(float64(n), 'G', int(p), 64) + ";"
	case phpv.ZtString:
		s := value.AsString(ctx)
		result = fmt.Sprintf(`s:%d:"%s";`, len(s), s)
	case phpv.ZtArray:
		arr := value.AsArray(ctx)
		count := strconv.FormatInt(int64(arr.Count(ctx)), 10)

		var buf bytes.Buffer
		buf.WriteString("a:")
		buf.WriteString(count)
		buf.WriteString(":{")
		for k, v := range arr.Iterate(ctx) {
			sub, err := serialize(ctx, k)
			if err != nil {
				return "", err
			}
			buf.WriteString(sub)
			sub, err = serialize(ctx, v)
			if err != nil {
				return "", err
			}
			buf.WriteString(sub)
		}
		buf.WriteString("}")
		result = buf.String()
	case phpv.ZtObject:
		obj := value.AsObject(ctx)
		var props *phpv.ZArray
		if method, ok := obj.GetClass().GetMethod(phpv.ZString("__sleep")); ok {
			val, err := ctx.Call(ctx, method.Method, nil, obj)
			if err != nil {
				return "", err
			}
			props = val.AsArray(ctx)
		}

		var buf bytes.Buffer
		propCount := 0

		if props != nil {
			for _, prop := range props.Iterate(ctx) {
				sub, err := serialize(ctx, prop)
				if err != nil {
					return "", err
				}
				buf.WriteString(sub)

				v := obj.HashTable().GetString(prop.AsString(ctx))
				sub, err = serialize(ctx, v)
				if err != nil {
					return "", err
				}
				buf.WriteString(sub)
				propCount++
			}
		} else {
			for prop := range obj.IterProps(ctx) {
				sub, err := serialize(ctx, prop.VarName.ZVal())
				if err != nil {
					return "", err
				}
				buf.WriteString(sub)

				v := obj.HashTable().GetString(prop.VarName)
				sub, err = serialize(ctx, v)
				if err != nil {
					return "", err
				}
				buf.WriteString(sub)

				propCount++
			}
		}

		contents := buf.String()
		buf.Reset()
		className := string(obj.GetClass().GetName())
		buf.WriteString(fmt.Sprintf(`O:%d:"%s":%d:`, len(className), className, propCount))
		buf.WriteString("{")
		buf.WriteString(contents)
		buf.WriteString("}")

		result = buf.String()

	}

	return result, nil
}

type unserializeError struct {
	offset int
	length int
}

func (ue *unserializeError) Error() string {
	return fmt.Sprintf("Error at offset %d of %d bytes", ue.offset, ue.length)
}

func (d *deserializer) parse(ctx phpv.Context, str string, offsetArg ...int) (result *phpv.ZVal, nextOffset int, err error) {
	offset := 0
	if len(offsetArg) > 0 {
		offset = offsetArg[0]
	}
	if len(str) == 0 {
		return phpv.ZNULL.ZVal(), offset, nil
	}
	readError := &unserializeError{offset, len(str)}

	if len(str) < offset+2 || (str[offset] != 'N' && str[offset+1] != ':') {
		return nil, offset, readError
	}

	i := offset + 2 // points the char after the first colon

	indexOf := func(haystack, needle string, from int) int {
		i := strings.Index(haystack[from:], needle)
		if i < 0 {
			return i
		}
		return from + i
	}

	switch str[offset] {
	case 'N':
		// N;
		if core.StrIdx(str, offset+1) != ';' {
			return nil, offset, readError
		}
		return phpv.ZNULL.ZVal(), offset + 2, nil
	case 'b':
		// b:1; or b:0;
		if core.StrIdx(str, i+1) != ';' {
			return nil, offset, readError
		}
		v := core.StrIdx(str, i)
		switch v {
		case '1':
			return phpv.ZTrue.ZVal(), i + 2, nil
		case '0':
			return phpv.ZFalse.ZVal(), i + 2, nil
		default:
			return nil, offset, readError
		}
	case 'i':
		// i:123456;
		semicIndex := indexOf(str, ";", i)
		if semicIndex < 0 {
			return nil, offset, readError
		}
		s := str[i:semicIndex]
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, offset, readError
		}
		return phpv.ZInt(n).ZVal(), semicIndex + 1, nil
	case 'd':
		// d:123.456;
		semicIndex := indexOf(str, ";", i)
		if semicIndex < 0 {
			return nil, offset, readError
		}
		s := str[i:semicIndex]
		p := ctx.GetConfig("serialize_precision", phpv.ZInt(14).ZVal()).AsInt(ctx)
		n, err := strconv.ParseFloat(s, int(p))
		if err != nil {
			return nil, offset, readError
		}
		return phpv.ZFloat(n).ZVal(), semicIndex, nil
	case 's':
		// s:3:"foo";
		//   ^1  ^2
		// 1 - string length
		// 2 - string contents, no escapes
		j := indexOf(str, ":", i)
		if j < 0 {
			return nil, offset, readError
		}
		s := str[i:j]
		strLen, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, offset, readError
		}
		readError.offset = 2

		startQuote := j + 1
		content := j + 2
		endQuote := content + int(strLen)
		semi := endQuote + 1

		switch {
		case content+int(strLen) >= len(str):
			return nil, offset, readError
		case core.StrIdx(str, startQuote) != '"':
			readError.offset = startQuote
			return nil, offset, readError
		case core.StrIdx(str, endQuote) != '"':
			readError.offset = endQuote
			return nil, offset, readError
		case core.StrIdx(str, semi) != ';':
			return nil, offset, readError
		}

		s = str[content : content+int(strLen)]
		return phpv.ZStr(s), semi + 1, nil

	case 'a':
		// "a:2:{i:0;s:1:"x";s:1:"y";s:1:"z";}" == ["x", "y" => z]
		//    -  --- -------
		//    ^1  ^2   ^3
		// 1 - array length
		// 2 - key of first item
		// 3 - value  of first item
		j := indexOf(str, ":", i)
		if j < 0 || j < i+1 || j >= len(str) {
			return nil, offset, readError
		}
		s := str[i:j]
		numItems, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, offset, readError
		}

		if core.StrIdx(str, j+1) != '{' {
			return nil, offset, readError
		}
		i = j + 2

		readError.offset = 2

		arr := phpv.NewZArray()
		for numItems > 0 {
			var key, value *phpv.ZVal
			key, i, err = d.parse(ctx, str, i)
			if err != nil {
				return nil, offset, readError
			}
			value, i, err = d.parse(ctx, str, i)
			if err != nil {
				return nil, offset, readError
			}
			arr.OffsetSet(ctx, key, value)
			numItems--
		}
		if core.StrIdx(str, i) != '}' {
			return nil, offset, readError
		}

		return arr.ZVal(), i + 1, nil
	case 'O':
		// O:3:"Xyz":1:{s:3:"foo";i:123;}
		//   ^1 ^2   ^3
		// 1 - class name length
		// 2 - class name
		// 3 - property count

		j := indexOf(str, ":", i)
		if j < 0 {
			return nil, offset, readError
		}
		s := str[i:j]
		strLen, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, offset, readError
		}
		readError.offset = 2

		startQuote := j + 1
		content := j + 2
		endQuote := content + int(strLen)

		switch {
		case content+int(strLen) >= len(str):
			return nil, offset, readError
		case core.StrIdx(str, startQuote) != '"':
			readError.offset = startQuote
			return nil, offset, readError
		case core.StrIdx(str, endQuote) != '"':
			readError.offset = endQuote
			return nil, offset, readError
		}

		className := str[content : content+int(strLen)]
		i = endQuote + 1
		if core.StrIdx(str, i) != ':' {
			return nil, offset, readError
		}
		i++
		j = indexOf(str, ":", i)
		if j < 0 || j < i+1 {
			return nil, offset, readError
		}
		numProps, err := strconv.Atoi(str[i:j])
		if err != nil {
			return nil, offset, readError
		}

		if core.StrIdx(str, j+1) != '{' {
			return nil, offset, readError
		}

		allowedClass := d.allowAllClasses
		if !allowedClass {
			_, allowedClass = d.allowedClasses[phpv.ZString(className)]
		}

		class, err := ctx.Global().GetClass(ctx, phpv.ZString(className), true)
		if err != nil || !allowedClass || class == nil {
			class = phpobj.IncompleteClass
		}

		obj, err := phpobj.CreateZObject(ctx, class)
		if err != nil {
			return nil, offset, err
		}
		if class == phpobj.IncompleteClass {
			obj.ObjectSet(ctx, phpv.ZStr("__PHP_Incomplete_Class_Name"), phpv.ZStr(className))
		}

		if method, ok := obj.GetClass().GetMethod(phpv.ZString("__wakeup")); ok {
			_, err := ctx.Call(ctx, method.Method, nil, obj)
			if err != nil {
				return nil, offset, err
			}
		}

		i = j + 2
		for numProps > 0 {
			var key, value *phpv.ZVal
			key, i, err = d.parse(ctx, str, i)
			if err != nil {
				return nil, offset, readError
			}
			value, i, err = d.parse(ctx, str, i)
			if err != nil {
				return nil, offset, readError
			}
			obj.ObjectSet(ctx, key.AsString(ctx), value)

			numProps--
		}
		return obj.ZVal(), i, nil
	}

	return nil, offset, readError
}
