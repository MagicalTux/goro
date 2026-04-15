package standard

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type deserializer struct {
	allowedClasses  map[phpv.ZString]struct{}
	allowAllClasses bool
	refs            []*phpv.ZVal // reference tracking: index 0 is unused, index 1 is the first value parsed
	maxDepth        int          // maximum nesting depth (0 = use ini setting)
	currentDepth    int          // current parse depth
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

	// Get max_depth from ini setting (default 4096)
	iniMaxDepth := 4096
	if iniVal := ctx.GetConfig("unserialize_max_depth", phpv.ZInt(4096).ZVal()); iniVal != nil {
		if n := int(iniVal.AsInt(ctx)); n >= 0 {
			iniMaxDepth = n
		}
	}
	deserializer := &deserializer{
		allowAllClasses: true,
		allowedClasses:  map[phpv.ZString]struct{}{},
		maxDepth:        iniMaxDepth,
	}
	if optionsArg.HasArg() {
		options := optionsArg.Get()
		if options.GetType() == phpv.ZtArray {
			// Parse max_depth option
			if maxDepthVal, _ := options.AsArray(ctx).OffsetGet(ctx, phpv.ZString("max_depth")); maxDepthVal != nil && !maxDepthVal.IsNull() {
				// max_depth must be an integer
				if maxDepthVal.GetType() != phpv.ZtInt {
					typeName := maxDepthVal.GetType().TypeName()
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("unserialize(): Option \"max_depth\" must be of type int, %s given", typeName))
				}
				n := int(maxDepthVal.AsInt(ctx))
				if n < 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "unserialize(): Option \"max_depth\" must be greater than or equal to 0")
				}
				deserializer.maxDepth = n
			}
			arg, _ := options.AsArray(ctx).OffsetGet(ctx, phpv.ZString("allowed_classes"))
			// Only process allowed_classes if the key was actually present in the options array.
			// Use OffsetExists to distinguish "key not present" from "key is null".
			hasAllowedClasses, _ := options.AsArray(ctx).OffsetExists(ctx, phpv.ZString("allowed_classes"))
			if hasAllowedClasses {
				switch arg.GetType() {
				case phpv.ZtArray:
					deserializer.allowAllClasses = false
					for _, className := range arg.AsArray(ctx).Iterate(ctx) {
						switch className.GetType() {
						case phpv.ZtString:
							// Validate the class name string for invalid patterns
							cn := string(className.AsString(ctx))
							if err := validateAllowedClassName(cn); err != nil {
								return nil, phpobj.ThrowError(ctx, phpobj.ValueError, err.Error())
							}
							deserializer.allowedClasses[phpv.ZString(strings.ToLower(cn))] = struct{}{}
						case phpv.ZtObject:
							// Object with __toString is OK; call As(ZtString) which triggers __toString
							sv, err := className.As(ctx, phpv.ZtString)
							if err != nil {
								return nil, err
							}
							s := sv.Value().(phpv.ZString)
							deserializer.allowedClasses[phpv.ZString(strings.ToLower(string(s)))] = struct{}{}
						default:
							// null, bool, int, float, array, resource → TypeError
							// PHP uses specific value names: "null", "false", "true", "int", "float", "array", "resource"
							var typeName string
							switch className.GetType() {
							case phpv.ZtNull:
								typeName = "null"
							case phpv.ZtBool:
								if className.AsBool(ctx) {
									typeName = "true"
								} else {
									typeName = "false"
								}
							default:
								typeName = className.GetType().TypeName()
							}
							return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("unserialize(): Option \"allowed_classes\" must be an array of class names, %s given", typeName))
						}
					}
				case phpv.ZtBool:
					deserializer.allowAllClasses = bool(arg.AsBool(ctx))
				default:
					// allowed_classes must be bool or array
					typeName := arg.GetType().TypeName()
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("unserialize(): Option \"allowed_classes\" must be of type array|bool, %s given", typeName))
				}
			}
		}
	}

	strData := string(str)
	result, nextOffset, err := deserializer.parse(ctx, strData)
	if err != nil {
		// If it's a thrown exception (Error/Exception), propagate it directly
		if _, ok := phpv.UnwrapError(err).(*phperr.PhpThrow); ok {
			return nil, err
		}
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

// validateAllowedClassName checks that a class name string in the allowed_classes
// option is valid. PHP throws ValueError for:
// - strings with leading/trailing whitespace
// - strings with embedded whitespace or newlines
// - strings starting with '$'
// - strings containing null bytes (PHP truncates at null byte in the error message)
func validateAllowedClassName(name string) error {
	// Null byte: truncate at null byte position for error message
	if nulIdx := strings.IndexByte(name, 0); nulIdx >= 0 {
		truncated := name[:nulIdx]
		return fmt.Errorf("unserialize(): Option \"allowed_classes\" must be an array of class names, \"%s\" given", truncated)
	}
	// Leading or trailing whitespace
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("unserialize(): Option \"allowed_classes\" must be an array of class names, \"%s\" given", name)
	}
	// Embedded whitespace (spaces, tabs, newlines)
	for _, r := range name {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f' {
			return fmt.Errorf("unserialize(): Option \"allowed_classes\" must be an array of class names, \"%s\" given", name)
		}
	}
	// Starts with '$'
	if len(name) > 0 && name[0] == '$' {
		return fmt.Errorf("unserialize(): Option \"allowed_classes\" must be an array of class names, \"%s\" given", name)
	}
	return nil
}

// serializeSeen tracks both arrays and objects to prevent infinite recursion
// in serialize, particularly when Serializable::serialize() calls serialize()
// internally with cross-referenced objects.
type serializeSeen struct {
	// activeArrays is the SET of arrays currently being serialized (on the call stack).
	// We use a slice for O(depth) lookup, which is fine for typical nesting depths.
	// This prevents detecting "cycles" due to COW-shared arrays across different paths.
	activeArrays []*phpv.ZArray
	// activeArrayRef maps each currently-active array to its reference slot index.
	// This enables circular-reference detection: when a PHP ref points to an active
	// array, we can emit R:N where N is the slot of the outer array.
	activeArrayRef map[*phpv.ZArray]int
	objects        map[phpv.ZObject]bool
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
		activeArrayRef: make(map[*phpv.ZArray]int),
		objects:        make(map[phpv.ZObject]bool),
		objRefs:        make(map[phpv.ZObject]int),
		valRefs:        make(map[*phpv.ZVal]int),
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

// refChaseInner follows a chain of reference wrappers to find the ultimate
// non-reference inner ZVal. This handles multi-level references like:
//   outerRef = NewZVal(v1) where v1 = {v: inner} (v1 is also a ref)
// Both v1 and outerRef have the same ultimate inner, so they represent
// the same PHP reference target.
func refChaseInner(z *phpv.ZVal) *phpv.ZVal {
	cur := z
	for {
		next := cur.RefTarget()
		if next == nil {
			return cur
		}
		if !next.IsRef() {
			return next
		}
		cur = next
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
		// Chase the reference chain to find the ultimate non-ref inner ZVal.
		// This handles cases where refs are stacked (e.g., outerRef -> v1 -> inner).
		ultimate := refChaseInner(rawZVal)
		if ultimate == nil || ultimate == rawZVal {
			return serializeWithDepth(ctx, rawZVal, depth, seen)
		}
		if refIdx, ok := seen.valRefs[ultimate]; ok {
			// This reference target was already serialized; produce R:N;
			return "R:" + strconv.Itoa(refIdx) + ";", nil
		}
		// If this is a PHP reference to an object that was already serialized (in objRefs),
		// produce R:N using the object's reference index (not r:N which is object identity).
		// In PHP: $obj->a = &$obj serializes as R:1 (PHP reference), not r:1 (object copy).
		if ultimate.GetType() == phpv.ZtObject {
			if obj := ultimate.AsObject(ctx); obj != nil {
				if refIdx, ok := seen.objRefs[obj]; ok {
					// Register in valRefs too so future refs also produce R:N
					seen.valRefs[ultimate] = refIdx
					return "R:" + strconv.Itoa(refIdx) + ";", nil
				}
			}
		}
		// First time seeing this reference target - register it before serializing
		nextIdx := seen.refCount + 1
		seen.valRefs[ultimate] = nextIdx
		// If the reference target is an array currently being serialized (circular ref),
		// temporarily remove it from the active stack so it can be serialized in full.
		// The inner self-references will produce R:N; via valRefs.
		// Note: we must use SameData comparison because Dup() (used when passing arrays
		// to functions like serialize()) creates a COW copy with a different *ZArray
		// pointer but sharing the same underlying data.
		if ultimate.GetType() == phpv.ZtArray {
			if arr := ultimate.AsArray(ctx); arr != nil {
				for i, activeArr := range seen.activeArrays {
					if activeArr == arr || activeArr.HashTable().SameData(arr.HashTable()) {
						// Temporarily remove from active stack
						seen.activeArrays = append(seen.activeArrays[:i], seen.activeArrays[i+1:]...)
						activeSlot := seen.activeArrayRef[activeArr]
						delete(seen.activeArrayRef, activeArr)
						result, err := serializeWithDepth(ctx, ultimate, depth, seen)
						// Restore to active stack (re-insert at same position)
						seen.activeArrays = append(seen.activeArrays[:i], append([]*phpv.ZArray{activeArr}, seen.activeArrays[i:]...)...)
						seen.activeArrayRef[activeArr] = activeSlot
						return result, err
					}
				}
			}
		}
		// Serialize the unwrapped value (serializeWithDepth will assign the same index)
		return serializeWithDepth(ctx, ultimate, depth, seen)
	}

	// Non-ref path: also check if rawZVal is itself in valRefs (meaning a ref
	// wrapper pointing to rawZVal was seen before this direct occurrence).
	// This can happen if a ref to this value was serialized before the value itself.
	if refIdx, ok := seen.valRefs[rawZVal]; ok {
		return "R:" + strconv.Itoa(refIdx) + ";", nil
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

		// Detect array cycles (circular references).
		// We only consider it a cycle if the array is CURRENTLY being iterated on the
		// active call stack. COW-shared arrays that appear via different object paths
		// are not cycles and should be serialized fresh.
		// Use SameData comparison to handle COW copies (Dup'd arrays share same underlying data).
		for _, activeArr := range seen.activeArrays {
			if activeArr == arr || activeArr.HashTable().SameData(arr.HashTable()) {
				// Circular reference: emit R:N referencing the outer array's slot.
				if slot, ok := seen.activeArrayRef[activeArr]; ok {
					return "R:" + strconv.Itoa(slot) + ";", nil
				}
				return "N;", nil
			}
		}
		slot := seen.nextRef() // array gets a reference slot
		seen.activeArrays = append(seen.activeArrays, arr)
		seen.activeArrayRef[arr] = slot
		defer func() {
			// Pop this array from the active stack
			seen.activeArrays = seen.activeArrays[:len(seen.activeArrays)-1]
			delete(seen.activeArrayRef, arr)
		}()

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

		// Lazy objects: serialize triggers initialization unless SKIP_INITIALIZATION_ON_SERIALIZE
		if zo, ok := obj.(*phpobj.ZObject); ok && zo.IsLazy() && !zo.SkipsInitOnSerialize() {
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
			seen.nextRef() // N; still consumes a reference slot
			return "N;", nil
		}
		seen.objects[obj] = true

		// Assign reference number for this object
		objRefIdx := seen.nextRef()
		// Register in objRefs so future encounters produce r:N;
		seen.objRefs[obj] = objRefIdx

		// PHP's array recursion guard is per-array-copy: when we enter an object context,
		// the parent's array guards no longer apply to the object's property values.
		// This is because in PHP, clone creates a separate copy of arrays (COW), so
		// an array accessed via an object property is a distinct logical copy.
		// Save and clear the active arrays for the duration of this object's serialization.
		savedActiveArrays := seen.activeArrays
		seen.activeArrays = nil
		defer func() { seen.activeArrays = savedActiveArrays }()

		// Handle __PHP_Incomplete_Class: serialize using the original class name,
		// excluding __PHP_Incomplete_Class_Name from the output
		if obj.GetClass() == phpobj.IncompleteClass {
			zobj := obj.(*phpobj.ZObject)
			// Get the original class name
			origClassName := "__PHP_Incomplete_Class"
			if cnVal, ok := zobj.HashTable().GetStringB("__PHP_Incomplete_Class_Name"); ok && cnVal != nil && !cnVal.IsNull() {
				origClassName = string(cnVal.AsString(ctx))
			}
			// Serialize all dynamic properties except __PHP_Incomplete_Class_Name
			var buf bytes.Buffer
			propCount := 0
			it := zobj.HashTable().NewIterator()
			for it.Valid(ctx) {
				key, _ := it.Key(ctx)
				keyStr := string(key.AsString(ctx))
				if keyStr == "__PHP_Incomplete_Class_Name" {
					it.Next(ctx)
					continue
				}
				v, _ := it.Current(ctx)
				sub := "s:" + strconv.Itoa(len(keyStr)) + ":\"" + keyStr + "\";"
				buf.WriteString(sub)
				sub2, err := serializeValue(ctx, v, depth+1, seen)
				if err != nil {
					return "", err
				}
				buf.WriteString(sub2)
				propCount++
				it.Next(ctx)
			}
			contents := buf.String()
			buf.Reset()
			buf.WriteString("O:" + strconv.Itoa(len(origClassName)) + ":\"" + origClassName + "\":" + strconv.Itoa(propCount) + ":")
			buf.WriteString("{")
			buf.WriteString(contents)
			buf.WriteString("}")
			return buf.String(), nil
		}

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

		// Check C-level DenySerialize handler (checked before PHP-level __serialize,
		// so subclass overrides of __serialize do not bypass this).
		for cl := obj.GetClass(); cl != nil; cl = cl.GetParent() {
			if h := cl.Handlers(); h != nil && h.DenySerialize {
				return "", phpobj.ThrowError(ctx, phpobj.Exception, fmt.Sprintf("Serialization of '%s' is not allowed", obj.GetClass().GetName()))
			}
		}

		// Anonymous classes cannot be serialized
		// Anonymous class names contain "@anonymous" (e.g. "class@anonymous\0file:line$0")
		if anonIdx := strings.Index(string(obj.GetClass().GetName()), "@anonymous"); anonIdx >= 0 {
			// Use only the prefix up to (and including "@anonymous") for the error message
			displayName := string(obj.GetClass().GetName())
			if nullIdx := strings.IndexByte(displayName, 0); nullIdx >= 0 {
				displayName = displayName[:nullIdx]
			}
			return "", phpobj.ThrowError(ctx, phpobj.Exception, fmt.Sprintf("Serialization of '%s' is not allowed", displayName))
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
			sleepSeen := make(map[string]bool) // keyed by mangled name
			for _, prop := range props.Iterate(ctx) {
				// PHP warns if __sleep() returns an array with non-string elements
				if prop.GetType() != phpv.ZtString {
					ctx.Warn("%s::__sleep() should return an array only containing the names of instance-variables to serialize", obj.GetClass().GetName())
				}
				// Cast non-string elements to string (PHP does this)
				propName := prop.AsString(ctx)
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
						if sleepSeen[mangledName] {
							ctx.Warn("\"%s\" is returned from __sleep() multiple times", propName)
							continue
						}
						sleepSeen[mangledName] = true
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
				if sleepSeen[mangledName] {
					ctx.Warn("\"%s\" is returned from __sleep() multiple times", propName)
					continue
				}
				sleepSeen[mangledName] = true

				// Check if the property actually exists in the object
				// (it may have been unset, even if it was declared)
				// Internal key format: private = "*ClassName:propName", public/protected = bare name
				var internalKey phpv.ZString
				if classProp.Modifiers.IsPrivate() {
					// Internal format: "*DeclClassName:propName"
					declClassName := string(zobj.GetDeclClassName(classProp))
					internalKey = phpv.ZString("*" + declClassName + ":" + string(classProp.VarName))
				} else {
					internalKey = classProp.VarName
				}
				if !zobj.HashTable().HasString(internalKey) {
					// Typed properties that were never initialized are silently skipped.
					// Untyped properties that were explicitly unset() produce a warning.
					if classProp.TypeHint == nil {
						ctx.Warn("\"%s\" returned as member variable from __sleep() but does not exist", propName)
					}
					continue
				}
				v := zobj.GetPropValue(classProp)

				sub := "s:" + strconv.Itoa(len(mangledName)) + ":\"" + mangledName + "\";"
				buf.WriteString(sub)

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
	// PHP only allows integer or string keys in arrays and object properties.
	// For compound types (arrays, objects, etc.), PHP detects the type byte immediately
	// and reports the error at the position of the opening '{' in the type header + 1.
	// This avoids recursively parsing complex structures only to reject them later.
	if offset < len(str) {
		typeChar := str[offset]
		switch typeChar {
		case 'a', 'O', 'C': // array, object, custom - these are always invalid as keys
			// Find the first '{' starting from offset and report error there + 1
			errOffset := offset
			for pos := offset; pos < len(str); pos++ {
				if str[pos] == '{' {
					errOffset = pos + 1
					break
				}
			}
			// We need nextOffset to point past the key so the caller can skip,
			// but since we're returning an error, just return offset as nextOffset.
			return nil, offset, &unserializeError{errOffset, len(str)}
		}
	}

	// Save ref count, parse, then restore (keys don't count as references)
	savedLen := len(d.refs)
	result, nextOffset, err = d.parse(ctx, str, offset)
	d.refs = d.refs[:savedLen]
	if err != nil {
		return result, nextOffset, err
	}
	// Also reject null, bool, float, and object/array types parsed from other type codes.
	if result != nil {
		t := result.GetType()
		if t != phpv.ZtInt && t != phpv.ZtString {
			// Find the first '{' in range for the error offset
			errOffset := nextOffset
			for pos := offset; pos < nextOffset && pos < len(str); pos++ {
				if str[pos] == '{' {
					errOffset = pos + 1
					break
				}
			}
			return nil, nextOffset, &unserializeError{errOffset, len(str)}
		}
	}
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
		// S:N:"<encoded>"; - escaped string format (deprecated in PHP 8.5)
		// N is the DECODED length; the encoded content may be longer (\xx escapes)
		ctx.Deprecated("Unserializing the 'S' format is deprecated")
		j := indexOf(str, ":", i)
		if j < 0 {
			return nil, offset, readError
		}
		s := str[i:j]
		decodedLen, err := parseLengthUnsigned(s)
		if err != nil || decodedLen < 0 {
			return nil, offset, readError
		}

		startQuote := j + 1
		content := j + 2

		if startQuote >= len(str) || core.StrIdx(str, startQuote) != '"'  {
			return nil, offset, readError
		}

		// Scan forward to find the closing ""; and decode simultaneously
		var buf bytes.Buffer
		k := content
		for buf.Len() < int(decodedLen) {
			if k >= len(str) {
				return nil, offset, readError
			}
			if str[k] == '\\' && k+2 < len(str) {
				hi := unhex(str[k+1])
				lo := unhex(str[k+2])
				buf.WriteByte(hi<<4 | lo)
				k += 3
			} else {
				buf.WriteByte(str[k])
				k++
			}
		}
		// Expect closing ";
		if k >= len(str) || str[k] != '"'  {
			return nil, offset, readError
		}
		if k+1 >= len(str) || str[k+1] != ';'  {
			return nil, offset, readError
		}
		val := phpv.ZString(buf.String()).ZVal()
		d.addRef(val)
		return val, k + 2, nil

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
		orig := d.getRef(int(index))
		if orig == nil {
			// Invalid reference index - report error at position after the ';'
			return nil, offset, &unserializeError{semicIndex + 1, len(str)}
		}
		// R: creates a PHP reference. MakeRef() converts the original slot
		// in-place so both the original array entry and this new entry share
		// the same inner reference cell. Then return the same pointer so the
		// array entry for this key also points to the same reference.
		orig.MakeRef()
		d.addRef(orig)
		return orig, semicIndex + 1, nil

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
			// Invalid reference index - report error at position after the ';'
			return nil, offset, &unserializeError{semicIndex + 1, len(str)}
		}
		// r: is only valid for objects (not arrays or scalars)
		if ref.GetType() != phpv.ZtObject {
			return nil, offset, &unserializeError{semicIndex + 1, len(str)}
		}
		// r: doesn't create a PHP reference, just shares the object identity.
		// We must return a NEW ZVal wrapping the same object so that reassigning
		// one slot doesn't affect another (PHP objects are shared by identity,
		// not by ZVal pointer).
		refCopy := phpv.NewZVal(ref.Value())
		d.addRef(refCopy)
		return refCopy, semicIndex + 1, nil

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

		// Check max depth before entering nested structure
		if d.maxDepth > 0 && d.currentDepth >= d.maxDepth {
			ctx.Warn("Maximum depth of %d exceeded. The depth limit can be changed using the max_depth unserialize() option or the unserialize_max_depth ini setting", d.maxDepth)
			return nil, i, &unserializeError{i, len(str)}
		}
		d.currentDepth++

		arr := phpv.NewZArray()
		// Register the array in refs before parsing its contents.
		// We must use the SAME ZVal later so that if R:N makes it a reference,
		// the returned value reflects that.
		arrZVal := arr.ZVal()
		d.addRef(arrZVal)

		for numItems > 0 {
			var key, value *phpv.ZVal
			key, i, err = d.parseKey(ctx, str, i)
			if err != nil {
				// Propagate PhpThrow exceptions (e.g. from autoloaders) directly
				if _, ok := phpv.UnwrapError(err).(*phperr.PhpThrow); ok {
					return nil, offset, err
				}
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
				// Propagate PhpThrow exceptions (e.g. from autoloaders) directly
				if _, ok := phpv.UnwrapError(err).(*phperr.PhpThrow); ok {
					return nil, offset, err
				}
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
			d.currentDepth--
			return nil, offset, &unserializeError{i, len(str)}
		}
		d.currentDepth--

		return arrZVal, i + 1, nil
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
		// Find the `:` that ends the numProps field.
		// Scan digit by digit so we can report the exact position of any invalid character.
		j = i
		for j < len(str) && str[j] >= '0' && str[j] <= '9' {
			j++
		}
		if j >= len(str) || str[j] != ':' {
			// Invalid character in numProps field - report position of the bad character
			return nil, offset, &unserializeError{j, len(str)}
		}
		if j == i {
			// Empty numProps (e.g. "O:2:"yy":: ") - error at position after the colon
			return nil, offset, &unserializeError{j + 1, len(str)}
		}
		numPropsStr := str[i:j]
		numProps, err := strconv.Atoi(numPropsStr)
		if err != nil || numProps < 0 {
			return nil, offset, &unserializeError{i, len(str)}
		}

		if core.StrIdx(str, j+1) != '{' {
			return nil, offset, &unserializeError{j + 1, len(str)}
		}

		// Sanity check: the remaining bytes (including closing '}') must be at least
		// as large as the number of properties claimed (each needs at least 1 byte).
		// This prevents huge allocation attempts without causing premature failures.
		if numProps > 0 {
			remaining := len(str) - (j + 1) // bytes from '{' to end of string
			if remaining <= 0 || numProps > remaining {
				return nil, offset, &unserializeError{j, len(str)}
			}
		}

		allowedClass := d.allowAllClasses
		if !allowedClass {
			// Case-insensitive class name matching
			_, allowedClass = d.allowedClasses[phpv.ZString(strings.ToLower(className))]
		}

		class, err := ctx.Global().GetClass(ctx, phpv.ZString(className), true)
		// If GetClass returned an error that is NOT a simple "class not found" error
		// (e.g. the autoloader threw an exception), propagate it immediately.
		if err != nil {
			if throw, ok := phpv.UnwrapError(err).(*phperr.PhpThrow); ok {
				// "Class not found" errors are generated by GetClass itself with class "Error"
				// and a message of the form: Class "X" not found
				// Autoloader exceptions have a different class or message, so propagate them.
				msg := throw.Obj.HashTable().GetString("message")
				msgStr := ""
				if msg != nil {
					msgStr = msg.String()
				}
				classNotFound := string(throw.Obj.GetClass().GetName()) == "Error" &&
					(msgStr == fmt.Sprintf("Class \"%s\" not found", className) ||
						msgStr == fmt.Sprintf("Class \"%s\" not found", className))
				if !classNotFound {
					return nil, offset, err
				}
				// Release the ID of the spurious Error object to keep object IDs compact.
				// Use the GetObjID interface if available.
				type objIDer interface{ GetObjID() int }
				if idObj, ok := throw.Obj.(objIDer); ok {
					ctx.Global().ReleaseObjectID(idObj.GetObjID())
				}
			}
		}
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
					// PHP throws an Error when callback is not found
					return nil, offset, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Invalid callback %s, function \"%s\" not found or invalid function name", cbName, cbName))
				}
			}
		}
		if err != nil || !allowedClass || class == nil {
			class = phpobj.IncompleteClass
		}

		// Class names with null bytes or anonymous class names cannot be unserialized
		// PHP generates "Error at offset 0" for these
		if strings.IndexByte(className, 0) >= 0 {
			return nil, offset, &unserializeError{0, len(str)}
		}
		// Anonymous classes cannot be unserialized
		if class != phpobj.IncompleteClass && strings.Contains(string(class.GetName()), "@anonymous") {
			class = phpobj.IncompleteClass
		}

		// Check if the class is an enum (enums cannot be unserialized with O:)
		if class.GetType().Has(phpv.ZClassTypeEnum) {
			return nil, offset, &unserializeError{0, len(str)}
		}

		// Check if the class is abstract (abstract classes cannot be instantiated)
		if class != phpobj.IncompleteClass && (class.GetType().Has(phpv.ZClassTypeExplicitAbstract) || class.GetType().Has(phpv.ZClassTypeImplicitAbstract)) {
			return nil, offset, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot instantiate abstract class %s", class.GetName()))
		}

		obj, err := phpobj.CreateZObject(ctx, class)
		if err != nil {
			return nil, offset, err
		}
		// Only add __PHP_Incomplete_Class_Name when the original class was NOT found
		// (i.e., the original serialized class name differs from __PHP_Incomplete_Class itself)
		if class == phpobj.IncompleteClass && className != "__PHP_Incomplete_Class" {
			obj.ObjectSet(ctx, phpv.ZStr("__PHP_Incomplete_Class_Name"), phpv.ZStr(className))
		}

		// Register object in refs before parsing properties (for back-references).
		// We must use the SAME ZVal later so that if r:N references it, it is the same pointer.
		objZVal := obj.ZVal()
		d.addRef(objZVal)

		// Check max depth before entering nested structure
		if d.maxDepth > 0 && d.currentDepth >= d.maxDepth {
			ctx.Warn("Maximum depth of %d exceeded. The depth limit can be changed using the max_depth unserialize() option or the unserialize_max_depth ini setting", d.maxDepth)
			return nil, i, &unserializeError{i, len(str)}
		}
		d.currentDepth++

		// Check C-level DenyUnserialize handler (checked before PHP-level __unserialize).
		for cl := obj.GetClass(); cl != nil; cl = cl.GetParent() {
			if h := cl.Handlers(); h != nil && h.DenyUnserialize {
				return nil, offset, phpobj.ThrowError(ctx, phpobj.Exception, fmt.Sprintf("Unserialization of '%s' is not allowed", obj.GetClass().GetName()))
			}
		}

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
					// Propagate PhpThrow exceptions directly
					if _, ok := phpv.UnwrapError(err).(*phperr.PhpThrow); ok {
						return nil, offset, err
					}
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
					// Propagate PhpThrow exceptions directly
					if _, ok := phpv.UnwrapError(err).(*phperr.PhpThrow); ok {
						return nil, offset, err
					}
					if ue, ok := err.(*unserializeError); ok {
						return nil, offset, ue
					}
					return nil, offset, &unserializeError{i, len(str)}
				}
				arr.OffsetSet(ctx, key, value)
				numProps--
			}
			method, _ := obj.GetClass().GetMethod(phpv.ZString("__unserialize"))
			callable := phpv.Callable(&phpv.MethodCallable{
				Callable: phpobj.WrapCallableWithName(method.Method, "__unserialize"),
				Class:    method.Class,
			})
			_, err := ctx.Global().CallZValInternal(ctx, callable, []*phpv.ZVal{arr.ZVal()}, obj)
			if err != nil {
				return nil, offset, err
			}
		} else {
			for numProps > 0 {
				var key, value *phpv.ZVal
				key, i, err = d.parseKey(ctx, str, i)
				if err != nil {
					// Propagate PhpThrow exceptions directly
					if _, ok := phpv.UnwrapError(err).(*phperr.PhpThrow); ok {
						return nil, offset, err
					}
					if ue, ok := err.(*unserializeError); ok {
						return nil, offset, ue
					}
					return nil, offset, &unserializeError{i, len(str)}
				}
				value, i, err = d.parse(ctx, str, i)
				if err != nil {
					// Propagate PhpThrow exceptions directly
					if _, ok := phpv.UnwrapError(err).(*phperr.PhpThrow); ok {
						return nil, offset, err
					}
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
		// Require closing '}'
		if i >= len(str) || str[i] != '}' {
			d.currentDepth--
			return nil, offset, &unserializeError{i, len(str)}
		}
		i++
		d.currentDepth--
		return objZVal, i, nil
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

		// Look up class BEFORE checking dataLen so we can warn about missing class first.
		allowedClass := d.allowAllClasses
		if !allowedClass {
			// Case-insensitive class name matching
			_, allowedClass = d.allowedClasses[phpv.ZString(strings.ToLower(className))]
		}

		class, err := ctx.Global().GetClass(ctx, phpv.ZString(className), true)
		if err != nil {
			// Release the spurious Error object's ID (class not found) to keep IDs compact.
			if throw, ok := phpv.UnwrapError(err).(*phperr.PhpThrow); ok {
				type objIDer interface{ GetObjID() int }
				if idObj, ok2 := throw.Obj.(objIDer); ok2 {
					ctx.Global().ReleaseObjectID(idObj.GetObjID())
				}
			}
		}
		if err != nil || !allowedClass || class == nil {
			class = phpobj.IncompleteClass
		}

		// Check C-level DenyUnserialize handler for C: format too
		if class != phpobj.IncompleteClass {
			for cl := class; cl != nil; cl = cl.GetParent() {
				if h := cl.Handlers(); h != nil && h.DenyUnserialize {
					return nil, offset, phpobj.ThrowError(ctx, phpobj.Exception, fmt.Sprintf("Unserialization of '%s' is not allowed", class.GetName()))
				}
			}
		}

		// Check if the class implements Serializable, has an unserialize method, or is incomplete
		_, classHasUnserializeMethod := class.GetMethod(phpv.ZString("unserialize"))
		hasUnserializer := class == phpobj.IncompleteClass || class.Implements(phpobj.Serializable) || classHasUnserializeMethod
		if !hasUnserializer {
			// Class exists but doesn't implement Serializable - warn and create object
			ctx.Warn("Class %s has no unserializer", class.GetName(), logopt.NoFuncName(true))
		}
		if class == phpobj.IncompleteClass {
			// Class not found at all - warn with __PHP_Incomplete_Class
			ctx.Warn("Class __PHP_Incomplete_Class has no unserializer", logopt.NoFuncName(true))
		}

		// Scan digit by digit for the data length field so we can report the exact position
		// of any invalid character (e.g., 'x' in '3x{...').
		j = i
		for j < len(str) && str[j] >= '0' && str[j] <= '9' {
			j++
		}
		if j >= len(str) || str[j] != ':' {
			return nil, offset, &unserializeError{j, len(str)}
		}
		if j == i {
			return nil, offset, &unserializeError{j + 1, len(str)}
		}
		dataLenStr := str[i:j]
		// Try parsing as int64 first; if it overflows, treat it as a very large number
		dataLen64, parseErr := strconv.ParseInt(dataLenStr, 10, 64)
		if parseErr != nil {
			// Could be an overflow (very large number) — negative sign not allowed
			if len(dataLenStr) > 0 && dataLenStr[0] == '-' {
				return nil, offset, &unserializeError{i, len(str)}
			}
			// dataLen is huge (overflows even uint64) — warn about insufficient data and return error
			available := len(str) - (j + 2)
			// Use a placeholder large number for the warning (actual value overflows any integer type)
			ctx.Warn("Insufficient data for unserializing - %d required, %d present", uint64(math.MaxUint64), available, logopt.NoFuncName(true))
			return nil, offset, &unserializeError{j + 2, len(str)}
		}
		if dataLen64 < 0 {
			return nil, offset, &unserializeError{i, len(str)}
		}
		dataLen := int(dataLen64)
		// Check for integer overflow or unreasonable size
		if int64(dataLen) != dataLen64 || dataLen > len(str) {
			// PHP emits a warning about insufficient data when dataLen is huge (without function name prefix)
			ctx.Warn("Insufficient data for unserializing - %d required, %d present", dataLen64, len(str)-(j+2), logopt.NoFuncName(true))
			return nil, offset, &unserializeError{j + 2, len(str)}
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

		obj, err := phpobj.CreateZObject(ctx, class)
		if err != nil {
			return nil, offset, err
		}
		// Add incomplete class name if needed
		if class == phpobj.IncompleteClass && className != "__PHP_Incomplete_Class" {
			obj.ObjectSet(ctx, phpv.ZStr("__PHP_Incomplete_Class_Name"), phpv.ZStr(className))
		}

		// Register object in refs.
		// We must use the SAME ZVal later so that if r:N references it, it is the same pointer.
		serializableZVal := obj.ZVal()
		d.addRef(serializableZVal)

		// Call the unserialize($data) method on the object (only if it has one)
		if hasUnserializer && class != phpobj.IncompleteClass {
			if method, ok := obj.GetClass().GetMethod(phpv.ZString("unserialize")); ok {
				callable := phpv.Callable(&phpv.MethodCallable{
					Callable: phpobj.WrapCallableWithName(method.Method, "unserialize"),
					Class:    method.Class,
				})
				_, err := ctx.Global().CallZValInternal(ctx, callable, []*phpv.ZVal{phpv.ZStr(data).ZVal()}, obj)
				if err != nil {
					return nil, offset, err
				}
			}
		}

		return serializableZVal, dataEnd + 1, nil
	}

	return nil, offset, readError
}

// unserializeSetProperty sets a property on an object during unserialization,
// handling PHP's property name mangling for visibility.
// Mangled names: "\0ClassName\0prop" for private, "\0*\0prop" for protected
//
// Internally, private properties are stored under the key "*ClassName:propName"
// in the object's hash table. Protected/public properties are stored under
// their plain name.
func unserializeSetProperty(ctx phpv.Context, obj phpv.ZObject, key phpv.ZString, value *phpv.ZVal) {
	keyStr := string(key)
	zobj, isZObj := obj.(*phpobj.ZObject)

	// Determine the actual property name (strip mangling) and whether it was
	// originally private (from a specific class) or protected/public.
	actualPropName := key
	origPrivateClass := "" // non-empty if \0ClassName\0prop format
	isProtectedMangled := false

	if len(keyStr) > 0 && keyStr[0] == '\x00' {
		// Find the second \x00
		secondNull := strings.IndexByte(keyStr[1:], '\x00')
		if secondNull >= 0 {
			classOrStar := keyStr[1 : secondNull+1]
			propName := phpv.ZString(keyStr[secondNull+2:])
			actualPropName = propName
			if classOrStar == "*" {
				isProtectedMangled = true
			} else {
				origPrivateClass = classOrStar
			}
		}
	}

	if !isZObj {
		// Non-ZObject: use normal ObjectSet
		obj.ObjectSet(ctx, key, value)
		return
	}

	// If there's an origPrivateClass, first try to find the property declared
	// exactly in that class (not in subclasses). This handles the case where
	// a class hierarchy has both a Base::private $id and Derived::protected $id.
	if origPrivateClass != "" {
		zclass, ok := obj.GetClass().(*phpobj.ZClass)
		if ok {
			// Walk hierarchy to find the specific class
			for cl := zclass; cl != nil; cl = func() *phpobj.ZClass {
				if p := cl.GetParent(); p != nil {
					c, _ := p.(*phpobj.ZClass)
					return c
				}
				return nil
			}() {
				if string(cl.GetName()) != origPrivateClass {
					continue
				}
				// Found the original declaring class; look for the property here
				for _, p := range cl.Props {
					if p.VarName == actualPropName {
						// Use its declared visibility in the original class
						if p.Modifiers.IsPrivate() {
							internalKey := phpobj.GetPrivatePropNameExt(cl, actualPropName)
							zobj.HashTable().ForceSetString(internalKey, value)
						} else {
							zobj.HashTable().ForceSetString(actualPropName, value)
						}
						return
					}
				}
				// Class found but property not there — fall through to regular lookup
				break
			}
		}
	}

	// Look up the property in the current class hierarchy by plain name
	prop, found := obj.GetClass().GetProp(actualPropName)
	if found {
		if prop.Modifiers.IsPrivate() {
			// If the serialized data came from a different class name (origPrivateClass)
			// that doesn't exist in the hierarchy, don't overwrite the declared private
			// property. PHP ignores the value in this case, keeping the default.
			if origPrivateClass != "" {
				// Check if origPrivateClass exists in the hierarchy
				classFound := false
				if zc, ok := obj.GetClass().(*phpobj.ZClass); ok {
					for cl := zc; cl != nil; cl = func() *phpobj.ZClass {
						if p := cl.GetParent(); p != nil {
							c, _ := p.(*phpobj.ZClass)
							return c
						}
						return nil
					}() {
						if string(cl.GetName()) == origPrivateClass {
							classFound = true
							break
						}
					}
				}
				if !classFound {
					// Class not in hierarchy — skip this property (keep default)
					return
				}
			}
			// Current class has it as private — store under *DeclaredClass:propName
			declClass := zobj.GetDeclClassName(prop)
			var declZClass phpv.ZClass = obj.GetClass()
			for declZClass != nil {
				if declZClass.GetName() == declClass {
					break
				}
				declZClass = declZClass.GetParent()
			}
			if declZClass == nil {
				declZClass = obj.GetClass()
			}
			internalKey := phpobj.GetPrivatePropNameExt(declZClass, actualPropName)
			zobj.HashTable().ForceSetString(internalKey, value)
		} else {
			// Protected/public — store under plain name
			zobj.HashTable().ForceSetString(actualPropName, value)
		}
		return
	}

	// Not a declared property: bypass handlers and store directly for __PHP_Incomplete_Class,
	// since its HandlePropSet would throw. For other classes, use ObjectSet to emit
	// the deprecation warning for dynamic property creation (PHP 8.1+).
	if isZObj && obj.GetClass() == phpobj.IncompleteClass {
		zobj.HashTable().ForceSetString(actualPropName, value)
		return
	}
	// If the original key used protected mangling (\0*\0propName) but the class doesn't declare
	// the property, still store it with protected mangling (so var_dump shows it as protected).
	if isProtectedMangled {
		protectedKey := phpv.ZString("\x00*\x00" + string(actualPropName))
		// Emit deprecation for dynamic property creation (PHP 8.2+) only when creating a new property.
		if isZObj && !zobj.AllowsDynamicProperties() {
			if _, exists := zobj.HashTable().GetStringB(protectedKey); !exists {
				hasMagicProp := false
				if zc, ok := zobj.GetClass().(*phpobj.ZClass); ok {
					_, hasGet := zc.Methods["__get"]
					_, hasSet := zc.Methods["__set"]
					hasMagicProp = hasGet || hasSet
				}
				if !hasMagicProp {
					ctx.Deprecated("Creation of dynamic property %s::$%s is deprecated",
						zobj.GetClass().GetName(), string(actualPropName), logopt.NoFuncName(true))
				}
			}
		}
		zobj.HashTable().ForceSetString(protectedKey, value)
		return
	}
	obj.ObjectSet(ctx, actualPropName, value)
}

// StreamDeserializer provides sequential unserialize operations with shared
// reference tracking. This is needed by SplObjectStorage::unserialize() and
// ArrayObject::unserialize() which parse multiple PHP values from a single
// serialized stream while maintaining cross-reference integrity.
type StreamDeserializer struct {
	d *deserializer
}

// NewStreamDeserializer creates a StreamDeserializer that allows all classes.
func NewStreamDeserializer() *StreamDeserializer {
	return &StreamDeserializer{
		d: &deserializer{
			allowAllClasses: true,
			allowedClasses:  map[phpv.ZString]struct{}{},
		},
	}
}

// NewStreamDeserializerWithRefs creates a StreamDeserializer pre-seeded with
// existing references. This is needed when a Serializable::unserialize() callback
// must resolve R:/r: references that point to objects registered by the parent
// deserializer (e.g. the object being unserialized itself).
func NewStreamDeserializerWithRefs(refs ...*phpv.ZVal) *StreamDeserializer {
	return &StreamDeserializer{
		d: &deserializer{
			allowAllClasses: true,
			allowedClasses:  map[phpv.ZString]struct{}{},
			refs:            refs,
		},
	}
}

// ParseAt unserializes a single PHP value starting at the given offset in str.
// It returns the parsed value and the offset immediately after the consumed data.
// Reference tracking is shared across all calls on the same StreamDeserializer.
func (sd *StreamDeserializer) ParseAt(ctx phpv.Context, str string, offset int) (*phpv.ZVal, int, error) {
	return sd.d.parse(ctx, str, offset)
}

// ParseKeyAt parses a value without registering it in the reference table (for keys).
func (sd *StreamDeserializer) ParseKeyAt(ctx phpv.Context, str string, offset int) (*phpv.ZVal, int, error) {
	return sd.d.parseKey(ctx, str, offset)
}

