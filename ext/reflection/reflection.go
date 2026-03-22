package reflection

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// Reflection is the static utility class (Reflection::getModifierNames, etc.)
var ReflectionStatic *phpobj.ZClass

func initReflectionStatic() {
	ReflectionStatic = &phpobj.ZClass{
		Name: "Reflection",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"getmodifiernames": {Name: "getModifierNames", Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic, Method: phpobj.NativeMethod(reflectionGetModifierNames)},
		},
	}
}

func reflectionGetModifierNames(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return phpv.NewZArray().ZVal(), nil
	}
	modifiers := int64(args[0].AsInt(ctx))
	arr := phpv.NewZArray()

	if modifiers&ReflectionMethodIS_ABSTRACT != 0 {
		arr.OffsetSet(ctx, nil, phpv.ZString("abstract").ZVal())
	}
	if modifiers&ReflectionMethodIS_FINAL != 0 {
		arr.OffsetSet(ctx, nil, phpv.ZString("final").ZVal())
	}
	if modifiers&ReflectionMethodIS_PUBLIC != 0 {
		arr.OffsetSet(ctx, nil, phpv.ZString("public").ZVal())
	}
	if modifiers&ReflectionMethodIS_PROTECTED != 0 {
		arr.OffsetSet(ctx, nil, phpv.ZString("protected").ZVal())
	}
	if modifiers&ReflectionMethodIS_PRIVATE != 0 {
		arr.OffsetSet(ctx, nil, phpv.ZString("private").ZVal())
	}
	if modifiers&ReflectionMethodIS_STATIC != 0 {
		arr.OffsetSet(ctx, nil, phpv.ZString("static").ZVal())
	}
	if modifiers&128 != 0 { // IS_READONLY
		arr.OffsetSet(ctx, nil, phpv.ZString("readonly").ZVal())
	}

	return arr.ZVal(), nil
}
