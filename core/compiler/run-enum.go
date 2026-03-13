package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// runEnumCaseInit initializes an enum case constant. It creates the singleton
// enum object the first time it's evaluated and caches it.
type runEnumCaseInit struct {
	className    phpv.ZString
	caseName     phpv.ZString
	backingValue phpv.Runnable // nil for unit enums
	backingType  phpv.ZType
}

func (r *runEnumCaseInit) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s::%s", r.className, r.caseName)
	return err
}

func (r *runEnumCaseInit) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// Look up the enum class
	class, err := ctx.Global().GetClass(ctx, r.className, false)
	if err != nil {
		return nil, err
	}

	zc, ok := class.(*phpobj.ZClass)
	if !ok {
		return nil, fmt.Errorf("enum %s is not a ZClass", r.className)
	}

	// Create the enum case object using the special enum constructor
	// that avoids init() to prevent infinite recursion (since enum cases
	// are stored as class constants, and init resolves constants).
	obj := phpobj.NewZObjectEnum(ctx, zc)

	// Set the name property
	obj.HashTable().SetString("name", phpv.ZString(r.caseName).ZVal())

	// Set the value property for backed enums
	if r.backingValue != nil {
		val, err := r.backingValue.Run(ctx)
		if err != nil {
			return nil, err
		}
		obj.HashTable().SetString("value", val)
	}

	return obj.ZVal(), nil
}
