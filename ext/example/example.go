package example

import (
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// The comments // > func is required so that it will be
// registered as a PHP function, and should be placed
// above the go function definition.
// The important part is the function name.
// The arguments are ignored by the generator tool,
// but it's still useful to add as a reference when writing the function.
// The extension functions must have the signature
//   func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error)
// otherwise it won't build.

// This hello function takes a string argument and returns a string.
// The go variable names doesn't need to match, and can be named with anything else.
// > func string hello ( string $name )
func fncHello(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name phpv.ZString
	_, err := core.Expand(ctx, args, &name)
	if err != nil {
		return nil, err
	}

	result := phpv.ZStr("hello " + string(name))
	return result.ZVal(), nil
}

// This hey function takes a reference string param, and returns a bool.
// Unlike the hello function, hey will modify and put the output
// in the name argument.
// Functions can also have aliases. hey() have yo() and ey() aliases,
// any of these names can be used interchangeably.
// > func bool hey ( string &$name )
// > alias yo
// > alias ey
func fncHey(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Note: phpv.ZString is inside the core.Ref
	var name core.Ref[phpv.ZString]
	_, err := core.Expand(ctx, args, &name)
	if err != nil {
		return nil, err
	}

	result := phpv.ZString("hey " + string(name.Get()))
	// write the result into the name argument
	// this is similar to *name = result
	name.Set(ctx, result)

	return phpv.ZTrue.ZVal(), nil
}

// Wah can take three arguments. $name is required,
// $count is optional with default value.
// $out is an optional reference.
// See below how these are expressed in go.
// > func void wah (string $name [ , int count = 10, string &$out  ] )
func fncWah(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var name phpv.ZString
	var count core.Optional[phpv.ZInt]
	var out core.OptionalRef[phpv.ZString]
	_, err := core.Expand(ctx, args, &name, &count, &out)
	if err != nil {
		return nil, err
	}

	outResult := strings.Repeat("wah ", int(count.GetOrDefault(10))) + string(name)
	out.Set(ctx, phpv.ZString(outResult))

	result := phpv.ZString("wah " + string(name))
	return result.ZVal(), nil
}
