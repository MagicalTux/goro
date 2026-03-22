package reflection

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

func initReflectionEnum() {
	ReflectionEnumUnitCase.Extends = ReflectionClassConstant
	ReflectionEnumUnitCase.Props = ReflectionClassConstant.Props
	ReflectionEnumUnitCase.Methods = phpobj.CopyMethods(ReflectionClassConstant.Methods)
	ReflectionEnumUnitCase.Methods["__construct"] = &phpv.ZClassMethod{Name: "__construct", Method: phpobj.NativeMethod(reflectionEnumUnitCaseConstruct)}
	ReflectionEnumUnitCase.Methods["getenum"] = &phpv.ZClassMethod{Name: "getEnum", Method: phpobj.NativeMethod(reflectionEnumCaseGetEnum)}
	ReflectionEnumUnitCase.Methods["getvalue"] = &phpv.ZClassMethod{Name: "getValue", Method: phpobj.NativeMethod(reflectionEnumUnitCaseGetValue)}
	ReflectionEnumBackedCase.Extends = ReflectionEnumUnitCase
	ReflectionEnumBackedCase.Props = ReflectionEnumUnitCase.Props
	ReflectionEnumBackedCase.Methods = phpobj.CopyMethods(ReflectionEnumUnitCase.Methods)
	ReflectionEnumBackedCase.Methods["__construct"] = &phpv.ZClassMethod{Name: "__construct", Method: phpobj.NativeMethod(reflectionEnumBackedCaseConstruct)}
	ReflectionEnumBackedCase.Methods["getbackingvalue"] = &phpv.ZClassMethod{Name: "getBackingValue", Method: phpobj.NativeMethod(reflectionEnumBackedCaseGetBackingValue)}
	ReflectionEnum.Methods["__construct"] = &phpv.ZClassMethod{Name: "__construct", Method: phpobj.NativeMethod(reflectionEnumConstruct)}
	ReflectionEnum.Methods["getbackingtype"] = &phpv.ZClassMethod{Name: "getBackingType", Method: phpobj.NativeMethod(reflectionEnumGetBackingType)}
	ReflectionEnum.Methods["getcases"] = &phpv.ZClassMethod{Name: "getCases", Method: phpobj.NativeMethod(reflectionEnumGetCases)}
	ReflectionEnum.Methods["getcase"] = &phpv.ZClassMethod{Name: "getCase", Method: phpobj.NativeMethod(reflectionEnumGetCase)}
	ReflectionEnum.Methods["hascase"] = &phpv.ZClassMethod{Name: "hasCase", Method: phpobj.NativeMethod(reflectionEnumHasCase)}
	ReflectionEnum.Methods["isbacked"] = &phpv.ZClassMethod{Name: "isBacked", Method: phpobj.NativeMethod(reflectionEnumIsBacked)}
}

func reflectionEnumConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 { return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionEnum::__construct() expects exactly 1 argument, 0 given") }
	arg := args[0]; var class phpv.ZClass
	if arg.GetType() == phpv.ZtObject { class = arg.AsObject(ctx).GetClass()
	} else if arg.GetType() == phpv.ZtArray { return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionEnum::__construct(): Argument #1 ($objectOrClass) must be of type object|string, array given")
	} else { className := arg.AsString(ctx); var err error; class, err = resolveClass(ctx, className); if err != nil { return nil, err } }
	if !class.GetType().Has(phpv.ZClassTypeEnum) { return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Class \"%s\" is not an enum", class.GetName())) }
	o.HashTable().SetString("name", class.GetName().ZVal()); o.SetOpaque(ReflectionClass, class); return nil, nil
}
func reflectionEnumGetBackingType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o); if zc == nil { return phpv.ZNULL.ZVal(), nil }
	if zc.EnumBackingType == 0 { return phpv.ZNULL.ZVal(), nil }
	var hint *phpv.TypeHint
	switch zc.EnumBackingType { case phpv.ZtString: hint = phpv.ParseTypeHint("string"); case phpv.ZtInt: hint = phpv.ParseTypeHint("int"); default: return phpv.ZNULL.ZVal(), nil }
	return createReflectionNamedTypeObject(ctx, hint)
}
func reflectionEnumGetCases(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o); if zc == nil { return phpv.NewZArray().ZVal(), nil }
	arr := phpv.NewZArray()
	for _, caseName := range zc.EnumCases { val, err := createEnumCaseObject(ctx, zc, caseName); if err != nil { return nil, err }; arr.OffsetSet(ctx, nil, val) }
	return arr.ZVal(), nil
}
func reflectionEnumGetCase(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 { return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionEnum::getCase() expects exactly 1 argument, 0 given") }
	zc := getZClass(o); if zc == nil { return nil, phpobj.ThrowError(ctx, ReflectionException, "Internal error: Failed to retrieve the reflection object") }
	caseName := args[0].AsString(ctx); isCase := false
	for _, cn := range zc.EnumCases { if cn == caseName { isCase = true; break } }
	if !isCase { _, constExists := lookupClassConst(zc, caseName); if constExists { return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("%s::%s is not a case", zc.GetName(), caseName)) }; return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Case %s::%s does not exist", zc.GetName(), caseName)) }
	return createEnumCaseObject(ctx, zc, caseName)
}
func reflectionEnumHasCase(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 { return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionEnum::hasCase() expects exactly 1 argument, 0 given") }
	zc := getZClass(o); if zc == nil { return phpv.ZBool(false).ZVal(), nil }
	caseName := args[0].AsString(ctx); for _, cn := range zc.EnumCases { if cn == caseName { return phpv.ZBool(true).ZVal(), nil } }; return phpv.ZBool(false).ZVal(), nil
}
func reflectionEnumIsBacked(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	zc := getZClass(o); if zc == nil { return phpv.ZBool(false).ZVal(), nil }; return phpv.ZBool(zc.EnumBackingType != 0).ZVal(), nil
}
func createEnumCaseObject(ctx phpv.Context, zc *phpobj.ZClass, caseName phpv.ZString) (*phpv.ZVal, error) {
	constVal, found := lookupClassConst(zc, caseName); if !found { return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Case %s::%s does not exist", zc.GetName(), caseName)) }
	var targetClass *phpobj.ZClass; if zc.EnumBackingType != 0 { targetClass = ReflectionEnumBackedCase } else { targetClass = ReflectionEnumUnitCase }
	obj, err := phpobj.CreateZObject(ctx, targetClass); if err != nil { return nil, err }
	data := &reflectionClassConstantData{constName: caseName, constVal: constVal, class: zc}
	obj.HashTable().SetString("name", caseName.ZVal()); obj.HashTable().SetString("class", zc.GetName().ZVal()); obj.SetOpaque(ReflectionClassConstant, data); return obj.ZVal(), nil
}
func reflectionEnumUnitCaseConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 { return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionEnumUnitCase::__construct() expects exactly 2 arguments") }
	if args[0].GetType() == phpv.ZtArray { return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionEnumUnitCase::__construct(): Argument #1 ($class) must be of type object|string, array given") }
	var class phpv.ZClass; var err error
	if args[0].GetType() == phpv.ZtObject { class = args[0].AsObject(ctx).GetClass() } else { className := args[0].AsString(ctx); class, err = resolveClass(ctx, className); if err != nil { return nil, err } }
	caseName := args[1].AsString(ctx); zc, ok := class.(*phpobj.ZClass); if !ok { return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Class \"%s\" does not have a constant \"%s\"", class.GetName(), caseName)) }
	constVal, found := lookupClassConst(zc, caseName); if !found { return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Constant %s::%s does not exist", class.GetName(), caseName)) }
	data := &reflectionClassConstantData{constName: caseName, constVal: constVal, class: zc}
	o.HashTable().SetString("name", caseName.ZVal()); o.HashTable().SetString("class", class.GetName().ZVal()); o.SetOpaque(ReflectionClassConstant, data); return nil, nil
}
func reflectionEnumCaseGetEnum(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o); if data == nil { return phpv.ZNULL.ZVal(), nil }
	obj, err := phpobj.CreateZObject(ctx, ReflectionEnum); if err != nil { return nil, err }
	obj.HashTable().SetString("name", data.class.GetName().ZVal()); obj.SetOpaque(ReflectionClass, data.class); return obj.ZVal(), nil
}
func reflectionEnumUnitCaseGetValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o); if data == nil { return phpv.ZNULL.ZVal(), nil }
	val := data.constVal.Value; if cd, ok := val.(*phpv.CompileDelayed); ok { resolved, err := cd.Run(ctx); if err != nil { return nil, err }; return resolved, nil }; return val.ZVal(), nil
}
func reflectionEnumBackedCaseConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 { return nil, phpobj.ThrowError(ctx, phpobj.Error, "ReflectionEnumBackedCase::__construct() expects exactly 2 arguments") }
	if args[0].GetType() == phpv.ZtArray { return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionEnumBackedCase::__construct(): Argument #1 ($class) must be of type object|string, array given") }
	var class phpv.ZClass; var err error
	if args[0].GetType() == phpv.ZtObject { class = args[0].AsObject(ctx).GetClass() } else { className := args[0].AsString(ctx); class, err = resolveClass(ctx, className); if err != nil { return nil, err } }
	caseName := args[1].AsString(ctx); zc, ok := class.(*phpobj.ZClass); if !ok { return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Class \"%s\" does not have a constant \"%s\"", class.GetName(), caseName)) }
	if zc.EnumBackingType == 0 { return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Enum case %s::%s is not a backed case", class.GetName(), caseName)) }
	constVal, found := lookupClassConst(zc, caseName); if !found { return nil, phpobj.ThrowError(ctx, ReflectionException, fmt.Sprintf("Constant %s::%s does not exist", class.GetName(), caseName)) }
	data := &reflectionClassConstantData{constName: caseName, constVal: constVal, class: zc}
	o.HashTable().SetString("name", caseName.ZVal()); o.HashTable().SetString("class", class.GetName().ZVal()); o.SetOpaque(ReflectionClassConstant, data); return nil, nil
}
func reflectionEnumBackedCaseGetBackingValue(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	data := getClassConstData(o); if data == nil { return phpv.ZNULL.ZVal(), nil }
	val := data.constVal.Value
	if cd, ok := val.(*phpv.CompileDelayed); ok { resolved, err := cd.Run(ctx); if err != nil { return nil, err }; if resolved.GetType() == phpv.ZtObject { obj := resolved.AsObject(ctx); backingVal := obj.HashTable().GetString("value"); if backingVal != nil { return backingVal, nil } }; return phpv.ZNULL.ZVal(), nil }
	if obj, ok := val.(*phpobj.ZObject); ok { backingVal := obj.HashTable().GetString("value"); if backingVal != nil { return backingVal, nil } }
	return phpv.ZNULL.ZVal(), nil
}
