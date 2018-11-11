package core

import "errors"

func ParseParameters(ctx Context, args []*ZVal, spec string, out ...interface{}) (int, error) {
	arg_no := 0
	opt := false

	for _, s := range spec {
		if arg_no >= len(args) {
			if !opt {
				// TODO improve message
				return arg_no, errors.New("this function require more arguments")
			}
		}
		v := args[arg_no]
		o := out[arg_no]

		switch s {
		case 'a', 'h':
			// array
			tmp, err := v.As(ctx, ZtArray)
			if err != nil {
				return arg_no, err
			}
			switch tgt := o.(type) {
			case *ZArray:
				tgt = tmp.v.(*ZArray)
			case *ZHashTable:
				tgt = tmp.v.(*ZArray).h
			case *Val:
				*tgt = tmp.Value()
			case *ZVal:
				tgt.v = tmp.v
			case *ZArrayAccess:
				*tgt = tmp.Array()
			case *ZIterator:
				*tgt = tmp.NewIterator()
			default:
				return arg_no, errors.New("invalid target variable")
			}
			arg_no += 1
		case 'b':
			// boolean
			tmp, err := v.As(ctx, ZtBool)
			if err != nil {
				return arg_no, err
			}
			switch tgt := o.(type) {
			case *ZBool:
				*tgt = tmp.v.(ZBool)
			case *bool:
				*tgt = bool(tmp.v.(ZBool))
			case *Val:
				*tgt = tmp.Value()
			case *ZVal:
				tgt.v = tmp.v
			default:
				return arg_no, errors.New("invalid target variable")
			}
			arg_no += 1
		case 'f':
			// callable
			var f Callable
			switch t := v.v.(type) {
			case Callable:
				f = t
			default:
				return arg_no, errors.New("argument needs to be callable (TODO handle strings)") // TODO
			}

			switch tgt := o.(type) {
			case *Callable:
				*tgt = f
			default:
				return arg_no, errors.New("invalid target variable")
			}
			arg_no += 1
		case 'z':
			switch tgt := o.(type) {
			case **ZVal:
				*tgt = v
			case *ZVal:
				tgt.v = v.v
			case *Val:
				*tgt = v.v
			}
			arg_no += 1
		case 's':
			// string
			tmp, err := v.As(ctx, ZtString)
			if err != nil {
				return arg_no, err
			}
			switch tgt := o.(type) {
			case *string:
				*tgt = tmp.String()
			case *ZString:
				*tgt = tmp.v.(ZString)
			case *Val:
				*tgt = tmp.Value()
			case *ZVal:
				tgt.v = tmp.v
			default:
				return arg_no, errors.New("failed to store string value")
			}
			arg_no += 1
		}
	}
	return arg_no, nil
}
