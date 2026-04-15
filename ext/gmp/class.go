package gmp

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
	"github.com/MagicalTux/goro/ext/standard"
)

// > class GMP
var GMP = &phpobj.ZClass{
	Name: "GMP",
	Attr: phpv.ZClassFinal,
}

// namedMethod wraps a NativeMethod with a proper name for stack traces.
// This ensures that when serialize.go calls method.Method directly,
// the stack trace shows the correct method name instead of "__construct".
type namedMethod struct {
	phpobj.NativeMethod
	name string
}

func (n *namedMethod) Name() string { return n.name }
func (n *namedMethod) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return n.NativeMethod.Call(ctx, args)
}
func (n *namedMethod) GetType() phpv.ZType                       { return phpv.ZtCallable }
func (n *namedMethod) ZVal() *phpv.ZVal                          { return phpv.NewZVal(n) }
func (n *namedMethod) Value() phpv.Val                           { return n }
func (n *namedMethod) String() string                            { return "Callable" }
func (n *namedMethod) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	return n.NativeMethod.AsVal(ctx, t)
}

// getGMPInt extracts the *big.Int from a GMP object.
func getGMPInt(o phpv.ZObject) *big.Int {
	opaque := o.GetOpaque(GMP)
	if opaque == nil {
		return big.NewInt(0)
	}
	return opaque.(*big.Int)
}

// readOperand converts a ZVal to *big.Int for operator overloading.
// Supports GMP objects, integers, and integer strings.
func readOperand(ctx phpv.Context, v *phpv.ZVal) (*big.Int, error) {
	if v == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Number must be of type GMP|string|int, null given")
	}
	switch v.GetType() {
	case phpv.ZtObject:
		obj, ok := v.Value().(*phpobj.ZObject)
		if ok && obj.Class == GMP {
			return getGMPInt(obj), nil
		}
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Number must be of type GMP|string|int, %s given", obj.Class.GetName()))
	case phpv.ZtInt:
		return big.NewInt(int64(v.Value().(phpv.ZInt))), nil
	case phpv.ZtFloat:
		f := float64(v.Value().(phpv.ZFloat))
		// Check if float has fractional part
		intVal := int64(f)
		if float64(intVal) != f {
			// Deprecated: implicit conversion loses precision
			ctx.Deprecated("Implicit conversion from float %s to int loses precision", phpv.FormatFloatPrecision(f, -1))
		}
		return big.NewInt(intVal), nil
	case phpv.ZtString:
		s := string(v.Value().(phpv.ZString))
		s = strings.TrimSpace(s)
		i := &big.Int{}
		_, ok := i.SetString(s, 0)
		if !ok {
			// Try base 10 as fallback
			_, ok = i.SetString(s, 10)
			if !ok {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Number is not an integer string")
			}
		}
		return i, nil
	case phpv.ZtNull:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Number must be of type GMP|string|int, null given")
	case phpv.ZtBool:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Number must be of type GMP|string|int, bool given")
	case phpv.ZtArray:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Number must be of type GMP|string|int, array given")
	case phpv.ZtResource:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Number must be of type GMP|string|int, resource given")
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Number must be of type GMP|string|int, %s given", v.GetType()))
	}
}

func init() {
	GMP.H = &phpv.ZClassHandlers{
		HandleCast:        gmpHandleCast,
		HandleDoOperation: gmpHandleDoOperation,
		HandleCompare:     gmpHandleCompare,
	}

	GMP.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// GMP::__construct() accepts 0 or more args
				if len(args) == 0 {
					o.SetOpaque(GMP, big.NewInt(0))
					return nil, nil
				}

				var num *phpv.ZVal
				var base *phpv.ZInt

				_, err := core.Expand(ctx, args, &num, &base)
				if err != nil {
					return nil, err
				}

				// Validate base
				if base != nil {
					b := int(*base)
					if b != 0 && (b < 2 || b > 62) {
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "GMP::__construct(): Argument #2 ($base) must be 0 or between 2 and 62")
					}
				}

				if num == nil {
					// No arguments: zero
					o.SetOpaque(GMP, big.NewInt(0))
					return nil, nil
				}

				// Check if num is a GMP object - PHP disallows this
				if num.GetType() == phpv.ZtObject {
					if obj, ok := num.Value().(*phpobj.ZObject); ok && obj.Class == GMP {
						return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "gmp_init(): Argument #1 ($num) must be of type string|int, GMP given")
					}
				}

				var i *big.Int

				switch num.GetType() {
				case phpv.ZtNull, phpv.ZtBool, phpv.ZtInt, phpv.ZtFloat:
					num, err = num.As(ctx, phpv.ZtInt)
					if err != nil {
						return nil, err
					}
					i = big.NewInt(int64(num.Value().(phpv.ZInt)))
				default:
					num, err = num.As(ctx, phpv.ZtString)
					if err != nil {
						return nil, err
					}
					s := string(num.AsString(ctx))
					s = strings.TrimSpace(s)
					i = &big.Int{}
					b := 0
					if base != nil {
						b = int(*base)
					}
					if s == "" {
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "GMP::__construct(): Argument #1 ($num) is not an integer string")
					}
					// PHP does not accept leading '+' sign
					if strings.HasPrefix(s, "+") {
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "GMP::__construct(): Argument #1 ($num) is not an integer string")
					}
					// Allow matching prefix with explicit base
					parseStr := s
					parseBase := b
					if b != 0 {
						neg := strings.HasPrefix(s, "-")
						body := s
						if neg {
							body = s[1:]
						}
						// PHP GMP allows whitespace throughout the string - strip internal whitespace
						body = stripWhitespace(body)
						switch b {
						case 2:
							if strings.HasPrefix(strings.ToLower(body), "0b") {
								body = body[2:]
								parseBase = 2
							}
						case 16:
							if strings.HasPrefix(strings.ToLower(body), "0x") {
								body = body[2:]
								parseBase = 16
							}
						case 8:
							if strings.HasPrefix(strings.ToLower(body), "0o") {
								body = body[2:]
								parseBase = 8
							} else if strings.HasPrefix(body, "0") && len(body) > 1 {
								body = body[1:]
								parseBase = 8
							}
						}
						if neg {
							parseStr = "-" + body
						} else {
							parseStr = body
						}
					}
					_, ok := i.SetString(parseStr, parseBase)
					if !ok {
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "GMP::__construct(): Argument #1 ($num) is not an integer string")
					}
				}

				o.SetOpaque(GMP, i)
				return nil, nil
			}),
		},
		"__tostring": {
			Name: "__toString",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				i := getGMPInt(o)
				return phpv.ZString(i.String()).ZVal(), nil
			}),
		},
		"__debuginfo": {
			Name: "__debugInfo",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				i := getGMPInt(o)
				arr := phpv.NewZArray()
				// Include dynamic properties first, then the GMP numeric value
				ht := o.HashTable()
				it := ht.NewIterator()
				for it.Valid(ctx) {
					k, _ := it.Key(ctx)
					v, _ := it.Current(ctx)
					arr.OffsetSet(ctx, k, v)
					it.Next(ctx)
				}
				arr.OffsetSet(ctx, phpv.ZString("num").ZVal(), phpv.ZString(i.String()).ZVal())
				return arr.ZVal(), nil
			}),
		},
		"__serialize": {
			Name:      "__serialize",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				i := getGMPInt(o)
				arr := phpv.NewZArray()
				// GMP serializes the number as hex string at index 0
				hexStr := fmt.Sprintf("%x", i)
				if i.Sign() < 0 {
					hexStr = fmt.Sprintf("-%x", new(big.Int).Abs(i))
				}
				arr.OffsetSet(ctx, phpv.ZInt(0).ZVal(), phpv.ZString(hexStr).ZVal())

				// Also serialize any dynamic properties at index 1 as sub-array
				dynProps := phpv.NewZArray()
				hasDynProps := false
				for prop := range o.IterProps(ctx) {
					if prop.VarName == "" {
						continue
					}
					v := o.GetPropValue(prop)
					dynProps.OffsetSet(ctx, prop.VarName.ZVal(), v)
					hasDynProps = true
				}
				// Also check for dynamic properties in hash table
				ht := o.HashTable()
				it := ht.NewIterator()
				for it.Valid(ctx) {
					k, _ := it.Key(ctx)
					v, _ := it.Current(ctx)
					if k.Value().GetType() == phpv.ZtString {
						dynProps.OffsetSet(ctx, k.Value(), v)
						hasDynProps = true
					}
					it.Next(ctx)
				}
				if hasDynProps {
					arr.OffsetSet(ctx, phpv.ZInt(1).ZVal(), dynProps.ZVal())
				}

				return arr.ZVal(), nil
			}),
		},
		"unserialize": {
			Name:      "unserialize",
			Modifiers: phpv.ZAttrPublic,
			Method: &namedMethod{name: "unserialize", NativeMethod: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// Legacy Serializable::unserialize() for C: format backward compatibility.
				// The data format is: s:N:"hex_or_decimal_number";a:M:{...properties...}
				var data *phpv.ZVal
				_, err := core.Expand(ctx, args, &data)
				if err != nil {
					return nil, err
				}
				if data == nil || data.GetType() != phpv.ZtString {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				rawData := string(data.AsString(ctx))

				// Step 1: Parse the number part: s:N:"value";
				if len(rawData) < 2 || rawData[0] != 's' || rawData[1] != ':' {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				colonIdx := strings.Index(rawData[2:], ":")
				if colonIdx < 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				lenStr := rawData[2 : 2+colonIdx]
				numLen := 0
				for _, c := range lenStr {
					if c >= '0' && c <= '9' {
						numLen = numLen*10 + int(c-'0')
					} else {
						return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
					}
				}
				afterColon := 2 + colonIdx + 1 // position after the colon following length
				if afterColon+numLen+2 > len(rawData) ||
					rawData[afterColon] != '"' || rawData[afterColon+numLen+1] != '"' {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				numStr := rawData[afterColon+1 : afterColon+1+numLen]
				if numStr == "" {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				i := &big.Int{}
				// Try parsing as decimal first (legacy format), then hex
				if _, ok := i.SetString(numStr, 10); !ok {
					if _, ok := i.SetString(numStr, 16); !ok {
						return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
					}
				}
				// afterNumber points to the char after the closing '"' and ';'
				// i.e., afterColon+numLen+2 is the closing '"', +1 is ';'
				afterNumber := afterColon + numLen + 2 // position of ';'
				if afterNumber >= len(rawData) || rawData[afterNumber] != ';' {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				afterNumber++ // skip ';'

				// Step 2: Parse the properties array: a:N:{...}
				// The rest of rawData should be an array serialization.
				rest := rawData[afterNumber:]
				if len(rest) == 0 || rest[0] != 'a' {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize properties")
				}
				// Parse a:N:{...} minimally to extract properties
				// a:N:{...} - N items in array
				if len(rest) < 4 || rest[1] != ':' {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize properties")
				}
				// Find the count and opening brace
				braceIdx := strings.Index(rest[2:], ":{")
				if braceIdx < 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize properties")
				}
				// Verify the closing brace
				if rest[len(rest)-1] != '}' {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize properties")
				}

				// Set the GMP value
				o.SetOpaque(GMP, i)

				// Parse properties from the inner content of a:N:{...}
				inner := rest[2+braceIdx+2 : len(rest)-1]
				if len(inner) > 0 {
					// Use the standard PHP deserializer to parse the inner content as pairs
					d := standard.NewStreamDeserializerWithRefs(o.ZVal())
					pos := 0
					for pos < len(inner) {
						// Parse key
						kv, nextPos, kErr := d.ParseKeyAt(ctx, inner, pos)
						if kErr != nil || kv == nil {
							break
						}
						pos = nextPos
						// Parse value
						vv, nextPos2, vErr := d.ParseAt(ctx, inner, pos)
						if vErr != nil || vv == nil {
							break
						}
						pos = nextPos2
						// Set as dynamic property directly (bypass deprecation
						// check since PHP's GMP unserialize allows dynamic props)
						keyStr, _ := kv.Value().AsVal(ctx, phpv.ZtString)
						o.HashTable().SetString(keyStr.(phpv.ZString), vv)
					}
				}
				return nil, nil
			})},
		},
		"__unserialize": {
			Name:      "__unserialize",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				var data *phpv.ZVal
				_, err := core.Expand(ctx, args, &data)
				if err != nil {
					return nil, err
				}
				if data.GetType() != phpv.ZtArray {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				arr := data.AsArray(ctx)

				// Index 0 should be the hex string
				numVal, _ := arr.OffsetGet(ctx, phpv.ZInt(0).ZVal())
				if numVal == nil || numVal.GetType() != phpv.ZtString {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				hexStr := string(numVal.AsString(ctx))
				if hexStr == "" {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				i := &big.Int{}
				_, ok := i.SetString(hexStr, 16)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize number")
				}
				o.SetOpaque(GMP, i)

				// Index 1 should be dynamic properties (optional)
				propsVal, _ := arr.OffsetGet(ctx, phpv.ZInt(1).ZVal())
				if propsVal != nil && propsVal.GetType() == phpv.ZtArray {
					propsArr := propsVal.AsArray(ctx)
					for k, v := range propsArr.Iterate(ctx) {
						switch k.GetType() {
						case phpv.ZtString:
							o.ObjectSet(ctx, k.Value(), v)
						case phpv.ZtInt:
							// Integer-keyed dynamic property: convert key to string name
							o.ObjectSet(ctx, phpv.ZString(fmt.Sprintf("%d", k.Value().(phpv.ZInt))), v)
						}
					}
				} else if propsVal != nil && !propsVal.IsNull() {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Could not unserialize properties")
				}

				return nil, nil
			}),
		},
	}
}

// gmpHandleCast handles (int), (float), (bool) casts for GMP objects.
func gmpHandleCast(ctx phpv.Context, o phpv.ZObject, t phpv.ZType) (phpv.Val, error) {
	i := getGMPInt(o)
	switch t {
	case phpv.ZtInt:
		return phpv.ZInt(i.Int64()), nil
	case phpv.ZtFloat:
		f, _ := new(big.Float).SetInt(i).Float64()
		return phpv.ZFloat(f), nil
	case phpv.ZtBool:
		return phpv.ZBool(i.Sign() != 0), nil
	default:
		return nil, fmt.Errorf("unsupported cast to %s", t)
	}
}

// gmpHandleCompare handles comparison between two GMP objects.
// Both a and b must be GMP objects. If either is not GMP, this throws an appropriate error.
func gmpHandleCompare(ctx phpv.Context, a, b phpv.ZObject) (int, error) {
	// Validate that a is a GMP object
	aObj, aOk := a.(*phpobj.ZObject)
	if !aOk || aObj.Class != GMP {
		typeName := "object"
		if aOk {
			typeName = string(aObj.Class.GetName())
		}
		return 0, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Number must be of type GMP|string|int, %s given", typeName))
	}
	// Validate that b is a GMP object
	bObj, bOk := b.(*phpobj.ZObject)
	if !bOk || bObj.Class != GMP {
		typeName := "object"
		if bOk {
			typeName = string(bObj.Class.GetName())
		}
		return 0, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Number must be of type GMP|string|int, %s given", typeName))
	}
	ia := getGMPInt(aObj)
	ib := getGMPInt(bObj)
	return ia.Cmp(ib), nil
}

// gmpTypeName returns the type name for an invalid operand error message.
func gmpTypeName(v *phpv.ZVal) string {
	if v == nil {
		return "null"
	}
	switch v.GetType() {
	case phpv.ZtArray:
		return "array"
	case phpv.ZtNull:
		return "null"
	case phpv.ZtBool:
		return "bool"
	case phpv.ZtResource:
		return "resource"
	case phpv.ZtObject:
		if obj, ok := v.Value().(*phpobj.ZObject); ok {
			return string(obj.Class.GetName())
		}
		return "object"
	default:
		return v.GetType().String()
	}
}

// gmpHandleDoOperation handles arithmetic/bitwise operator overloading for GMP.
func gmpHandleDoOperation(ctx phpv.Context, op int, a, b *phpv.ZVal) (*phpv.ZVal, error) {
	itemOp := tokenizer.ItemType(op)

	// Handle unary operators (a == nil)
	if a == nil {
		ib, err := readOperand(ctx, b)
		if err != nil {
			return nil, err
		}
		switch itemOp {
		case tokenizer.Rune('-'):
			r := new(big.Int).Neg(ib)
			return returnInt(ctx, r)
		case tokenizer.Rune('+'):
			r := new(big.Int).Set(ib)
			return returnInt(ctx, r)
		case tokenizer.Rune('~'):
			r := new(big.Int).Not(ib)
			return returnInt(ctx, r)
		default:
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Unsupported operand types: GMP %s GMP", itemOp.OpString()))
		}
	}

	// For power and shift operators, invalid operand types throw "Unsupported operand types" instead of "Number must be of type..."
	switch itemOp {
	case tokenizer.T_POW, tokenizer.T_POW_EQUAL,
		tokenizer.T_SL, tokenizer.T_SL_EQUAL,
		tokenizer.T_SR, tokenizer.T_SR_EQUAL:
		// Determine type names for the error message
		aTypeName := "GMP"
		if a != nil {
			if a.GetType() != phpv.ZtObject {
				aTypeName = gmpTypeName(a)
			} else if obj, ok := a.Value().(*phpobj.ZObject); !ok || obj.Class != GMP {
				aTypeName = gmpTypeName(a)
			}
		}
		bTypeName := gmpTypeName(b)
		// Check if b is an unsupported type (not GMP, int, float, string)
		if b != nil {
			switch b.GetType() {
			case phpv.ZtArray, phpv.ZtNull, phpv.ZtResource, phpv.ZtBool:
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Unsupported operand types: %s %s %s", aTypeName, itemOp.OpString(), bTypeName))
			case phpv.ZtObject:
				if obj, ok := b.Value().(*phpobj.ZObject); !ok || obj.Class != GMP {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("Unsupported operand types: %s %s %s", aTypeName, itemOp.OpString(), bTypeName))
				}
			}
		}
	}

	// For comparison operators, null is treated as 0 (PHP's standard comparison semantics)
	isComparison := false
	switch itemOp {
	case tokenizer.Rune('<'), tokenizer.T_IS_SMALLER_OR_EQUAL,
		tokenizer.Rune('>'), tokenizer.T_IS_GREATER_OR_EQUAL,
		tokenizer.T_IS_EQUAL, tokenizer.T_IS_NOT_EQUAL,
		tokenizer.T_SPACESHIP, tokenizer.T_IS_IDENTICAL, tokenizer.T_IS_NOT_IDENTICAL:
		isComparison = true
	}

	var ia, ib *big.Int
	var err error

	if isComparison && a != nil && a.GetType() == phpv.ZtNull {
		ia = big.NewInt(0)
	} else {
		ia, err = readOperand(ctx, a)
		if err != nil {
			return nil, err
		}
	}

	// For ** and shift operators, convert float without emitting a Deprecated warning
	noDeprecatedFloat := itemOp == tokenizer.T_POW || itemOp == tokenizer.T_POW_EQUAL ||
		itemOp == tokenizer.T_SL || itemOp == tokenizer.T_SL_EQUAL ||
		itemOp == tokenizer.T_SR || itemOp == tokenizer.T_SR_EQUAL
	if noDeprecatedFloat {
		if b != nil && b.GetType() == phpv.ZtFloat {
			f := float64(b.Value().(phpv.ZFloat))
			ib = big.NewInt(int64(f))
		} else {
			ib, err = readOperand(ctx, b)
			if err != nil {
				return nil, err
			}
		}
	} else if isComparison && b != nil && b.GetType() == phpv.ZtNull {
		ib = big.NewInt(0)
	} else {
		ib, err = readOperand(ctx, b)
		if err != nil {
			return nil, err
		}
	}

	// For compound assignment operators (+=, -=, <<=, etc.), modify the
	// original GMP object's big.Int in place to avoid creating a new object.
	// This prevents memory exhaustion when doing e.g. $gmp <<= 1 in a loop,
	// since Go's GC doesn't notify the PHP-level memory tracker when objects
	// are collected.
	isCompound := false
	switch itemOp {
	case tokenizer.T_PLUS_EQUAL, tokenizer.T_MINUS_EQUAL, tokenizer.T_MUL_EQUAL,
		tokenizer.T_DIV_EQUAL, tokenizer.T_MOD_EQUAL, tokenizer.T_POW_EQUAL,
		tokenizer.T_OR_EQUAL, tokenizer.T_AND_EQUAL, tokenizer.T_XOR_EQUAL,
		tokenizer.T_SL_EQUAL, tokenizer.T_SR_EQUAL:
		if a != nil && a.GetType() == phpv.ZtObject {
			if obj, ok := a.Value().(*phpobj.ZObject); ok && obj.Class == GMP {
				isCompound = true
			}
		}
	}

	var r *big.Int
	if isCompound {
		// Reuse ia directly (it's the *big.Int from the GMP object's opaque data)
		r = ia
	} else {
		r = new(big.Int)
	}

	switch itemOp {
	case tokenizer.Rune('+'), tokenizer.T_PLUS_EQUAL:
		r.Add(ia, ib)
	case tokenizer.Rune('-'), tokenizer.T_MINUS_EQUAL:
		r.Sub(ia, ib)
	case tokenizer.Rune('*'), tokenizer.T_MUL_EQUAL:
		r.Mul(ia, ib)
	case tokenizer.Rune('/'), tokenizer.T_DIV_EQUAL:
		if ib.Sign() == 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "Division by zero")
		}
		r.Quo(ia, ib)
	case tokenizer.Rune('%'), tokenizer.T_MOD_EQUAL:
		if ib.Sign() == 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.DivisionByZeroError, "Modulo by zero")
		}
		r.Rem(ia, ib)
	case tokenizer.T_POW, tokenizer.T_POW_EQUAL:
		if ib.Sign() < 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Exponent must be greater than or equal to 0")
		}
		r.Exp(ia, ib, nil)
	case tokenizer.Rune('|'), tokenizer.T_OR_EQUAL:
		r.Or(ia, ib)
	case tokenizer.Rune('&'), tokenizer.T_AND_EQUAL:
		r.And(ia, ib)
	case tokenizer.Rune('^'), tokenizer.T_XOR_EQUAL:
		r.Xor(ia, ib)
	case tokenizer.T_SL, tokenizer.T_SL_EQUAL:
		if ib.Sign() < 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Shift must be greater than or equal to 0")
		}
		r.Lsh(ia, uint(ib.Int64()))
	case tokenizer.T_SR, tokenizer.T_SR_EQUAL:
		if ib.Sign() < 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Shift must be greater than or equal to 0")
		}
		r.Rsh(ia, uint(ib.Int64()))
	case tokenizer.Rune('<'), tokenizer.T_IS_SMALLER_OR_EQUAL,
		tokenizer.Rune('>'), tokenizer.T_IS_GREATER_OR_EQUAL,
		tokenizer.T_IS_EQUAL, tokenizer.T_IS_NOT_EQUAL,
		tokenizer.T_SPACESHIP, tokenizer.T_IS_IDENTICAL, tokenizer.T_IS_NOT_IDENTICAL:
		// Comparison operators: convert both sides to GMP, then compare.
		// If conversion fails (e.g. non-numeric string), error propagates.
		cmp := ia.Cmp(ib)
		var res bool
		switch itemOp {
		case tokenizer.Rune('<'):
			res = cmp < 0
		case tokenizer.T_IS_SMALLER_OR_EQUAL:
			res = cmp <= 0
		case tokenizer.Rune('>'):
			res = cmp > 0
		case tokenizer.T_IS_GREATER_OR_EQUAL:
			res = cmp >= 0
		case tokenizer.T_IS_EQUAL, tokenizer.T_IS_IDENTICAL:
			res = cmp == 0
		case tokenizer.T_IS_NOT_EQUAL, tokenizer.T_IS_NOT_IDENTICAL:
			res = cmp != 0
		case tokenizer.T_SPACESHIP:
			return phpv.ZInt(phpv.ZInt(cmp)).ZVal(), nil
		}
		return phpv.ZBool(res).ZVal(), nil
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Unsupported operand types: GMP %s GMP", itemOp.OpString()))
	}

	// For compound assignment, the big.Int was modified in place on the
	// existing GMP object. Return the original ZVal so no new object is
	// allocated and no extra memory is tracked.
	if isCompound {
		return a, nil
	}
	return returnInt(ctx, r)
}
