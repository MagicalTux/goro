package core

import (
	"fmt"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpv"
)

// reservedClassAliasNames contains names that cannot be used as class aliases.
var reservedClassAliasNames = map[string]bool{
	"self": true, "parent": true, "static": true,
	"int": true, "float": true, "bool": true, "string": true,
	"array": true, "object": true, "null": true, "void": true,
	"never": true, "true": true, "false": true, "mixed": true,
	"callable": true, "iterable": true,
}

// > func bool class_alias ( string $class , string $alias [, bool $autoload = true ] )
func fncClassAlias(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var className phpv.ZString
	var alias phpv.ZString
	var autoloadArg Optional[phpv.ZBool]
	_, err := Expand(ctx, args, &className, &alias, &autoloadArg)
	if err != nil {
		return nil, err
	}

	// Check reserved names
	aliasLower := strings.ToLower(string(alias))
	if reservedClassAliasNames[aliasLower] {
		return nil, &phpv.PhpError{
			Err:  fmt.Errorf("Cannot use \"%s\" as a class alias as it is reserved", alias),
			Code: phpv.E_ERROR,
			Loc:  ctx.Loc(),
		}
	}

	autoload := bool(autoloadArg.GetOrDefault(phpv.ZBool(true)))

	// Resolve the original class
	class, err := ctx.Global().GetClass(ctx, className, autoload)
	if err != nil {
		ctx.Warn("Class \"%s\" not found", className)
		return phpv.ZFalse.ZVal(), nil
	}

	// Register the class under the alias name
	err = ctx.Global().RegisterClass(alias, class)
	if err != nil {
		// For class_alias, construct the message using the alias name (not the existing name)
		if redeclErr, ok := err.(interface {
			RedeclareKind() string
			RedeclarePrevLoc() string
		}); ok {
			ctx.Warn("Cannot redeclare %s %s%s", redeclErr.RedeclareKind(), alias, redeclErr.RedeclarePrevLoc(), logopt.NoFuncName(true))
		} else {
			ctx.Warn("%s", err, logopt.NoFuncName(true))
		}
		return phpv.ZTrue.ZVal(), nil // PHP returns true even on redeclaration warning
	}

	return phpv.ZTrue.ZVal(), nil
}
