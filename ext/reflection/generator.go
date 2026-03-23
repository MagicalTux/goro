package reflection

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// ReflectionGenerator class - stub implementation
var ReflectionGenerator *phpobj.ZClass

// ReflectionFiber class - stub implementation
var ReflectionFiber *phpobj.ZClass

// ReflectionReference class - stub implementation
var ReflectionReference *phpobj.ZClass

func initReflectionGenerator() {
	ReflectionGenerator = &phpobj.ZClass{
		Name: "ReflectionGenerator",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) < 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionGenerator::__construct() expects exactly 1 argument, 0 given")
				}
				// Check if the argument is a Generator object
				if args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionGenerator::__construct(): Argument #1 ($generator) must be of type Generator, %s given", args[0].GetType()))
				}
				obj := args[0].AsObject(ctx)
				if obj.GetClass().GetName() != "Generator" {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("ReflectionGenerator::__construct(): Argument #1 ($generator) must be of type Generator, %s given", obj.GetClass().GetName()))
				}
				o.SetOpaque(ReflectionGenerator, obj)
				return nil, nil
			})},
			"getexecutingline": {Name: "getExecutingLine", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZInt(0).ZVal(), nil
			})},
			"getexecutingfile": {Name: "getExecutingFile", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZString("").ZVal(), nil
			})},
			"getexecutinggenerator": {Name: "getExecutingGenerator", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				gen := o.GetOpaque(ReflectionGenerator)
				if gen == nil {
					return nil, phpobj.ThrowError(ctx, ReflectionException, "Cannot fetch information from a closed Generator")
				}
				obj := gen.(*phpobj.ZObject)
				return obj.ZVal(), nil
			})},
			"getfunction": {Name: "getFunction", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZNULL.ZVal(), nil
			})},
			"getthis": {Name: "getThis", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZNULL.ZVal(), nil
			})},
			"gettrace": {Name: "getTrace", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.NewZArray().ZVal(), nil
			})},
			"isclosed": {Name: "isClosed", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZBool(false).ZVal(), nil
			})},
		},
	}

	ReflectionFiber = &phpobj.ZClass{
		Name: "ReflectionFiber",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) < 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionFiber::__construct() expects exactly 1 argument, 0 given")
				}
				o.SetOpaque(ReflectionFiber, args[0])
				return nil, nil
			})},
			"getfiber": {Name: "getFiber", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				v := o.GetOpaque(ReflectionFiber)
				if v == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return v.(*phpv.ZVal), nil
			})},
			"getexecutingline": {Name: "getExecutingLine", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZInt(0).ZVal(), nil
			})},
			"getexecutingfile": {Name: "getExecutingFile", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZString("").ZVal(), nil
			})},
			"getcallable": {Name: "getCallable", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZNULL.ZVal(), nil
			})},
			"gettrace": {Name: "getTrace", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.NewZArray().ZVal(), nil
			})},
		},
	}

	ReflectionReference = &phpobj.ZClass{
		Name: "ReflectionReference",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Modifiers: phpv.ZAttrPrivate, Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot directly construct ReflectionReference")
			})},
			"fromarrayelement": {Name: "fromArrayElement", Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic, Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) < 2 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionReference::fromArrayElement() expects exactly 2 arguments")
				}
				// Check if the array element is a reference
				if args[0].GetType() != phpv.ZtArray {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ReflectionReference::fromArrayElement(): Argument #1 ($array) must be of type array")
				}
				// If the element is not a reference, return null
				return phpv.ZNULL.ZVal(), nil
			})},
			"getid": {Name: "getId", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZString("").ZVal(), nil
			})},
		},
	}
}
