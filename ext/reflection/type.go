package reflection

import (
	"strings"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

var ReflectionType *phpobj.ZClass
var ReflectionNamedType *phpobj.ZClass
var ReflectionUnionType *phpobj.ZClass
var ReflectionIntersectionType *phpobj.ZClass

type reflectionTypeData struct {
	name     phpv.ZString
	nullable bool
	builtin  bool
}
type reflectionUnionTypeData struct {
	types    []*phpv.TypeHint
	nullable bool
}
type reflectionIntersectionTypeData struct {
	types []*phpv.TypeHint
}

func initReflectionType() {
	ReflectionType = &phpobj.ZClass{Name: "ReflectionType", Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"allowsnull": {Name: "allowsNull", Method: phpobj.NativeMethod(reflectionTypeAllowsNull)},
		"__tostring": {Name: "__toString", Method: phpobj.NativeMethod(reflectionTypeToString)},
	}}
	ReflectionNamedType = &phpobj.ZClass{Name: "ReflectionNamedType", Extends: ReflectionType, Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"allowsnull": {Name: "allowsNull", Method: phpobj.NativeMethod(reflectionTypeAllowsNull)},
		"__tostring": {Name: "__toString", Method: phpobj.NativeMethod(reflectionTypeToString)},
		"getname":    {Name: "getName", Method: phpobj.NativeMethod(reflectionNamedTypeGetName)},
		"isbuiltin":  {Name: "isBuiltin", Method: phpobj.NativeMethod(reflectionNamedTypeIsBuiltin)},
	}}
	ReflectionUnionType = &phpobj.ZClass{Name: "ReflectionUnionType", Extends: ReflectionType, Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"allowsnull": {Name: "allowsNull", Method: phpobj.NativeMethod(reflectionUnionTypeAllowsNull)},
		"__tostring": {Name: "__toString", Method: phpobj.NativeMethod(reflectionUnionTypeToString)},
		"gettypes":   {Name: "getTypes", Method: phpobj.NativeMethod(reflectionUnionTypeGetTypes)},
	}}
	ReflectionIntersectionType = &phpobj.ZClass{Name: "ReflectionIntersectionType", Extends: ReflectionType, Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"allowsnull": {Name: "allowsNull", Method: phpobj.NativeMethod(reflectionIntersectionTypeAllowsNull)},
		"__tostring": {Name: "__toString", Method: phpobj.NativeMethod(reflectionIntersectionTypeToString)},
		"gettypes":   {Name: "getTypes", Method: phpobj.NativeMethod(reflectionIntersectionTypeGetTypes)},
	}}
}

func createReflectionTypeObject(ctx phpv.Context, hint *phpv.TypeHint) (*phpv.ZVal, error) {
	if hint == nil { return nil, nil }
	if len(hint.Union) > 0 {
		if len(hint.Union) == 1 && len(hint.Union[0].Intersection) > 0 {
			return createReflectionIntersectionTypeObject(ctx, hint.Union[0])
		}
		if len(hint.Union) == 2 {
			var nullIdx, otherIdx int = -1, -1
			for i, u := range hint.Union {
				if u.Type() == phpv.ZtNull && len(u.Union) == 0 && len(u.Intersection) == 0 { nullIdx = i } else if len(u.Union) == 0 && len(u.Intersection) == 0 { otherIdx = i }
			}
			if nullIdx >= 0 && otherIdx >= 0 {
				normalized := &phpv.TypeHint{}
				*normalized = *hint.Union[otherIdx]
				normalized.Nullable = true
				return createReflectionNamedTypeObject(ctx, normalized)
			}
		}
		return createReflectionUnionTypeObject(ctx, hint)
	}
	if len(hint.Intersection) > 0 { return createReflectionIntersectionTypeObject(ctx, hint) }
	return createReflectionNamedTypeObject(ctx, hint)
}

func createReflectionNamedTypeObject(ctx phpv.Context, hint *phpv.TypeHint) (*phpv.ZVal, error) {
	data := &reflectionTypeData{nullable: hint.Nullable}
	// For reflection, use ClassName() for special types like "iterable" instead of String()
	// which may expand iterable to "Traversable|array"
	typeName := hint.String()
	if hint.ClassName() == "iterable" {
		typeName = "iterable"
	}
	if hint.Nullable && len(typeName) > 0 && typeName[0] == '?' { typeName = typeName[1:] }
	data.name = phpv.ZString(typeName)
	switch hint.Type() {
	case phpv.ZtBool, phpv.ZtInt, phpv.ZtFloat, phpv.ZtString, phpv.ZtArray, phpv.ZtVoid, phpv.ZtNever, phpv.ZtMixed:
		data.builtin = true
	case phpv.ZtNull:
		data.builtin = true
		data.nullable = true // null type always allows null
	case phpv.ZtObject:
		if hint.ClassName() == "" || hint.ClassName() == "callable" || hint.ClassName() == "iterable" { data.builtin = true }
	}
	obj, err := phpobj.NewZObjectOpaque(ctx, ReflectionNamedType, data)
	if err != nil { return nil, err }
	return obj.ZVal(), nil
}
func createReflectionUnionTypeObject(ctx phpv.Context, hint *phpv.TypeHint) (*phpv.ZVal, error) {
	data := &reflectionUnionTypeData{types: hint.Union, nullable: hint.IsNullable()}
	obj, err := phpobj.NewZObjectOpaque(ctx, ReflectionUnionType, data)
	if err != nil { return nil, err }
	return obj.ZVal(), nil
}
func createReflectionIntersectionTypeObject(ctx phpv.Context, hint *phpv.TypeHint) (*phpv.ZVal, error) {
	data := &reflectionIntersectionTypeData{types: hint.Intersection}
	obj, err := phpobj.NewZObjectOpaque(ctx, ReflectionIntersectionType, data)
	if err != nil { return nil, err }
	return obj.ZVal(), nil
}
func getTypeData(o *phpobj.ZObject) *reflectionTypeData {
	v := o.GetOpaque(ReflectionNamedType); if v == nil { v = o.GetOpaque(ReflectionType) }; if v == nil { return nil }; return v.(*reflectionTypeData)
}
func getUnionTypeData(o *phpobj.ZObject) *reflectionUnionTypeData {
	v := o.GetOpaque(ReflectionUnionType); if v == nil { return nil }; return v.(*reflectionUnionTypeData)
}
func getIntersectionTypeData(o *phpobj.ZObject) *reflectionIntersectionTypeData {
	v := o.GetOpaque(ReflectionIntersectionType); if v == nil { return nil }; return v.(*reflectionIntersectionTypeData)
}
func reflectionTypeAllowsNull(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getTypeData(o); if data == nil { return phpv.ZBool(false).ZVal(), nil }; return phpv.ZBool(data.nullable).ZVal(), nil
}
func reflectionTypeToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getTypeData(o); if data == nil { return phpv.ZString("").ZVal(), nil }
	name := data.name; if data.nullable { name = "?" + name }; return name.ZVal(), nil
}
func reflectionNamedTypeGetName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getTypeData(o); if data == nil { return phpv.ZString("").ZVal(), nil }; return data.name.ZVal(), nil
}
func reflectionNamedTypeIsBuiltin(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getTypeData(o); if data == nil { return phpv.ZBool(false).ZVal(), nil }; return phpv.ZBool(data.builtin).ZVal(), nil
}
func reflectionUnionTypeAllowsNull(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getUnionTypeData(o); if data == nil { return phpv.ZBool(false).ZVal(), nil }; return phpv.ZBool(data.nullable).ZVal(), nil
}
func reflectionUnionTypeToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getUnionTypeData(o); if data == nil { return phpv.ZString("").ZVal(), nil }
	parts := make([]string, 0, len(data.types))
	for _, t := range data.types {
		if t.Type() == phpv.ZtObject && t.ClassName() == "iterable" && len(t.Union) == 0 && len(t.Intersection) == 0 { parts = append(parts, "Traversable", "array"); continue }
		s := t.String(); if len(t.Intersection) > 0 { s = "(" + s + ")" }; parts = append(parts, s)
	}
	return phpv.ZString(strings.Join(parts, "|")).ZVal(), nil
}
func reflectionUnionTypeGetTypes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getUnionTypeData(o); if data == nil { return phpv.NewZArray().ZVal(), nil }
	arr := phpv.NewZArray()
	for _, t := range data.types {
		if t.Type() == phpv.ZtObject && t.ClassName() == "iterable" && len(t.Union) == 0 && len(t.Intersection) == 0 {
			for _, expanded := range []*phpv.TypeHint{phpv.ParseTypeHint("Traversable"), phpv.ParseTypeHint("array")} {
				val, err := createReflectionTypeObject(ctx, expanded); if err != nil { return nil, err }; if val != nil { arr.OffsetSet(ctx, nil, val) }
			}; continue
		}
		val, err := createReflectionTypeObject(ctx, t); if err != nil { return nil, err }; if val != nil { arr.OffsetSet(ctx, nil, val) }
	}
	return arr.ZVal(), nil
}
func reflectionIntersectionTypeAllowsNull(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) { return phpv.ZBool(false).ZVal(), nil }
func reflectionIntersectionTypeToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getIntersectionTypeData(o); if data == nil { return phpv.ZString("").ZVal(), nil }
	parts := make([]string, 0, len(data.types)); for _, t := range data.types { parts = append(parts, t.String()) }
	return phpv.ZString(strings.Join(parts, "&")).ZVal(), nil
}
func reflectionIntersectionTypeGetTypes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getIntersectionTypeData(o); if data == nil { return phpv.NewZArray().ZVal(), nil }
	arr := phpv.NewZArray()
	for _, t := range data.types { val, err := createReflectionTypeObject(ctx, t); if err != nil { return nil, err }; if val != nil { arr.OffsetSet(ctx, nil, val) } }
	return arr.ZVal(), nil
}
