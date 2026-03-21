package standard

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > const
const (
	SCANDIR_SORT_ASCENDING phpv.ZInt = iota
	SCANDIR_SORT_DESCENDING
	SCANDIR_SORT_NONE
)

// > const
const (
	GLOB_ERR     phpv.ZInt = 0x1
	GLOB_MARK    phpv.ZInt = 0x2
	GLOB_NOSORT  phpv.ZInt = 0x4
	GLOB_NOCHECK phpv.ZInt = 0x10
	GLOB_NOESCAPE phpv.ZInt = 0x40
	GLOB_BRACE   phpv.ZInt = 0x400
	GLOB_ONLYDIR phpv.ZInt = 0x40000000
)

// DirectoryClass is the builtin Directory class returned by dir()
var DirectoryClass *phpobj.ZClass

func init() {
	DirectoryClass = &phpobj.ZClass{
		Name: "Directory",
		Props: []*phpv.ZClassProp{
			{VarName: "path", Default: phpv.ZStr("").ZVal(), Modifiers: phpv.ZAttrPublic},
			{VarName: "handle", Default: phpv.ZNULL.ZVal(), Modifiers: phpv.ZAttrPublic},
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"read": {Name: "read", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				handleZv := o.HashTable().GetString("handle")
				if handleZv == nil || handleZv.GetType() != phpv.ZtResource {
					return phpv.ZFalse.ZVal(), nil
				}
				dh, ok := handleZv.Value().(*dirHandle)
				if !ok {
					return phpv.ZFalse.ZVal(), nil
				}
				if dh.pos == -2 {
					dh.pos = -1
					return phpv.ZStr("."), nil
				}
				if dh.pos == -1 {
					dh.pos = 0
					return phpv.ZStr(".."), nil
				}
				if dh.pos >= len(dh.entries) {
					return phpv.ZFalse.ZVal(), nil
				}
				name := dh.entries[dh.pos].Name()
				dh.pos++
				return phpv.ZString(name).ZVal(), nil
			})},
			"rewind": {Name: "rewind", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				handleZv := o.HashTable().GetString("handle")
				if handleZv == nil || handleZv.GetType() != phpv.ZtResource {
					return phpv.ZNULL.ZVal(), nil
				}
				dh, ok := handleZv.Value().(*dirHandle)
				if !ok {
					return phpv.ZNULL.ZVal(), nil
				}
				dh.pos = -2
				return phpv.ZNULL.ZVal(), nil
			})},
			"close": {Name: "close", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZNULL.ZVal(), nil
			})},
		},
	}
}

// > func string getcwd ( void )
func fncGetcwd(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	cwd := ctx.Global().Getwd()
	if cwd == "" {
		return phpv.ZBool(false).ZVal(), nil
	}

	return cwd.ZVal(), nil
}

// > func bool chdir ( string $directory )
func fncChdir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var p phpv.ZString
	_, err := core.Expand(ctx, args, &p)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(p), "chdir"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	err = ctx.Global().Chdir(p)
	if err != nil {
		ctx.Warn("%s (errno 2)", err)
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

// > func array scandir ( string $directory [, int $sorting_order = SCANDIR_SORT_ASCENDING [, resource $context ]] )
func fncScanDir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dir phpv.ZString
	var sortingOrderArg *phpv.ZInt
	var context **phpv.ZVal // TODO: use context arg
	_, err := core.Expand(ctx, args, &dir, &sortingOrderArg, &context)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	sortingOrder := core.Deref(sortingOrderArg, SCANDIR_SORT_ASCENDING)

	if dir == "" {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "scandir(): Argument #1 ($directory) must not be empty")
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(dir), "scandir"); err != nil {
		ctx.Warn("scandir(%s): Failed to open directory: Operation not permitted", dir, logopt.NoFuncName(true))
		ctx.Warn("scandir(): (errno 1): Operation not permitted", logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}

	p := string(dir)
	if !filepath.IsAbs(p) {
		p = filepath.Join(string(ctx.Global().Getwd()), p)
	}
	files, err := os.ReadDir(p)
	if err != nil {
		ctx.Warn("scandir(%s): Failed to open directory: %s", dir, err, logopt.NoFuncName(true))
		ctx.Warn("scandir(): (errno 2): No such file or directory", logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}

	// Build full list including . and ..
	names := make([]string, 0, len(files)+2)
	names = append(names, ".", "..")
	for _, file := range files {
		names = append(names, file.Name())
	}

	// Sort based on sorting order
	switch sortingOrder {
	case SCANDIR_SORT_ASCENDING:
		sort.Strings(names)
	case SCANDIR_SORT_DESCENDING:
		sort.Sort(sort.Reverse(sort.StringSlice(names)))
	case SCANDIR_SORT_NONE:
		// No sorting needed
	default:
		// Any other non-zero value means descending in PHP
		if sortingOrder != 0 {
			sort.Sort(sort.Reverse(sort.StringSlice(names)))
		} else {
			sort.Strings(names)
		}
	}

	result := phpv.NewZArray()
	for _, name := range names {
		result.OffsetSet(ctx, nil, phpv.ZStr(name))
	}

	return result.ZVal(), nil
}

// > func Directory|false dir ( string $directory [, resource $context ] )
func fncDir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dirPath phpv.ZString
	_, err := core.Expand(ctx, args, &dirPath)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(dirPath), "dir"); err != nil {
		ctx.Warn("dir(%s): Failed to open directory: Operation not permitted", dirPath, logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}

	p := string(dirPath)
	if !filepath.IsAbs(p) {
		p = filepath.Join(string(ctx.Global().Getwd()), p)
	}

	entries, readErr := os.ReadDir(p)
	if readErr != nil {
		ctx.Warn("dir(%s): Failed to open directory: %s", dirPath, readErr, logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}

	dh := &dirHandle{
		entries: entries,
		pos:     -2,
		path:    p,
		id:      nextDirHandleID,
	}
	nextDirHandleID++

	obj, err := phpobj.NewZObject(ctx, DirectoryClass)
	if err != nil {
		return nil, err
	}
	obj.HashTable().SetString("path", phpv.ZString(p).ZVal())
	obj.HashTable().SetString("handle", dh.ZVal())

	return obj.ZVal(), nil
}

// > func array|false glob ( string $pattern [, int $flags = 0 ] )
func fncGlob(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pattern phpv.ZString
	var flagsArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &pattern, &flagsArg)
	if err != nil {
		return nil, err
	}

	flags := core.Deref(flagsArg, phpv.ZInt(0))
	pat := string(pattern)
	if pat == "" {
		return phpv.ZFalse.ZVal(), nil
	}

	cwd := string(ctx.Global().Getwd())
	hasBasedir := ctx.Global().GetConfig("open_basedir", phpv.ZNULL.ZVal()).String() != ""
	hasWildcard := strings.ContainsAny(pat, "*?[")

	if !hasWildcard {
		return globLiteral(ctx, pat, cwd, flags, hasBasedir)
	}
	return globWildcard(ctx, pat, cwd, flags, hasBasedir)
}

// globLiteral handles glob patterns with no wildcards (literal path check)
func globLiteral(ctx phpv.Context, pat string, cwd string, flags phpv.ZInt, hasBasedir bool) (*phpv.ZVal, error) {
	absPath := pat
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(cwd, absPath)
	}
	absPath = filepath.Clean(absPath)

	// Check basedir first - if blocked, return false
	if hasBasedir && !ctx.Global().IsWithinOpenBasedir(absPath) {
		return phpv.ZFalse.ZVal(), nil
	}

	// Check existence
	info, statErr := os.Stat(absPath)
	if statErr != nil {
		// GLOB_NOCHECK: return pattern if no matches
		if flags&GLOB_NOCHECK != 0 {
			result := phpv.NewZArray()
			result.OffsetSet(ctx, nil, phpv.ZString(pat).ZVal())
			return result.ZVal(), nil
		}
		return phpv.NewZArray().ZVal(), nil // empty array - no match
	}

	// Check GLOB_ONLYDIR
	if flags&GLOB_ONLYDIR != 0 && !info.IsDir() {
		return phpv.NewZArray().ZVal(), nil
	}

	resultPath := pat
	// GLOB_MARK: add trailing / for directories
	if flags&GLOB_MARK != 0 && info.IsDir() && !strings.HasSuffix(resultPath, "/") {
		resultPath += "/"
	}

	result := phpv.NewZArray()
	result.OffsetSet(ctx, nil, phpv.ZString(resultPath).ZVal())
	return result.ZVal(), nil
}

// globWildcard handles glob patterns with wildcards in the last path component
func globWildcard(ctx phpv.Context, pat string, cwd string, flags phpv.ZInt, hasBasedir bool) (*phpv.ZVal, error) {
	// Split into directory and filename pattern
	dir := path.Dir(pat)
	base := path.Base(pat)

	// Resolve directory to absolute for filesystem access
	absDir := dir
	if !filepath.IsAbs(absDir) {
		absDir = filepath.Join(cwd, absDir)
	}
	absDir = filepath.Clean(absDir)

	// Try to list directory
	entries, readErr := os.ReadDir(absDir)
	if readErr != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	filtered := false
	var matchedPaths []string

	for _, e := range entries {
		matched, _ := filepath.Match(base, e.Name())
		if !matched {
			continue
		}

		// GLOB_ONLYDIR
		if flags&GLOB_ONLYDIR != 0 && !e.IsDir() {
			continue
		}

		// Build result path preserving original format
		var resultPath string
		if dir == "." {
			resultPath = e.Name()
		} else {
			resultPath = dir + "/" + e.Name()
		}

		// GLOB_MARK: add trailing / for directories
		if flags&GLOB_MARK != 0 && e.IsDir() && !strings.HasSuffix(resultPath, "/") {
			resultPath += "/"
		}

		// Check basedir on the absolute resolved path
		absPath := filepath.Clean(filepath.Join(absDir, e.Name()))
		if hasBasedir && !ctx.Global().IsWithinOpenBasedir(absPath) {
			filtered = true
			continue
		}

		matchedPaths = append(matchedPaths, resultPath)
	}

	// Sort unless GLOB_NOSORT
	if flags&GLOB_NOSORT == 0 {
		sort.Strings(matchedPaths)
	}

	// If basedir filtered all results, return false
	if hasBasedir && filtered && len(matchedPaths) == 0 {
		return phpv.ZFalse.ZVal(), nil
	}

	// GLOB_NOCHECK: return pattern if no matches
	if flags&GLOB_NOCHECK != 0 && len(matchedPaths) == 0 {
		result := phpv.NewZArray()
		result.OffsetSet(ctx, nil, phpv.ZString(pat).ZVal())
		return result.ZVal(), nil
	}

	result := phpv.NewZArray()
	for _, p := range matchedPaths {
		result.OffsetSet(ctx, nil, phpv.ZString(p).ZVal())
	}
	return result.ZVal(), nil
}
