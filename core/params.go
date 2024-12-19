package core

import (
	"errors"
	"reflect"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

var ErrNotEnoughArguments = errors.New("Too few arguments")

type Referable interface {
	refParamValue() any
	setName(name phpv.ZString)
	init()
}

type Ref[T phpv.Val] struct {
	Value T
	name  phpv.ZString
}

func (rp *Ref[T]) refParamValue() any {
	return &rp.Value
}

func (rp *Ref[T]) setName(name phpv.ZString) {
	rp.name = name
}

func (rp *Ref[T]) init() {
	t := reflect.TypeFor[T]()
	if t.Kind() == reflect.Pointer {
		v := reflect.New(t.Elem())
		rp.Value = v.Interface().(T)
	}
}

func (rp *Ref[T]) Get() T {
	return rp.Value
}
func (rp *Ref[T]) Set(ctx phpv.Context, value T) {
	ctx.Parent(1).OffsetSet(ctx, rp.name, value.ZVal())
	rp.Value = value
}

func zvalStore(ctx phpv.Context, z *phpv.ZVal, out interface{}) (phpv.Val, error) {
	switch tgt := out.(type) {
	case *bool:
		s, err := z.As(ctx, phpv.ZtBool)
		if err != nil {
			return nil, err
		}
		v := s.Value().(phpv.ZBool)
		*tgt = bool(v)
		return v, nil
	case *phpv.ZBool:
		s, err := z.As(ctx, phpv.ZtBool)
		if err != nil {
			return nil, err
		}
		v := s.Value().(phpv.ZBool)
		*tgt = v
		return v, nil
	case *int:
		s, err := z.As(ctx, phpv.ZtInt)
		if err != nil {
			return nil, err
		}
		v := s.Value().(phpv.ZInt)
		*tgt = int(v)
		return v, nil
	case *phpv.ZInt:
		s, err := z.As(ctx, phpv.ZtInt)
		if err != nil {
			return nil, err
		}
		v := s.Value().(phpv.ZInt)
		*tgt = v
		return v, nil
	case *phpv.ZFloat:
		s, err := z.As(ctx, phpv.ZtFloat)
		if err != nil {
			return nil, err
		}
		v := s.Value().(phpv.ZFloat)
		*tgt = v
		return v, nil
	case *string:
		s, err := z.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		v := s.Value().(phpv.ZString)
		*tgt = string(v)
		return v, nil
	case *[]byte:
		s, err := z.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		v := s.Value().(phpv.ZString)
		*tgt = []byte(v)
		return v, nil
	case *phpv.ZString:
		s, err := z.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		v := s.Value().(phpv.ZString)
		*tgt = v
		return v, nil
	case *phpv.Callable:
		s, err := SpawnCallable(ctx, z)
		if err != nil {
			return nil, err
		}
		*tgt = s
		return nil, nil
	case **phpobj.ZObject:
		s, err := z.As(ctx, phpv.ZtObject)
		if err != nil {
			return nil, err
		}
		obj, ok := s.Value().(*phpobj.ZObject)
		if !ok {
			return nil, ctx.Errorf("expected parameter to be object, %s given", z.GetType())
		}
		if *tgt != nil {
			if (*tgt).Class != nil {
				// check implements
				if (*tgt).Class != obj.Class {
					return nil, ctx.Errorf("expects parameter %d to be %s, %s given", 1, (*tgt).Class.GetName(), z.GetType())
				}
			}
		}
		*tgt = obj
		return obj, nil
	case **phpv.ZArray:
		s, err := z.As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, err
		}
		ar, ok := s.Value().(*phpv.ZArray)
		if !ok {
			return nil, ctx.Errorf("expected parameter to be array, %s given", z.GetType())
		}
		*tgt = ar
		return ar, nil
	case **phpv.ZVal:
		// as is
		*tgt = z
		return z, nil
	default:
		return nil, ctx.Errorf("unsupported target type %T", out)
	}
}

func Expand(ctx phpv.Context, args []*phpv.ZVal, out ...interface{}) (int, error) {
	for i, v := range out {
		reference, isRef := v.(Referable)
		if isRef {
			if i < len(args) {
				reference.init()
				name := args[i].GetName()
				reference.setName(name)
			}
			v = reference.refParamValue()
		}

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
			return i, ctx.FuncError(ErrNotEnoughArguments)
		}

		if isRef {
			args[i] = args[i].Ref()
		} else {
			args[i] = args[i].Dup()
		}

		out, err := zvalStore(ctx, args[i], v)
		if err != nil {
			return i, ctx.FuncError(err)
		}

		if isRef {
			// handle case foo($bar) where $bar is undefined
			// and foo takes a reference
			name := args[i].GetName()
			outZVal := out.ZVal()
			outZVal.Name = &name
			ctx.Parent(1).OffsetSet(ctx, outZVal.Name, outZVal)
		}
	}
	return len(out), nil
}
