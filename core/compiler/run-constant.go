package compiler

import (
	"fmt"
	"io"
	"strings"

	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
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
	return LookupConstant(ctx, r.c, r.noFallback, r.l)
}

// LookupConstant resolves a PHP constant by name, applying the same
// rules runConstant.Run did: case-insensitive null/true/false/self/
// parent, namespace-case-insensitive global lookup, qualified-name
// fallback to the global table, deprecation warnings via
// checkConstantDeprecated. Both the AST runner and the VM's
// OP_LOAD_CONSTANT_BY_NAME share this code path.
func LookupConstant(ctx phpv.Context, name string, noFallback bool, l *phpv.Loc) (*phpv.ZVal, error) {
	short := shortName(name)
	isQualified := short != name

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

	normalizedName := name
	if idx := strings.LastIndex(normalizedName, "\\"); idx >= 0 {
		normalizedName = strings.ToLower(normalizedName[:idx]) + normalizedName[idx:]
	}
	constName := phpv.ZString(normalizedName)
	if z, ok := ctx.Global().ConstantGet(constName); ok {
		if err := checkConstantDeprecated(ctx, constName, l); err != nil {
			return nil, err
		}
		return z.ZVal(), nil
	}

	if isQualified && !noFallback {
		shortZ := phpv.ZString(short)
		if z, ok := ctx.Global().ConstantGet(shortZ); ok {
			if err := checkConstantDeprecated(ctx, shortZ, l); err != nil {
				return nil, err
			}
			return z.ZVal(), nil
		}
		switch strings.ToLower(short) {
		case "null":
			return phpv.ZNull{}.ZVal(), nil
		case "true":
			return phpv.ZBool(true).ZVal(), nil
		case "false":
			return phpv.ZBool(false).ZVal(), nil
		}
	}

	return nil, phpobj.ThrowErrorAt(ctx, phpobj.Error, fmt.Sprintf("Undefined constant \"%s\"", name), l)
}

// builtinDeprecatedConstants maps constant names to their deprecation message.
var builtinDeprecatedConstants = map[phpv.ZString]string{
	"ASSERT_ACTIVE":    "Constant ASSERT_ACTIVE is deprecated since 8.3, as assert_options() is deprecated",
	"ASSERT_WARNING":   "Constant ASSERT_WARNING is deprecated since 8.3, as assert_options() is deprecated",
	"ASSERT_BAIL":      "Constant ASSERT_BAIL is deprecated since 8.3, as assert_options() is deprecated",
	"ASSERT_EXCEPTION": "Constant ASSERT_EXCEPTION is deprecated since 8.3, as assert_options() is deprecated",
	"ASSERT_CALLBACK":  "Constant ASSERT_CALLBACK is deprecated since 8.3, as assert_options() is deprecated",
	"FILE_BINARY":           "Constant FILE_BINARY is deprecated since 8.1, as the constant has no effect",
	"FILE_TEXT":              "Constant FILE_TEXT is deprecated since 8.1, as the constant has no effect",
	"SUNFUNCS_RET_TIMESTAMP": "Constant SUNFUNCS_RET_TIMESTAMP is deprecated since 8.4, as date_sunrise() and date_sunset() were deprecated in 8.1",
	"SUNFUNCS_RET_STRING":    "Constant SUNFUNCS_RET_STRING is deprecated since 8.4, as date_sunrise() and date_sunset() were deprecated in 8.1",
	"SUNFUNCS_RET_DOUBLE":    "Constant SUNFUNCS_RET_DOUBLE is deprecated since 8.4, as date_sunrise() and date_sunset() were deprecated in 8.1",
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
