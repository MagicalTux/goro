package core

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func void spl_autoload ( string $class_name [, string $file_extensions = ".php,.inc" ] )
func fncSplAutoload(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var className phpv.ZString
	var extensions *phpv.ZString
	_, err := Expand(ctx, args, &className, &extensions)
	if err != nil {
		return nil, err
	}

	exts := ctx.Global().GetAutoloadExtensions()
	if extensions != nil {
		exts = string(*extensions)
	}

	// Convert class name to lowercase filename (PHP default autoload behavior)
	filename := strings.ToLower(string(className))

	// Try each extension in the include path
	includePath := string(ctx.Global().GetConfig("include_path", phpv.ZStr(".")).AsString(ctx))
	paths := strings.Split(includePath, string(filepath.ListSeparator))

	for _, ext := range strings.Split(exts, ",") {
		for _, dir := range paths {
			fullPath := filepath.Join(dir, filename+ext)
			// Resolve relative to working directory
			if !filepath.IsAbs(fullPath) {
				fullPath = filepath.Join(string(ctx.Global().Getwd()), fullPath)
			}
			// Check if file exists before including (spl_autoload silently
			// skips missing files, unlike regular include which emits warnings)
			if _, statErr := os.Stat(fullPath); statErr != nil {
				continue
			}
			// Try to include the file
			_, err := ctx.Global().Include(ctx, phpv.ZString(fullPath))
			if err == nil {
				return nil, nil // successfully included
			}
		}
	}

	return nil, nil // silently fail (PHP behavior for spl_autoload)
}

// splAutoloadCallable wraps fncSplAutoload as a Callable for use as the default autoloader.
type splAutoloadCallable struct {
	phpv.CallableVal
}

func (s *splAutoloadCallable) Name() string { return "spl_autoload" }
func (s *splAutoloadCallable) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return fncSplAutoload(ctx, args)
}

// > func void spl_autoload_register ([ callable $autoload_function [, bool $throw = true [, bool $prepend = false ]]] )
func fncSplAutoloadRegister(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	prepend := false
	if len(args) >= 3 {
		prepend = bool(args[2].AsBool(ctx))
	}

	// Emit notice if $throw=false is explicitly passed (PHP 8.x always throws)
	if len(args) >= 2 && !args[1].AsBool(ctx) {
		ctx.Notice("Argument #2 ($do_throw) has been ignored, spl_autoload_register() will always throw")
	}

	if len(args) == 0 || args[0].IsNull() {
		// Register the default spl_autoload function
		ctx.Global().RegisterAutoload(&splAutoloadCallable{}, prepend)
		return phpv.ZTrue.ZVal(), nil
	}

	// Reject spl_autoload_call as an autoloader
	if args[0].GetType() == phpv.ZtString {
		name := string(args[0].AsString(ctx))
		if strings.EqualFold(name, "spl_autoload_call") {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "spl_autoload_register(): Argument #1 ($callback) must not be the spl_autoload_call() function")
		}
	}

	var handler phpv.Callable
	_, err := Expand(ctx, args, &handler)
	if err != nil {
		return nil, err
	}

	ctx.Global().RegisterAutoload(handler, prepend)
	return phpv.ZTrue.ZVal(), nil
}

// > func bool spl_autoload_unregister ( mixed $autoload_function )
func fncSplAutoloadUnregister(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return phpv.ZFalse.ZVal(), nil
	}

	// Handle string argument specially - match by name
	if args[0].GetType() == phpv.ZtString {
		name := string(args[0].AsString(ctx))

		// Special case: spl_autoload_call unregisters all autoloaders (deprecated)
		if strings.EqualFold(name, "spl_autoload_call") {
			ctx.Deprecated("Using spl_autoload_call() as a callback for spl_autoload_unregister() is deprecated, to remove all registered autoloaders, call spl_autoload_unregister() for all values returned from spl_autoload_functions()", logopt.NoFuncName(false))
			ctx.Global().ClearAutoloadFunctions()
			return phpv.ZTrue.ZVal(), nil
		}

		result := ctx.Global().UnregisterAutoloadByName(name)
		return phpv.ZBool(result).ZVal(), nil
	}

	var handler phpv.Callable
	_, err := Expand(ctx, args, &handler)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	result := ctx.Global().UnregisterAutoload(handler)
	return phpv.ZBool(result).ZVal(), nil
}
