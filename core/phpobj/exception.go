package phpobj

import (
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
)

// > class Exception
var Exception = &ZClass{
	Name:       "Exception",
	Implements: []*ZClass{Throwable},
	Props: []*phpv.ZClassProp{
		{VarName: phpv.ZString("message"), Default: phpv.ZStr("A").ZVal(), Modifiers: phpv.ZAttrProtected},
		{VarName: phpv.ZString("string"), Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPrivate},
		{VarName: phpv.ZString("code"), Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrProtected},
		{VarName: phpv.ZString("file"), Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrProtected},
		{VarName: phpv.ZString("line"), Default: phpv.ZInt(0).ZVal(), Modifiers: phpv.ZAttrProtected},
		{VarName: phpv.ZString("trace"), Default: phpv.NewZArray().ZVal(), Modifiers: phpv.ZAttrPrivate},
		{VarName: phpv.ZString("previous"), Default: phpv.ZNULL.ZVal(), Modifiers: phpv.ZAttrPrivate},
	},
	Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {Name: "__construct", Method: NativeMethod(exceptionConstruct)},
		// to implement getTrace, I have to make a few modifications
		// on CallZVal and FuncContext first

		// TODO: add methods
		// final public getMessage ( void ) : string
		// final public getPrevious ( void ) : Throwable
		// final public getCode ( void ) : mixed
		// final public getFile ( void ) : string
		// final public getLine ( void ) : int
		// final public getTrace ( void ) : array
		// final public getTraceAsString ( void ) : string
		// public __toString ( void ) : string
		// final private __clone ( void ) : void
	},
}

func SpawnException(ctx phpv.Context, l *phpv.Loc, msg phpv.ZString, code phpv.ZInt, prev *ZObject) (*ZObject, error) {
	o, err := NewZObject(ctx, Exception)
	if err != nil {
		return nil, err
	}

	if prev != nil {
		o.ObjectSet(ctx, phpv.ZString("previous").ZVal(), prev.ZVal())
	}
	return o, nil
}

func ThrowException(ctx phpv.Context, l *phpv.Loc, msg phpv.ZString, code phpv.ZInt) error {
	o, err := SpawnException(ctx, l, msg, code, nil)
	if err != nil {
		return err
	}
	return &phperr.PhpThrow{Obj: o}
}

// public __construct ([ string $message = "" [, int $code = 0 [, Throwable $previous = NULL ]]] )
func exceptionConstruct(ctx phpv.Context, o *ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	switch len(args) {
	case 3:
		o.ObjectSet(ctx, phpv.ZString("previous").ZVal(), args[2])
		fallthrough
	case 2:
		o.ObjectSet(ctx, phpv.ZString("code").ZVal(), args[1])
		fallthrough
	case 1:
		o.ObjectSet(ctx, phpv.ZString("message").ZVal(), args[0])
	}
	return phpv.ZNULL.ZVal(), nil
}
