package core

import (
	"errors"
	"reflect"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

var ErrNotEnoughArguments = errors.New("Too few arguments")

type optionable interface {
	getOptionalValue() any
}

type Optional[T phpv.Val] struct {
	value *T
}

func (o *Optional[T]) getOptionalValue() any {
	if o.value == nil {
		var empty T
		o.value = &empty
	}
	return o.value
}

func (o *Optional[T]) Get() T {
	var empty T
	if o.value == nil {
		return empty
	}
	return *o.value
}

func (o *Optional[T]) GetOrDefault(defaultVal T) T {
	if o.value == nil {
		return defaultVal
	}
	return *o.value
}

func (o *Optional[T]) GetType() phpv.ZType {
	return o.Get().GetType()
}
func (o *Optional[T]) ZVal() *phpv.ZVal {
	return o.Get().ZVal()
}
func (o *Optional[T]) Value() phpv.Val {
	return o.Get()
}
func (o *Optional[T]) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	return o.Get().AsVal(ctx, t)
}

func (o *Optional[T]) String() string {
	return o.Get().String()
}

func (or *Optional[T]) HasArg() bool {
	return or.value != nil
}

type referable interface {
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

type optionalReferable interface {
	referable
	setHasValue(bool)
}
type OptionalRef[T phpv.Val] struct {
	Ref[T]
	isSet bool
}

func (or *OptionalRef[T]) setHasValue(ok bool) {
	or.isSet = ok
}

func (or *OptionalRef[T]) GetOrDefault(defaultVal T) T {
	if !or.isSet {
		return defaultVal
	}
	return or.Value
}

func (or *OptionalRef[T]) HasArg() bool {
	return or.isSet
}

func zvalStore(ctx phpv.Context, i int, args []*phpv.ZVal, out interface{}) (phpv.Val, error) {
	z := args[i]
	paramNo := i + 1
	name := ctx.GetFuncName()
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
		return s, nil
	case *phpv.ZObject:
		s, err := z.As(ctx, phpv.ZtObject)
		if err != nil {
			return nil, err
		}
		obj, ok := s.Value().(*phpobj.ZObject)
		if !ok {
			return nil, ctx.Errorf("%s() expected parameter %d to be object, %s given", name, paramNo, z.GetType())
		}
		if *tgt != nil {
			if (*tgt).GetClass() != nil {
				// check implements
				if (*tgt).GetClass() != obj.Class {
					return nil, ctx.Errorf("%s() expects parameter %d to be %s, %s given", name, paramNo, (*tgt).GetClass().GetName(), z.GetType())
				}
			}
		}
		*tgt = obj
		return obj, nil
	case **phpobj.ZObject:
		s, err := z.As(ctx, phpv.ZtObject)
		if err != nil {
			return nil, err
		}
		obj, ok := s.Value().(*phpobj.ZObject)
		if !ok {
			return nil, ctx.Errorf("%s() expected parameter %d to be object, %s given", name, paramNo, z.GetType())
		}
		if *tgt != nil {
			if (*tgt).Class != nil {
				// check implements
				if (*tgt).Class != obj.Class {
					return nil, ctx.Errorf("%s() expects parameter %d to be %s, %s given", name, paramNo, (*tgt).Class.GetName(), z.GetType())
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
			return nil, ctx.Errorf("%s() expected parameter %d to be array, %s given", name, paramNo, z.GetType())
		}
		*tgt = ar
		return ar, nil
	case *phpv.Resource:
		s, err := z.As(ctx, phpv.ZtResource)
		if err != nil {
			return nil, err
		}
		res, ok := s.Value().(phpv.Resource)
		if !ok {
			return nil, ctx.Errorf("%s() expected parameter %d to be resource, %s given", name, paramNo, z.GetType())
		}
		*tgt = res
		return res, nil
	case **phpv.ZVal:
		// as is
		*tgt = z
		return z, nil
	default:
		return nil, ctx.Errorf("unsupported target type %T", out)
	}
}

func ExpandAt(ctx phpv.Context, args []*phpv.ZVal, i int, out interface{}) error {
	isRef := false
	switch r := out.(type) {
	case optionalReferable:
		if i >= len(args) {
			// no more args and this is optional, so no error
			return nil
		}
		isRef = true
		r.setHasValue(true)
		name := args[i].GetName()
		r.setName(name)
		out = r.refParamValue()
	case referable:
		isRef = true
		if i < len(args) {
			r.init()
			name := args[i].GetName()
			r.setName(name)
		}
		out = r.refParamValue()
	case optionable:
		if i >= len(args) {
			// no more args and this is optional, so no error
			return nil
		}
		out = r.getOptionalValue()
	default:
		rv := reflect.ValueOf(out)

		if rv.Kind() != reflect.Ptr {
			panic("expand requires arguments to be pointers")
		}

		if rv.Type().Elem().Kind() == reflect.Ptr {
			// TODO: remove instances of *blah with Optional[blah] then remove this part

			switch rv.Type().Elem() {
			// these are expected to be pointers
			case reflect.TypeOf(&phpv.ZVal{}):
			case reflect.TypeOf(&phpobj.ZObject{}):
			case reflect.TypeOf(&phpv.ZArray{}):

			default:
				// pointer of pointer → optional argument
				if len(args) <= i {
					// end of argments
					return nil
				}

				// we have an argument → instanciate and update v to point to the subvalue
				newv := reflect.New(rv.Type().Elem().Elem())
				rv.Elem().Set(newv)
				out = newv.Interface()
			}
		}
	}

	if len(args) <= i {
		// not enough arguments, such errors in PHP can be returned as either:
		// Uncaught ArgumentCountError: Too few arguments to function toto(), 0 passed
		// x() expects at least 2 parameters, 0 given
		return ctx.FuncError(ErrNotEnoughArguments)
	}

	if isRef {
		args[i] = args[i].Ref()
	} else {
		parentCtx := ctx.Parent(1)
		if parentCtx == nil {
			parentCtx = ctx
		}
		if args[i].GetName() != "" {
			if ok, _ := parentCtx.OffsetExists(ctx, args[i].GetName()); !ok {
				if err := ctx.Notice("Undefined variable: %s",
					args[i].GetName(), logopt.NoFuncName(true)); err != nil {
					return err
				}
			}
		}

		args[i] = args[i].Dup()
	}

	dest, err := zvalStore(ctx, i, args, out)
	if err != nil {
		return ctx.Error(err)
	}

	if isRef {
		// handle case foo($bar) where $bar is undefined
		// and foo takes a reference
		name := args[i].GetName()
		outZVal := dest.ZVal()
		outZVal.Name = &name
		if name != "GLOBALS" {
			// check if varname is not GLOBALS to avoid infinite loop
			ctx.Parent(1).OffsetSet(ctx, outZVal.Name, outZVal)
		}
	}
	return nil
}

func Expand(ctx phpv.Context, args []*phpv.ZVal, out ...interface{}) (int, error) {
	for i := range out {
		ExpandAt(ctx, args, i, out[i])
	}
	return len(out), nil
}
