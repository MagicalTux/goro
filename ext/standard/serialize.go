package standard

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type deserializer struct {
	allowedClasses  map[phpv.ZString]struct{}
	allowAllClasses bool
	refs            []*phpv.ZVal // reference tracking: index 0 is unused, index 1 is the first value parsed
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
		options := optionsArg.Get()
		if options.GetType() == phpv.ZtArray {
			arg, _ := options.AsArray(ctx).OffsetGet(ctx, phpv.ZString("allowed_classes"))
			switch arg.GetType() {
			case phpv.ZtArray:
				deserializer.allowAllClasses = false
				for _, className := range arg.AsArray(ctx).Iterate(ctx) {
					switch className.GetType() {
					case phpv.ZtString:
						// Direct string
						deserializer.allowedClasses[phpv.ZString(strings.ToLower(string(className.AsString(ctx))))] = struct{}{}
					case phpv.ZtObject:
						// Object with __toString is OK; call AsString which triggers __toString
						s := className.AsString(ctx)
						deserializer.allowedClasses[phpv.ZString(strings.ToLower(string(s)))] = struct{}{}
					default:
						// null, bool, int, float, array, resource → TypeError
						typeName := className.GetType().String()
						return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("unserialize(): Option \"allowed_classes\" must be an array of class names, %s given", typeName))
					}
				}
			case phpv.ZtBool:
				deserializer.allowAllClasses = bool(arg.AsBool(ctx))
			default:
				// allowed_classes must be bool or array
				typeName := arg.GetType().String()
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("unserialize(): Option \"allowed_classes\" must be of type array|bool, %s given", typeName))
			}
		}
	}

	strData := string(str)
	result, nextOffset, err := deserializer.parse(ctx, strData)
	if err != nil {
		// PHP emits a warning and returns false on unserialize errors
		ctx.Warn("%s", err.Error())
		return phpv.ZFalse.ZVal(), nil
	}
	// Warn about extra data after a valid value
	if nextOffset < len(strData) {
		ctx.Warn("Extra data starting at offset %d of %d bytes", nextOffset, len(strData))
	}
	return result, nil
}

// serializeSeen tracks both arrays and objects to prevent infinite recursion
// in serialize, particularly when Serializable::serialize() calls serialize()
// internally with cross-referenced objects.
type serializeSeen struct {
	arrays  map[*phpv.ZArray]bool
	objects map[phpv.ZObject]bool
	// Object reference tracking for r: references
	// Maps object identity to the 1-based reference index
	objRefs map[phpv.ZObject]int
	// PHP reference tracking for R: references
	// Maps inner ZVal pointer (the shared reference target) to the 1-based reference index
	valRefs map[*phpv.ZVal]int
	// Counter for reference tracking (1-based, increments for each value)
	refCount int
}

func newSerializeSeen() *serializeSeen {
	return &serializeSeen{
		arrays:  make(map[*phpv.ZArray]bool),
		objects: make(map[phpv.ZObject]bool),
		objRefs: make(map[phpv.ZObject]int),
		valRefs: make(map[*phpv.ZVal]int),
	}
}

// nextRef increments and returns the next reference index (1-based)
func (s *serializeSeen) nextRef() int {
	s.refCount++
	return s.refCount
}

func serialize(ctx phpv.Context, value *phpv.ZVal) (string, error) {
	seen := newSerializeSeen()
	// Share object tracking with nested serialize calls (e.g., Serializable::serialize()
	// calling the built-in serialize() internally). The Global stores a shared map.
	type serializeTracker interface {
		GetSerializeSeenObjects() map[phpv.ZObject]bool
		SetSerializeSeenObjects(map[phpv.ZObject]bool)
	}
	if g, ok := ctx.Global().(serializeTracker); ok {
		existing := g.GetSerializeSeenObjects()
		if existing != nil {
			// Reuse the existing seen objects from the outer serialize call
			seen.objects = existing
		} else {
			// First (outermost) serialize call: set up and clean up
			g.SetSerializeSeenObjects(seen.objects)
			defer g.SetSerializeSeenObjects(nil)
		}
	}
	return serializeValue(ctx, value, 0, seen)
}

const maxSerializeDepth = 128

// serializeKey serializes an array key (int or string) WITHOUT consuming a reference slot.
// In PHP, array keys do not participate in reference counting.
func serializeKey(ctx phpv.Context, value *phpv.ZVal) string {
	switch value.GetType() {
	case phpv.ZtInt:
		n := value.AsInt(ctx)
		return "i:" + strconv.FormatInt(int64(n), 10) + ";"
	case phpv.ZtString:
		s := string(value.AsString(ctx))
		return "s:" + strconv.Itoa(len(s)) + ":\"" + s + "\";"
	default:
		// Fallback: cast to string
		s := string(value.AsString(ctx))
		return "s:" + strconv.Itoa(len(s)) + ":\"" + s + "\";"
	}
}

// serializeValue serializes a raw ZVal (which may be a reference wrapper).
// This handles R: reference detection for PHP & references.
func serializeValue(ctx phpv.Context, rawZVal *phpv.ZVal, depth int, seen *serializeSeen) (string, error) {
	if depth > maxSerializeDepth {
		return "N;", nil // prevent infinite recursion
	}

	// Check for PHP reference (R:N;) - if rawZVal is a reference wrapper,
	// its inner ZVal is the shared target. If we've seen it before, produce R:N;
	if rawZVal.IsRef() {
		inner := rawZVal.RefTarget()
		if inner == nil {
			return serializeWithDepth(ctx, rawZVal, depth, seen)
		}
		if refIdx, ok := seen.valRefs[inner]; ok {
			// This reference target was already serialized; produce R:N;
			return "R:" + strconv.Itoa(refIdx) + ";", nil
		}
		// First time seeing this reference target - the ref index will be assigned
		// inside serializeWithDepth and we register it here
		// Peek at what index will be assigned and register the inner ZVal
		nextIdx := seen.refCount + 1
		seen.valRefs[inner] = nextIdx
		// Serialize the unwrapped value (serializeWithDepth will assign the same index)
		return serializeWithDepth(ctx, inner, depth, seen)
	}

	return serializeWithDepth(ctx, rawZVal, depth, seen)
}

func serializeWithDepth(ctx phpv.Context, value *phpv.ZVal, depth int, seen *serializeSeen) (string, error) {
	if depth > maxSerializeDepth {
		return "N;", nil // prevent infinite recursion
	}
	var result string
	switch value.GetType() {
	case phpv.ZtNull:
		seen.nextRef() // NULL still gets a reference slot
		result = "N;"
	case phpv.ZtResource:
		seen.nextRef()
		// PHP serializes resources as their integer ID
		r := value.Value().(phpv.Resource)
		result = "i:" + strconv.Itoa(r.GetResourceID()) + ";"
	case phpv.ZtBool:
		seen.nextRef()
		switch value.AsBool(ctx) {
		case true:
			result = "b:1;"
		case false:
			result = "b:0;"
		}
	case phpv.ZtInt:
		seen.nextRef()
		n := value.AsInt(ctx)
		result = "i:" + strconv.FormatInt(int64(n), 10) + ";"
	case phpv.ZtFloat:
		seen.nextRef()
		n := value.AsFloat(ctx)
		p := phpv.GetSerializePrecision(ctx)
		var s string
		if p == 0 {
			// PHP's zend_gcvt at ndigit=0 uses scientific notation for all
			// non-zero finite values (threshold: decpt > 0, always true).
			s = phpv.FormatFloatSerialize(float64(n))
		} else {
			s = phpv.FormatFloatPrecision(float64(n), p)
		}
		result = "d:" + s + ";"
	case phpv.ZtString:
		seen.nextRef()
		s := string(value.AsString(ctx))
		result = "s:" + strconv.Itoa(len(s)) + ":\"" + s + "\";"
	case phpv.ZtArray:
		arr := value.AsArray(ctx)

		// Detect array cycles - for references to the same array, produce R:N;
		if seen.arrays[arr] {
			return "N;", nil
		}
		seen.arrays[arr] = true
		defer delete(seen.arrays, arr)

		seen.nextRef() // array gets a reference slot

		count := strconv.FormatInt(int64(arr.Count(ctx)), 10)

		var buf bytes.Buffer
		buf.WriteString("a:")
		buf.WriteString(count)
		buf.WriteString(":{")

		for k, v := range arr.IterateRaw(ctx) {
			// Array keys don't consume reference slots in PHP
			buf.WriteString(serializeKey(ctx, k))
			sub, err := serializeValue(ctx, v, depth+1, seen)
			if err != nil {
				return "", err
			}
			buf.WriteString(sub)
		}
		buf.WriteString("}")
		result = buf.String()
	case phpv.ZtObject:
		obj := value.AsObject(ctx)

		// Lazy objects: serialize triggers initialization
		if zo, ok := obj.(*phpobj.ZObject); ok && zo.IsLazy() {
			if err := zo.TriggerLazyInit(ctx); err != nil {
				return "", err
			}
		}
		// For initialized proxies, serialize the real instance
		if zo, ok := obj.(*phpobj.ZObject); ok && zo.LazyState == phpobj.LazyProxyInitialized && zo.LazyInstance != nil {
			obj = zo.LazyInstance
		}

		// Check if we've already fully serialized this object - produce r:N; reference
		if refIdx, ok := seen.objRefs[obj]; ok {
			return "r:" + strconv.Itoa(refIdx) + ";", nil
		}

		// Detect object cycles to prevent infinite recursion
		// (especially with Serializable::serialize() calling serialize() internally)
		if seen.objects[obj] {
			return "N;", nil
		}
		seen.objects[obj] = true

		// Assign reference number for this object
		objRefIdx := seen.nextRef()
		// Register in objRefs so future encounters produce r:N;
		seen.objRefs[obj] = objRefIdx

		// Enum serialization: E:length:"ClassName:CaseName";
		if obj.GetClass().GetType().Has(phpv.ZClassTypeEnum) {
			zobj := obj.(*phpobj.ZObject)
			caseName := ""
			if nameVal := zobj.HashTable().GetString("name"); nameVal != nil {
				caseName = nameVal.String()
			}
			enumStr := fmt.Sprintf("%s:%s", obj.GetClass().GetName(), caseName)
			result = fmt.Sprintf(`E:%d:"%s";`, len(enumStr), enumStr)
			return result, nil
		}

		// Check for __serialize() method (PHP 7.4+, preferred over Serializable and __sleep)
		if method, ok := obj.GetClass().GetMethod(phpv.ZString("__serialize")); ok {
			val, err := ctx.Call(ctx, method.Method, nil, obj)
			if err != nil {
				return "", err
			}
			if val.GetType() != phpv.ZtArray {
				return "", phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("%s::__serialize() must return an array", obj.GetClass().GetName()))
			}
			arr := val.AsArray(ctx)
			if arr == nil {
				return "N;", nil
			}

			var buf bytes.Buffer
			propCount := 0
			for k, v := range arr.IterateRaw(ctx) {
				// Property keys don't consume reference slots
				buf.WriteString(serializeKey(ctx, k))
				sub, err := serializeValue(ctx, v, depth+1, seen)
				if err != nil {
					return "", err
				}
				buf.WriteString(sub)
				propCount++
			}

			contents := buf.String()
			buf.Reset()
			className := string(obj.GetClass().GetName())
			buf.WriteString("O:" + strconv.Itoa(len(className)) + ":\"" + className + "\":" + strconv.Itoa(propCount) + ":")
			buf.WriteString("{")
			buf.WriteString(contents)
			buf.WriteString("}")
			return buf.String(), nil
		}

		// Check for Serializable interface (deprecated in PHP 8.1+, after __serialize)
		if obj.GetClass().Implements(phpobj.Serializable) {
			if method, ok := obj.GetClass().GetMethod(phpv.ZString("serialize")); ok {
				val, err := ctx.Call(ctx, method.Method, nil, obj)
				if err != nil {
					return "", err
				}
				if val.IsNull() {
					// Serializable::serialize() returned null - remove object from objRefs
					// so that other references to this object also serialize as N;
					// Also remove any valRefs entries pointing to the same ref index,
					// so that PHP references (&) to this object also produce N;
					refIdx := seen.objRefs[obj]
					delete(seen.objRefs, obj)
					for k, v := range seen.valRefs {
						if v == refIdx {
							delete(seen.valRefs, k)
						}
					}
					return "N;", nil
				}
				if val.GetType() != phpv.ZtString {
					return "", phpobj.ThrowError(ctx, phpobj.Exception, fmt.Sprintf("%s::serialize() must return a string or NULL", obj.GetClass().GetName()))
				}
				data := string(val.AsString(ctx))
				className := string(obj.GetClass().GetName())
				result = "C:" + strconv.Itoa(len(className)) + ":\"" + className + "\":" + strconv.Itoa(len(data)) + ":{" + data + "}"
				return result, nil
			}
		}

		var props *phpv.ZArray
		if method, ok := obj.GetClass().GetMethod(phpv.ZString("__sleep")); ok {
			val, err := ctx.Call(ctx, method.Method, nil, obj)
			if err != nil {
				return "", err
			}
			if val.GetType() == phpv.ZtArray {
				props = val.AsArray(ctx)
			} else {
				// __sleep must return an array; if not, serialize returns NULL
				ctx.Warn("%s::__sleep() should return an array only containing the names of instance-variables to serialize", obj.GetClass().GetName())
				return "N;", nil
			}
		}

		var buf bytes.Buffer
		propCount := 0

		if props != nil {
			zobj := obj.(*phpobj.ZObject)
			sleepSeen := make(map[phpv.ZString]bool)
			for _, prop := range props.Iterate(ctx) {
				// Cast non-string elements to string (PHP does this)
				propName := prop.AsString(ctx)
				// Detect duplicate property names from __sleep()
				if sleepSeen[propName] {
					ctx.Warn("\"%s\" is returned from __sleep() multiple times", propName)
					continue
				}
				sleepSeen[propName] = true
				// Look up the actual property to determine visibility.
				// For private properties, we need to check if the property is
				// accessible from the object's actual class (not a parent's private).
				classProp, found := obj.GetClass().GetProp(propName)
				// Private properties from parent classes are not accessible by name
				// from the child class context in __sleep
				if found && classProp.Modifiers.IsPrivate() {
					// Check if the property is declared in the object's own class
					// (not inherited from a parent class)
					declClass := zobj.GetDeclClassName(classProp)
					if declClass != obj.GetClass().GetName() {
						// Private property in parent class - not accessible by simple name
						found = false
					}
				}
				if !found {
					// Check if it's a dynamic property on the object
					if zobj.HashTable().HasString(propName) {
						mangledName := string(propName)
						sub := "s:" + strconv.Itoa(len(mangledName)) + ":\"" + mangledName + "\";"
						buf.WriteString(sub)
						v := zobj.HashTable().GetString(propName)
						sub2, err := serializeValue(ctx, v, depth+1, seen)
						if err != nil {
							return "", err
						}
						buf.WriteString(sub2)
						propCount++
						continue
					}
					// Property not found - warn and skip
					ctx.Warn("\"%s\" returned as member variable from __sleep() but does not exist", propName)
					continue
				}
				// Mangle property name based on visibility
				// Use classProp.VarName (the real property name) for mangling,
				// since propName might already be in mangled format from __sleep()
				var mangledName string
				if classProp.Modifiers.IsPrivate() {
					className := string(zobj.GetDeclClassName(classProp))
					mangledName = "\x00" + className + "\x00" + string(classProp.VarName)
				} else if classProp.Modifiers.IsProtected() {
					mangledName = "\x00*\x00" + string(classProp.VarName)
				} else {
					mangledName = string(classProp.VarName)
				}
				sub := "s:" + strconv.Itoa(len(mangledName)) + ":\"" + mangledName + "\";"
				buf.WriteString(sub)

				v := zobj.GetPropValue(classProp)
				sub2, err := serializeValue(ctx, v, depth+1, seen)
				if err != nil {
					return "", err
				}
				buf.WriteString(sub2)
				propCount++
			}
		} else {
			zobj := obj.(*phpobj.ZObject)
			for prop := range obj.IterProps(ctx) {
				// Mangle property name based on visibility
				var mangledName string
				if prop.Modifiers.IsPrivate() {
					className := string(zobj.GetDeclClassName(prop))
					mangledName = "\x00" + className + "\x00" + string(prop.VarName)
				} else if prop.Modifiers.IsProtected() {
					mangledName = "\x00*\x00" + string(prop.VarName)
				} else {
					mangledName = string(prop.VarName)
				}
				sub := "s:" + strconv.Itoa(len(mangledName)) + ":\"" + mangledName + "\";"
				buf.WriteString(sub)

				v := zobj.GetPropValue(prop)
				sub2, err := serializeValue(ctx, v, depth+1, seen)
				if err != nil {
					return "", err
				}
				buf.WriteString(sub2)

				propCount++
			}
		}

		contents := buf.String()
		buf.Reset()
		className := string(obj.GetClass().GetName())
		buf.WriteString("O:" + strconv.Itoa(len(className)) + ":\"" + className + "\":" + strconv.Itoa(propCount) + ":")
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

// addRef registers a value in the reference table and returns its 1-based index.
func (d *deserializer) addRef(z *phpv.ZVal) int {
	d.refs = append(d.refs, z)
	return len(d.refs) // 1-based
}

// getRef returns the value at a 1-based reference index.
func (d *deserializer) getRef(index int) *phpv.ZVal {
	if index < 1 || index > len(d.refs) {
		return nil
	}
	return d.refs[index-1]
}

// parseKey parses a value without registering it in the reference table (for array/object keys).
func (d *deserializer) parseKey(ctx phpv.Context, str string, offset int) (result *phpv.ZVal, nextOffset int, err error) {
	// Save ref count, parse, then restore (keys don't count as references)
	savedLen := len(d.refs)
	result, nextOffset, err = d.parse(ctx, str, offset)
	// Remove any refs that were added during key parsing
	d.refs = d.refs[:savedLen]
	return
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

	if offset >= len(str) {
		return nil, offset, readError
	}

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

	// Helper: parse a length value and reject signed numbers (PHP rejects +N and -N for lengths)
	parseLengthUnsigned := func(s string) (int64, error) {
		if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
			return 0, fmt.Errorf("signed length")
		}
		return strconv.ParseInt(s, 10, 64)
	}

	switch str[offset] {
	case 'N':
		// N;
		if core.StrIdx(str, offset+1) != ';' {
			return nil, offset, readError
		}
		val := phpv.ZNULL.ZVal()
		d.addRef(val)
		return val, offset + 2, nil
	case 'b':
		// b:1; or b:0;
		if core.StrIdx(str, i+1) != ';' {
			return nil, offset, readError
		}
		v := core.StrIdx(str, i)
		var val *phpv.ZVal
		switch v {
		case '1':
			val = phpv.ZTrue.ZVal()
		case '0':
			val = phpv.ZFalse.ZVal()
		default:
			return nil, offset, readError
		}
		d.addRef(val)
		return val, i + 2, nil
	case 'i':
		// i:123456;
		semicIndex := indexOf(str, ";", i)
		if semicIndex < 0 {
			return nil, offset, &unserializeError{offset, len(str)}
		}
		s := str[i:semicIndex]
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, offset, &unserializeError{offset, len(str)}
		}
		val := phpv.ZInt(n).ZVal()
		d.addRef(val)
		return val, semicIndex + 1, nil
	case 'd':
		// d:123.456;
		semicIndex := indexOf(str, ";", i)
		if semicIndex < 0 {
			return nil, offset, readError
		}
		s := str[i:semicIndex]
		// Handle special float values
		var val *phpv.ZVal
		switch s {
		case "INF":
			val = phpv.ZFloat(math.Inf(1)).ZVal()
		case "-INF":
			val = phpv.ZFloat(math.Inf(-1)).ZVal()
		case "NAN":
			val = phpv.ZFloat(math.NaN()).ZVal()
		default:
			n, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, offset, readError
			}
			val = phpv.ZFloat(n).ZVal()
		}
		d.addRef(val)
		return val, semicIndex + 1, nil
	case 's':
		// s:3:"foo";
		j := indexOf(str, ":", i)
		if j < 0 {
			return nil, offset, readError
		}
		s := str[i:j]
		strLen, err := parseLengthUnsigned(s)
		if err != nil || strLen < 0 {
			return nil, offset, readError
		}

		startQuote := j + 1
		content := j + 2
		endQuote := content + int(strLen)
		semi := endQuote + 1

		switch {
		case content+int(strLen) >= len(str):
			return nil, offset, &unserializeError{offset + 2, len(str)}
		case core.StrIdx(str, startQuote) != '"':
			return nil, offset, &unserializeError{startQuote, len(str)}
		case core.StrIdx(str, endQuote) != '"':
			return nil, offset, &unserializeError{endQuote, len(str)}
		case core.StrIdx(str, semi) != ';':
			return nil, offset, &unserializeError{semi, len(str)}
		}

		s = str[content : content+int(strLen)]
		val := phpv.ZStr(s)
		d.addRef(val)
		return val, semi + 1, nil

	case 'S':
		// S:3:"\65bc"; - escaped string (deprecated in PHP 8.5)
		ctx.Deprecated("Unserializing the 'S' format is deprecated")
		j := indexOf(str, ":", i)
		if j < 0 {
			return nil, offset, readError
		}
		s := str[i:j]
		strLen, err := parseLengthUnsigned(s)
		if err != nil || strLen < 0 {
			return nil, offset, readError
		}

		startQuote := j + 1
		content := j + 2
		endQuote := content + int(strLen)
		semi := endQuote + 1

		switch {
		case content+int(strLen) >= len(str):
			return nil, offset, readError
		case core.StrIdx(str, startQuote) != '"':
			return nil, offset, readError
		case core.StrIdx(str, endQuote) != '"':
			return nil, offset, readError
		case core.StrIdx(str, semi) != ';':
			return nil, offset, readError
		}

		// Decode escaped string: \xx hex escapes
		raw := str[content : content+int(strLen)]
		var buf bytes.Buffer
		for k := 0; k < len(raw); k++ {
			if raw[k] == '\\' && k+2 < len(raw) {
				hi := unhex(raw[k+1])
				lo := unhex(raw[k+2])
				buf.WriteByte(hi<<4 | lo)
				k += 2
				continue
			}
			buf.WriteByte(raw[k])
		}
		val := phpv.ZString(buf.String()).ZVal()
		d.addRef(val)
		return val, semi + 1, nil

	case 'R':
		// R:N; - value reference (creates a PHP reference to the Nth value)
		semicIndex := indexOf(str, ";", i)
		if semicIndex < 0 {
			return nil, offset, readError
		}
		s := str[i:semicIndex]
		// Reject signed references
		if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
			return nil, offset, &unserializeError{offset, len(str)}
		}
		index, err := strconv.ParseInt(s, 10, 64)
		if err != nil || index < 1 {
			return nil, offset, readError
		}
		ref := d.getRef(int(index))
		if ref == nil {
			return nil, offset, readError
		}
		// R: creates a reference - make the referred value a reference if not already
		ref = ref.Ref()
		d.addRef(ref)
		return ref, semicIndex + 1, nil

	case 'r':
		// r:N; - object reference (reuses the Nth object without creating a PHP reference)
		semicIndex := indexOf(str, ";", i)
		if semicIndex < 0 {
			return nil, offset, readError
		}
		s := str[i:semicIndex]
		// Reject signed references
		if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
			return nil, offset, &unserializeError{offset, len(str)}
		}
		index, err := strconv.ParseInt(s, 10, 64)
		if err != nil || index < 1 {
			return nil, offset, readError
		}
		ref := d.getRef(int(index))
		if ref == nil {
			return nil, offset, readError
		}
		// r: doesn't create a reference, just reuses the value
		d.addRef(ref)
		return ref, semicIndex + 1, nil

	case 'a':
		// "a:2:{i:0;s:1:"x";s:1:"y";s:1:"z";}" == ["x", "y" => z]
		j := indexOf(str, ":", i)
		if j < 0 || j < i+1 || j >= len(str) {
			return nil, offset, readError
		}
		s := str[i:j]
		numItems, err := parseLengthUnsigned(s)
		if err != nil || numItems < 0 {
			return nil, offset, readError
		}

		if core.StrIdx(str, j+1) != '{' {
			return nil, offset, readError
		}
		i = j + 2

		arr := phpv.NewZArray()
		// Register the array in refs before parsing its contents
		d.addRef(arr.ZVal())

		for numItems > 0 {
			var key, value *phpv.ZVal
			key, i, err = d.parseKey(ctx, str, i)
			if err != nil {
				// Propagate the inner error offset if available
				if ue, ok := err.(*unserializeError); ok {
					return nil, offset, ue
				}
				return nil, offset, &unserializeError{i, len(str)}
			}

			if i >= len(str) {
				return nil, offset, &unserializeError{i, len(str)}
			}

			value, i, err = d.parse(ctx, str, i)
			if err != nil {
				// Propagate the inner error offset if available
				if ue, ok := err.(*unserializeError); ok {
					return nil, offset, ue
				}
				return nil, offset, &unserializeError{i, len(str)}
			}
			arr.OffsetSet(ctx, key, value)
			numItems--
		}
		if core.StrIdx(str, i) != '}' {
			return nil, offset, &unserializeError{i, len(str)}
		}

		return arr.ZVal(), i + 1, nil
	case 'E':
		// E:7:"Foo:Bar";
		// Enum unserialization
		j := indexOf(str, ":", i)
		if j < 0 {
			return nil, offset, readError
		}
		strLen, err := strconv.ParseInt(str[i:j], 10, 64)
		if err != nil {
			return nil, offset, readError
		}
		startQuote := j + 1
		content := j + 2
		endQuote := content + int(strLen)

		if content+int(strLen) >= len(str) ||
			core.StrIdx(str, startQuote) != '"' ||
			core.StrIdx(str, endQuote) != '"' {
			return nil, offset, readError
		}

		enumStr := str[content:endQuote]
		endSemi := endQuote + 1
		if endSemi >= len(str) || core.StrIdx(str, endSemi) != ';' {
			return nil, offset, readError
		}

		// Parse "ClassName:CaseName"
		colonIdx := strings.Index(enumStr, ":")
		if colonIdx < 0 {
			ctx.Warn("Invalid enum name '%s' (missing colon)", enumStr)
			return nil, offset, &unserializeError{0, len(str)}
		}
		className := enumStr[:colonIdx]
		caseName := enumStr[colonIdx+1:]

		// Look up the class
		cls, clsErr := ctx.Global().GetClass(ctx, phpv.ZString(className), false)
		if clsErr != nil {
			ctx.Warn("Class '%s' not found", className)
			return nil, offset, &unserializeError{0, len(str)}
		}

		// Verify it's an enum
		if !cls.GetType().Has(phpv.ZClassTypeEnum) {
			ctx.Warn("Class '%s' is not an enum", className)
			return nil, offset, &unserializeError{0, len(str)}
		}

		// Look up the case constant
		zc := cls.(*phpobj.ZClass)
		cc, exists := zc.Const[phpv.ZString(caseName)]
		if !exists {
			ctx.Warn("Undefined constant %s::%s", className, caseName)
			return nil, offset, &unserializeError{endSemi + 1, len(str)}
		}

		// Resolve CompileDelayed if needed
		val := cc.Value
		if cd, ok := val.(*phpv.CompileDelayed); ok {
			z2, err2 := cd.Run(ctx)
			if err2 != nil {
				return nil, offset, readError
			}
			zc.Const[phpv.ZString(caseName)].Value = z2.Value()
			val = z2.Value()
		}

		// Check that it's actually an enum case (not just a regular class constant)
		isEnumCase := false
		for _, ec := range zc.EnumCases {
			if string(ec) == caseName {
				isEnumCase = true
				break
			}
		}
		if !isEnumCase {
			ctx.Warn("%s::%s is not an enum case", className, caseName)
			return nil, offset, &unserializeError{endSemi + 1, len(str)}
		}
		if _, ok := val.(*phpobj.ZObject); !ok {
			ctx.Warn("%s::%s is not an enum case", className, caseName)
			return nil, offset, &unserializeError{endSemi + 1, len(str)}
		}

		zval := val.ZVal()
		d.addRef(zval)
		return zval, endSemi + 1, nil
	case 'O':
		// O:3:"Xyz":1:{s:3:"foo";i:123;}
		j := indexOf(str, ":", i)
		if j < 0 {
			return nil, offset, readError
		}
		s := str[i:j]
		strLen, err := parseLengthUnsigned(s)
		if err != nil || strLen < 0 {
			return nil, offset, readError
		}

		startQuote := j + 1
		content := j + 2
		endQuote := content + int(strLen)

		switch {
		case content+int(strLen) >= len(str):
			return nil, offset, &unserializeError{offset + 2, len(str)}
		case core.StrIdx(str, startQuote) != '"':
			return nil, offset, &unserializeError{startQuote, len(str)}
		case core.StrIdx(str, endQuote) != '"':
			return nil, offset, &unserializeError{endQuote, len(str)}
		}

		className := str[content : content+int(strLen)]
		i = endQuote + 1
		if core.StrIdx(str, i) != ':' {
			return nil, offset, &unserializeError{i, len(str)}
		}
		i++
		j = indexOf(str, ":", i)
		if j < 0 || j < i+1 {
			return nil, offset, &unserializeError{i, len(str)}
		}
		numPropsStr := str[i:j]
		if len(numPropsStr) > 0 && (numPropsStr[0] == '+' || numPropsStr[0] == '-') {
			return nil, offset, &unserializeError{i, len(str)}
		}
		numProps, err := strconv.Atoi(numPropsStr)
		if err != nil || numProps < 0 {
			return nil, offset, &unserializeError{i, len(str)}
		}

		if core.StrIdx(str, j+1) != '{' {
			return nil, offset, &unserializeError{j + 1, len(str)}
		}

		allowedClass := d.allowAllClasses
		if !allowedClass {
			// Case-insensitive class name matching
			_, allowedClass = d.allowedClasses[phpv.ZString(strings.ToLower(className))]
		}

		class, err := ctx.Global().GetClass(ctx, phpv.ZString(className), true)
		if (err != nil || class == nil) && allowedClass {
			// Try unserialize_callback_func if set
			cbFuncVal := ctx.GetConfig("unserialize_callback_func", phpv.ZNULL.ZVal())
			if cbFuncVal != nil && !cbFuncVal.IsNull() && cbFuncVal.String() != "" {
				cbName := cbFuncVal.AsString(ctx)
				if cbCallable, cbErr := ctx.Global().GetFunction(ctx, cbName); cbErr == nil && cbCallable != nil {
					ctx.Global().CallZVal(ctx, cbCallable, []*phpv.ZVal{phpv.ZStr(className)})
					// Try again after callback
					class, err = ctx.Global().GetClass(ctx, phpv.ZString(className), true)
					if err != nil || class == nil {
						ctx.Warn("Function %s() hasn't defined the class it was called for", cbName)
					}
				} else {
					ctx.Warn("Invalid callback %s, function \"%s\" not found or invalid function name", cbName, cbName)
				}
			}
		}
		if err != nil || !allowedClass || class == nil {
			class = phpobj.IncompleteClass
		}

		// Check if the class is an enum (enums cannot be unserialized with O:)
		if class.GetType().Has(phpv.ZClassTypeEnum) {
			return nil, offset, &unserializeError{0, len(str)}
		}

		// Check if the class is abstract (abstract classes cannot be instantiated)
		if class != phpobj.IncompleteClass && (class.GetType().Has(phpv.ZClassTypeExplicitAbstract) || class.GetType().Has(phpv.ZClassTypeImplicitAbstract)) {
			ctx.Warn("Cannot instantiate abstract class %s", class.GetName())
			return nil, offset, &unserializeError{0, len(str)}
		}

		obj, err := phpobj.CreateZObject(ctx, class)
		if err != nil {
			return nil, offset, err
		}
		if class == phpobj.IncompleteClass {
			obj.ObjectSet(ctx, phpv.ZStr("__PHP_Incomplete_Class_Name"), phpv.ZStr(className))
		}

		// Register object in refs before parsing properties (for back-references)
		d.addRef(obj.ZVal())

		// Check if class has __unserialize method
		_, hasUnserialize := obj.GetClass().GetMethod(phpv.ZString("__unserialize"))

		i = j + 2
		if hasUnserialize {
			// Collect key-value pairs into an array, then call __unserialize
			arr := phpv.NewZArray()
			for numProps > 0 {
				var key, value *phpv.ZVal
				key, i, err = d.parseKey(ctx, str, i)
				if err != nil {
					// PHP emits "Unexpected end of serialized data" warning
					if i >= len(str) || (i < len(str) && str[i] == '}') {
						ctx.Warn("Unexpected end of serialized data")
						return nil, offset, &unserializeError{i, len(str)}
					}
					if ue, ok := err.(*unserializeError); ok {
						return nil, offset, ue
					}
					return nil, offset, &unserializeError{i, len(str)}
				}
				value, i, err = d.parse(ctx, str, i)
				if err != nil {
					if ue, ok := err.(*unserializeError); ok {
						return nil, offset, ue
					}
					return nil, offset, &unserializeError{i, len(str)}
				}
				arr.OffsetSet(ctx, key, value)
				numProps--
			}
			method, _ := obj.GetClass().GetMethod(phpv.ZString("__unserialize"))
			_, err := ctx.Global().CallZVal(ctx, method.Method, []*phpv.ZVal{arr.ZVal()}, obj)
			if err != nil {
				return nil, offset, err
			}
		} else {
			for numProps > 0 {
				var key, value *phpv.ZVal
				key, i, err = d.parseKey(ctx, str, i)
				if err != nil {
					if ue, ok := err.(*unserializeError); ok {
						return nil, offset, ue
					}
					return nil, offset, &unserializeError{i, len(str)}
				}
				value, i, err = d.parse(ctx, str, i)
				if err != nil {
					if ue, ok := err.(*unserializeError); ok {
						return nil, offset, ue
					}
					return nil, offset, &unserializeError{i, len(str)}
				}
				unserializeSetProperty(ctx, obj, key.AsString(ctx), value)
				numProps--
			}
			if method, ok := obj.GetClass().GetMethod(phpv.ZString("__wakeup")); ok {
				_, err := ctx.Call(ctx, method.Method, nil, obj)
				if err != nil {
					return nil, offset, err
				}
			}
		}
		// Skip closing '}'
		if i < len(str) && str[i] == '}' {
			i++
		}
		return obj.ZVal(), i, nil
	case 'C':
		// C:3:"Xyz":6:{data_s}
		j := indexOf(str, ":", i)
		if j < 0 {
			return nil, offset, readError
		}
		s := str[i:j]
		strLen, err := parseLengthUnsigned(s)
		if err != nil || strLen < 0 {
			return nil, offset, readError
		}

		startQuote := j + 1
		content := j + 2
		endQuote := content + int(strLen)

		switch {
		case content+int(strLen) >= len(str):
			return nil, offset, readError
		case core.StrIdx(str, startQuote) != '"':
			return nil, offset, readError
		case core.StrIdx(str, endQuote) != '"':
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
		dataLenStr := str[i:j]
		dataLen64, err := strconv.ParseInt(dataLenStr, 10, 64)
		if err != nil || dataLen64 < 0 {
			return nil, offset, &unserializeError{i, len(str)}
		}
		dataLen := int(dataLen64)
		// Check for integer overflow or unreasonable size
		if int64(dataLen) != dataLen64 || dataLen > len(str) {
			// PHP emits a warning about insufficient data when dataLen is huge
			ctx.Warn("Insufficient data for unserializing - %d required, %d present", dataLen64, len(str)-(j+2))
			return nil, offset, &unserializeError{j + 2 + dataLen, len(str)}
		}

		if core.StrIdx(str, j+1) != '{' {
			return nil, offset, &unserializeError{j + 1, len(str)}
		}

		dataStart := j + 2
		dataEnd := dataStart + dataLen
		if dataEnd >= len(str) || core.StrIdx(str, dataEnd) != '}' {
			return nil, offset, &unserializeError{dataEnd, len(str)}
		}
		data := str[dataStart:dataEnd]

		allowedClass := d.allowAllClasses
		if !allowedClass {
			// Case-insensitive class name matching
			_, allowedClass = d.allowedClasses[phpv.ZString(strings.ToLower(className))]
		}

		class, err := ctx.Global().GetClass(ctx, phpv.ZString(className), true)
		if err != nil || !allowedClass || class == nil {
			class = phpobj.IncompleteClass
		}

		// Validate that class implementing Serializable uses C: format properly
		if class != phpobj.IncompleteClass && !class.Implements(phpobj.Serializable) {
			// If the class doesn't implement Serializable, C: format is invalid
			ctx.Warn("Erroneous data format for unserializing '%s'", className)
			return nil, offset, &unserializeError{i - 1, len(str)}
		}

		obj, err := phpobj.CreateZObject(ctx, class)
		if err != nil {
			return nil, offset, err
		}

		// Register object in refs
		d.addRef(obj.ZVal())

		// Call the unserialize($data) method on the object
		if method, ok := obj.GetClass().GetMethod(phpv.ZString("unserialize")); ok {
			_, err := ctx.Global().CallZVal(ctx, method.Method, []*phpv.ZVal{phpv.ZStr(data).ZVal()}, obj)
			if err != nil {
				return nil, offset, err
			}
		}

		return obj.ZVal(), dataEnd + 1, nil
	}

	return nil, offset, readError
}

// unserializeSetProperty sets a property on an object during unserialization,
// handling PHP's property name mangling for visibility.
// Mangled names: "\0ClassName\0prop" for private, "\0*\0prop" for protected
func unserializeSetProperty(ctx phpv.Context, obj phpv.ZObject, key phpv.ZString, value *phpv.ZVal) {
	keyStr := string(key)

	// Check if this is a mangled property name
	if len(keyStr) > 0 && keyStr[0] == '\x00' {
		// Find the second \x00
		secondNull := strings.IndexByte(keyStr[1:], '\x00')
		if secondNull >= 0 {
			propName := keyStr[secondNull+2:]
			if zobj, ok := obj.(*phpobj.ZObject); ok {
				// Check if the class has a declared property with this name
				if _, found := obj.GetClass().GetProp(phpv.ZString(propName)); found {
					// Directly set the property value in the hash table,
					// bypassing visibility checks (unserialization can set any property)
					zobj.HashTable().SetString(phpv.ZString(propName), value)
					return
				}
				// Property not declared - set as dynamic property using demangled name
				zobj.HashTable().SetString(phpv.ZString(propName), value)
				return
			}
		}
	}

	// Non-mangled name: for declared properties, set directly bypassing visibility
	if zobj, ok := obj.(*phpobj.ZObject); ok {
		if _, found := obj.GetClass().GetProp(key); found {
			zobj.HashTable().SetString(key, value)
			return
		}
	}
	// Dynamic property or non-ZObject: use normal ObjectSet
	obj.ObjectSet(ctx, key, value)
}

