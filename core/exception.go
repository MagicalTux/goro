package core

//> class Exception
var Exception = &ZClass{
	Name:       "Exception",
	Implements: []*ZClass{Throwable},
	Methods: map[ZString]*ZClassMethod{
		"__construct": &ZClassMethod{Name: "__construct", Method: NativeMethod(exceptionConstruct)},
	},
}

func SpawnException(ctx Context, l *Loc, msg ZString, code ZInt, prev *ZObject) (*ZObject, error) {
	o, err := NewZObject(ctx, Exception)
	if err != nil {
		return nil, err
	}

	o.ObjectSet(ctx, ZString("message").ZVal(), msg.ZVal())
	o.ObjectSet(ctx, ZString("code").ZVal(), code.ZVal())
	o.ObjectSet(ctx, ZString("file").ZVal(), ZString(l.Filename).ZVal())
	o.ObjectSet(ctx, ZString("line").ZVal(), ZInt(l.Line).ZVal())
	o.ObjectSet(ctx, ZString("char").ZVal(), ZInt(l.Char).ZVal())

	if prev != nil {
		o.ObjectSet(ctx, ZString("previous").ZVal(), prev.ZVal())
	}
	return o, nil
}

func ThrowException(ctx Context, l *Loc, msg ZString, code ZInt) error {
	o, err := SpawnException(ctx, l, msg, code, nil)
	if err != nil {
		return err
	}
	return &PhpThrow{o}
}

// public __construct ([ string $message = "" [, int $code = 0 [, Throwable $previous = NULL ]]] )
func exceptionConstruct(ctx Context, o *ZObject, args []*ZVal) (*ZVal, error) {
	switch len(args) {
	case 3:
		o.ObjectSet(ctx, ZString("previous").ZVal(), args[2])
		fallthrough
	case 2:
		o.ObjectSet(ctx, ZString("code").ZVal(), args[1])
		fallthrough
	case 1:
		o.ObjectSet(ctx, ZString("message").ZVal(), args[0])
	}
	return ZNULL.ZVal(), nil
}
