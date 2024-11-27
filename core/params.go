package core

import (
	"errors"
	"reflect"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

var ErrNotEnoughArguments = errors.New("Too few arguments")

func zvalStore(ctx phpv.Context, z *phpv.ZVal, out interface{}) error {
	switch tgt := out.(type) {
	case *bool:
		s, err := z.As(ctx, phpv.ZtBool)
		if err != nil {
			return err
		}
		*tgt = bool(s.Value().(phpv.ZBool))
		return nil
	case *phpv.ZBool:
		s, err := z.As(ctx, phpv.ZtBool)
		if err != nil {
			return err
		}
		*tgt = s.Value().(phpv.ZBool)
		return nil
	case *int:
		s, err := z.As(ctx, phpv.ZtInt)
		if err != nil {
			return err
		}
		*tgt = int(s.Value().(phpv.ZInt))
		return nil
	case *phpv.ZInt:
		s, err := z.As(ctx, phpv.ZtInt)
		if err != nil {
			return err
		}
		*tgt = s.Value().(phpv.ZInt)
		return nil
	case *phpv.ZFloat:
		s, err := z.As(ctx, phpv.ZtFloat)
		if err != nil {
			return err
		}
		*tgt = s.Value().(phpv.ZFloat)
		return nil
	case *string:
		s, err := z.As(ctx, phpv.ZtString)
		if err != nil {
			return err
		}
		*tgt = string(s.Value().(phpv.ZString))
		return nil
	case *[]byte:
		s, err := z.As(ctx, phpv.ZtString)
		if err != nil {
			return err
		}
		*tgt = []byte(s.Value().(phpv.ZString))
		return nil
	case *phpv.ZString:
		s, err := z.As(ctx, phpv.ZtString)
		if err != nil {
			return err
		}
		*tgt = s.Value().(phpv.ZString)
		return nil
	case *phpv.Callable:
		s, err := SpawnCallable(ctx, z)
		if err != nil {
			return err
		}
		*tgt = s
		return nil
	case **phpobj.ZObject:
		s, err := z.As(ctx, phpv.ZtObject)
		if err != nil {
			return err
		}
		obj, ok := s.Value().(*phpobj.ZObject)
		if !ok {
			return ctx.Errorf("expected parameter to be object, %s given", z.GetType())
		}
		if *tgt != nil {
			if (*tgt).Class != nil {
				// check implements
				if (*tgt).Class != obj.Class {
					return ctx.Errorf("expects parameter %d to be %s, %s given", 1, (*tgt).Class.GetName(), z.GetType())
				}
			}
		}
		*tgt = obj
		return nil
	case **phpv.ZArray:
		s, err := z.As(ctx, phpv.ZtArray)
		if err != nil {
			return err
		}
		ar, ok := s.Value().(*phpv.ZArray)
		if !ok {
			return ctx.Errorf("expected parameter to be array, %s given", z.GetType())
		}
		*tgt = ar
		return nil
	case **phpv.ZVal:
		// as is
		*tgt = z
		return nil
	default:
		return ctx.Errorf("unsupported target type %T", out)
	}
}

func Expand(ctx phpv.Context, args []*phpv.ZVal, out ...interface{}) (int, error) {
	for i, v := range out {
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Ptr {
			panic("expand requires arguments to be pointers")
		}
		if rv.Type().Elem().Kind() == reflect.Ptr {
			switch rv.Type().Elem() {
			// these are expected to be pointers
			case reflect.TypeOf(&phpv.ZVal{}):
			case reflect.TypeOf(&phpobj.ZObject{}):
			case reflect.TypeOf(&phpv.ZArray{}):
			default:
				// pointer of pointer → optional argument
				if len(args) <= i {
					// end of argments
					continue
				}
				// we have an argument → instanciate and update v to point to the subvalue
				newv := reflect.New(rv.Type().Elem().Elem())
				rv.Elem().Set(newv)
				v = newv.Interface()
			}
		}
		if len(args) <= i {
			// not enough arguments, such errors in PHP can be returned as either:
			// Uncaught ArgumentCountError: Too few arguments to function toto(), 0 passed
			// x() expects at least 2 parameters, 0 given
			return i, ctx.Error(ErrNotEnoughArguments)
		}

		err := zvalStore(ctx, args[i], v)
		if err != nil {
			return i, err
		}
	}
	return len(out), nil
}
