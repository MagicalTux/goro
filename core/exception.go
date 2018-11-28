package core

import "github.com/MagicalTux/goro/core/phpv"

//> class Exception
var Exception = &ZClass{
	Name:       "Exception",
	Implements: []*ZClass{Throwable},
	Methods: map[phpv.ZString]*ZClassMethod{
		"__construct": &ZClassMethod{Name: "__construct", Method: NativeMethod(exceptionConstruct)},
	},
}

func SpawnException(ctx phpv.Context, l *phpv.Loc, msg phpv.ZString, code phpv.ZInt, prev *ZObject) (*ZObject, error) {
	o, err := NewZObject(ctx, Exception)
	if err != nil {
		return nil, err
	}

	o.ObjectSet(ctx, phpv.ZString("message").ZVal(), msg.ZVal())
	o.ObjectSet(ctx, phpv.ZString("code").ZVal(), code.ZVal())
	o.ObjectSet(ctx, phpv.ZString("file").ZVal(), phpv.ZString(l.Filename).ZVal())
	o.ObjectSet(ctx, phpv.ZString("line").ZVal(), phpv.ZInt(l.Line).ZVal())
	o.ObjectSet(ctx, phpv.ZString("char").ZVal(), phpv.ZInt(l.Char).ZVal())

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
	return &PhpThrow{o}
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
