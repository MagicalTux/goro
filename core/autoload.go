package core

import (
	"path/filepath"
	"strings"

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

	exts := ".php,.inc"
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
			// Try to include the file
			_, err := ctx.Global().Include(ctx, phpv.ZString(fullPath))
			if err == nil {
				return nil, nil // successfully included
			}
		}
	}

	return nil, nil // silently fail (PHP behavior for spl_autoload)
}

// > func void spl_autoload_register ([ callable $autoload_function [, bool $throw = true [, bool $prepend = false ]]] )
func fncSplAutoloadRegister(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handler phpv.Callable
	_, err := Expand(ctx, args, &handler)
	if err != nil {
		return nil, err
	}

	ctx.Global().RegisterAutoload(handler)
	return nil, nil
}

// > func bool spl_autoload_unregister ( mixed $autoload_function )
func fncSplAutoloadUnregister(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var handler phpv.Callable
	_, err := Expand(ctx, args, &handler)
	if err != nil {
		return nil, err
	}

	result := ctx.Global().UnregisterAutoload(handler)
	return phpv.ZBool(result).ZVal(), nil
}
