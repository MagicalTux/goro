package core

import (
	"errors"
	"fmt"
	"reflect"
)

var ErrNotEnoughArguments = errors.New("Too few arguments")

func (z *ZVal) Store(ctx Context, out interface{}) error {
	switch tgt := out.(type) {
	case *bool:
		s, err := z.As(ctx, ZtBool)
		if err != nil {
			return err
		}
		*tgt = bool(s.Value().(ZBool))
		return nil
	case *ZBool:
		s, err := z.As(ctx, ZtBool)
		if err != nil {
			return err
		}
		*tgt = s.Value().(ZBool)
		return nil
	case *ZInt:
		s, err := z.As(ctx, ZtInt)
		if err != nil {
			return err
		}
		*tgt = s.Value().(ZInt)
		return nil
	case *ZFloat:
		s, err := z.As(ctx, ZtFloat)
		if err != nil {
			return err
		}
		*tgt = s.Value().(ZFloat)
		return nil
	case *string:
		s, err := z.As(ctx, ZtString)
		if err != nil {
			return err
		}
		*tgt = string(s.Value().(ZString))
		return nil
	case *ZString:
		s, err := z.As(ctx, ZtString)
		if err != nil {
			return err
		}
		*tgt = s.Value().(ZString)
		return nil
	case *Callable:
		s, err := SpawnCallable(ctx, z)
		if err != nil {
			return err
		}
		*tgt = s
		return nil
	case **ZObject:
		s, err := z.As(ctx, ZtObject)
		if err != nil {
			return err
		}
		obj, ok := s.Value().(*ZObject)
		if !ok {
			return fmt.Errorf("expected parameter to be object, %s given", z.GetType())
		}
		if *tgt != nil {
			if (*tgt).Class != nil {
				// check implements
				if (*tgt).Class != obj.Class {
					// TODO fix parameter #
					return fmt.Errorf("expects parameter %d to be %s, %s given", 1, (*tgt).Class.Name, z.GetType())
				}
			}
		}
		*tgt = obj
		return nil
	case **ZVal:
		// as is
		*tgt = z
		return nil
	default:
		return fmt.Errorf("unsupported target type %T", out)
	}
}

func Expand(ctx Context, args []*ZVal, out ...interface{}) (int, error) {
	for i, v := range out {
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Ptr {
			panic("expand requires arguments to be pointers")
		}
		if rv.Type().Elem().Kind() == reflect.Ptr {
			switch rv.Type().Elem() {
			// these are expected to be pointers
			case reflect.TypeOf(&ZVal{}):
			case reflect.TypeOf(&ZObject{}):
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
			return i, ErrNotEnoughArguments
		}

		err := args[i].Store(ctx, v)
		if err != nil {
			return i, err
		}
	}
	return len(out), nil
}
