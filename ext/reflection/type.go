package reflection

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// ReflectionType and ReflectionNamedType classes

var ReflectionType *phpobj.ZClass
var ReflectionNamedType *phpobj.ZClass

// reflectionTypeData is stored as opaque data on ReflectionType/ReflectionNamedType objects
type reflectionTypeData struct {
	name     phpv.ZString
	nullable bool
	builtin  bool
}

func initReflectionType() {
	ReflectionType = &phpobj.ZClass{
		Name: "ReflectionType",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"allowsnull": {Name: "allowsNull", Method: phpobj.NativeMethod(reflectionTypeAllowsNull)},
			"__tostring": {Name: "__toString", Method: phpobj.NativeMethod(reflectionTypeToString)},
		},
	}

	ReflectionNamedType = &phpobj.ZClass{
		Name:    "ReflectionNamedType",
		Extends: ReflectionType,
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			// inherit from ReflectionType
			"allowsnull": {Name: "allowsNull", Method: phpobj.NativeMethod(reflectionTypeAllowsNull)},
			"__tostring": {Name: "__toString", Method: phpobj.NativeMethod(reflectionTypeToString)},
			// own methods
			"getname":   {Name: "getName", Method: phpobj.NativeMethod(reflectionNamedTypeGetName)},
			"isbuiltin": {Name: "isBuiltin", Method: phpobj.NativeMethod(reflectionNamedTypeIsBuiltin)},
		},
	}
}

func createReflectionTypeObject(ctx phpv.Context, hint *phpv.TypeHint) (*phpv.ZVal, error) {
	if hint == nil {
		return nil, nil
	}

	data := &reflectionTypeData{
		nullable: hint.Nullable,
	}

	// Determine name and builtin status
	typeName := hint.String()
	if hint.Nullable && len(typeName) > 0 && typeName[0] == '?' {
		typeName = typeName[1:]
	}
	data.name = phpv.ZString(typeName)

	// Check if builtin
	switch hint.Type() {
	case phpv.ZtBool, phpv.ZtInt, phpv.ZtFloat, phpv.ZtString, phpv.ZtArray, phpv.ZtNull, phpv.ZtVoid, phpv.ZtNever, phpv.ZtMixed:
		data.builtin = true
	case phpv.ZtObject:
		if hint.ClassName() == "" || hint.ClassName() == "callable" || hint.ClassName() == "iterable" {
			data.builtin = true
		} else {
			data.builtin = false
		}
	default:
		data.builtin = false
	}

	obj, err := phpobj.NewZObjectOpaque(ctx, ReflectionNamedType, data)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func getTypeData(o *phpobj.ZObject) *reflectionTypeData {
	v := o.GetOpaque(ReflectionNamedType)
	if v == nil {
		v = o.GetOpaque(ReflectionType)
	}
	if v == nil {
		return nil
	}
	return v.(*reflectionTypeData)
}

func reflectionTypeAllowsNull(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getTypeData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.nullable).ZVal(), nil
}

func reflectionTypeToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getTypeData(o)
	if data == nil {
		return phpv.ZString("").ZVal(), nil
	}
	name := data.name
	if data.nullable {
		name = "?" + name
	}
	return name.ZVal(), nil
}

func reflectionNamedTypeGetName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getTypeData(o)
	if data == nil {
		return phpv.ZString("").ZVal(), nil
	}
	return data.name.ZVal(), nil
}

func reflectionNamedTypeIsBuiltin(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getTypeData(o)
	if data == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(data.builtin).ZVal(), nil
}
