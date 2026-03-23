package compiler

import (
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type runConstant struct {
	c          string
	l          *phpv.Loc
	noFallback bool // when true, do not fall back to global namespace
}

func (r *runConstant) Dump(w io.Writer) error {
	_, err := w.Write([]byte(r.c))
	return err
}

// shortName returns the part after the last backslash, or the full name if no backslash.
func shortName(name string) string {
	if idx := strings.LastIndexByte(name, '\\'); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func (r *runConstant) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	short := shortName(r.c)
	isQualified := short != r.c

	// For unqualified names, check special constants immediately
	if !isQualified {
		switch strings.ToLower(short) {
		case "null":
			return phpv.ZNull{}.ZVal(), nil
		case "true":
			return phpv.ZBool(true).ZVal(), nil
		case "false":
			return phpv.ZBool(false).ZVal(), nil
		case "self":
			return phpv.ZString("self").ZVal(), nil
		case "parent":
			return phpv.ZString("parent").ZVal(), nil
		}
	}

	// Try the full (possibly namespaced) name first
	// Normalize namespace part to lowercase (PHP namespace resolution is case-insensitive)
	normalizedName := r.c
	if idx := strings.LastIndex(normalizedName, "\\"); idx >= 0 {
		normalizedName = strings.ToLower(normalizedName[:idx]) + normalizedName[idx:]
	}
	constName := phpv.ZString(normalizedName)
	z, ok := ctx.Global().ConstantGet(constName)
	if ok {
		// Check #[\Deprecated] on the constant
		if err := checkConstantDeprecated(ctx, constName, r.l); err != nil {
			return nil, err
		}
		return z.ZVal(), nil
	}

	// Namespace fallback: if Foo\BAR is not found, try BAR (global)
	// Skip fallback for explicitly qualified names (namespace\FOO or \Foo\BAR)
	if isQualified && !r.noFallback {
		shortName := phpv.ZString(short)
		z, ok = ctx.Global().ConstantGet(shortName)
		if ok {
			if err := checkConstantDeprecated(ctx, shortName, r.l); err != nil {
				return nil, err
			}
			return z.ZVal(), nil
		}
		// For qualified names, fall back to built-in constants after lookup
		switch strings.ToLower(short) {
		case "null":
			return phpv.ZNull{}.ZVal(), nil
		case "true":
			return phpv.ZBool(true).ZVal(), nil
		case "false":
			return phpv.ZBool(false).ZVal(), nil
		}
	}

	// PHP 8: using an undefined constant is a fatal Error
	return nil, phpobj.ThrowErrorAt(ctx, phpobj.Error, fmt.Sprintf("Undefined constant \"%s\"", r.c), r.l)
}

// builtinDeprecatedConstants maps constant names to their deprecation message.
var builtinDeprecatedConstants = map[phpv.ZString]string{
	"ASSERT_ACTIVE":    "Constant ASSERT_ACTIVE is deprecated since 8.3, as assert_options() is deprecated",
	"ASSERT_WARNING":   "Constant ASSERT_WARNING is deprecated since 8.3, as assert_options() is deprecated",
	"ASSERT_BAIL":      "Constant ASSERT_BAIL is deprecated since 8.3, as assert_options() is deprecated",
	"ASSERT_EXCEPTION": "Constant ASSERT_EXCEPTION is deprecated since 8.3, as assert_options() is deprecated",
	"ASSERT_CALLBACK":  "Constant ASSERT_CALLBACK is deprecated since 8.3, as assert_options() is deprecated",
	"FILE_BINARY":      "Constant FILE_BINARY is deprecated since 8.1, as the constant has no effect",
	"FILE_TEXT":         "Constant FILE_TEXT is deprecated since 8.1, as the constant has no effect",
}

// checkConstantDeprecated checks if a global constant has #[\Deprecated] and emits a warning.
// loc is the compile-time location of the constant access.
func checkConstantDeprecated(ctx phpv.Context, name phpv.ZString, loc *phpv.Loc) error {
	// Check built-in deprecated constants
	if msg, ok := builtinDeprecatedConstants[name]; ok {
		if loc != nil {
			return ctx.Deprecated("%s", msg, logopt.NoFuncName(true), logopt.Data{Loc: loc})
		}
		return ctx.Deprecated("%s", msg, logopt.NoFuncName(true))
	}

	attrs := ctx.Global().ConstantGetAttributes(name)
	for _, attr := range attrs {
		if attr.ClassName == "Deprecated" {
			// Skip if this attribute's args are currently being resolved
			// (prevents infinite recursion for self-referencing constants)
			if attr.Resolving {
				return nil
			}
			// If we're inside attribute argument resolution, use the outer access
			// site location instead of this constant reference's compile-time location.
			useLoc := loc
			if attrResolveLoc != nil {
				useLoc = attrResolveLoc
			}
			// Set the context location before resolving, so that ResolveAttrArgs
			// captures the correct access-site location for nested accesses.
			if useLoc != nil {
				ctx.Tick(ctx, useLoc)
			}
			// Resolve lazy argument expressions (e.g., forward-referenced constants).
			if err := ResolveAttrArgs(ctx, attr); err != nil {
				return err
			}
			msg := FormatDeprecatedMsg("Constant", string(name), attr)
			if useLoc != nil {
				return ctx.UserDeprecated("%s", msg, logopt.NoFuncName(true), logopt.Data{Loc: useLoc})
			}
			return ctx.UserDeprecated("%s", msg, logopt.NoFuncName(true))
		}
	}
	return nil
}
