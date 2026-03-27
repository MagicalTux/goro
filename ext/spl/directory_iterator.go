package spl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// FilesystemIterator flag constants (matching PHP)
const (
	fsIterCurrentAsPathname = 0x00020 // 32
	fsIterCurrentAsSelf     = 0x00010 // 16
	fsIterCurrentAsFileinfo = 0x00000 // 0
	fsIterKeyAsPathname     = 0x00000 // 0
	fsIterKeyAsFilename     = 0x00100 // 256
	fsIterFollowSymlinks    = 0x00200 // 512
	fsIterNewCurrentAndKey  = 0x00100 // 256
	fsIterSkipDots          = 0x01000 // 4096
	fsIterUnixPaths         = 0x02000 // 8192
)

type directoryIteratorData struct {
	path    string
	entries []os.DirEntry
	pos     int
	flags   int
}

var DirectoryIteratorClass *phpobj.ZClass
var FilesystemIteratorClass *phpobj.ZClass
var GlobIteratorClass *phpobj.ZClass

func initDirectoryIterator() {
	// Build DirectoryIterator methods: SplFileInfo + DirectoryIterator's own
	diMethods := make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range SplFileInfoClass.Methods {
		diMethods[k] = v
	}
	diOwnMethods := map[phpv.ZString]*phpv.ZClassMethod{
		"__construct":  {Name: "__construct", Method: phpobj.NativeMethod(diConstruct)},
		"current":      {Name: "current", Method: phpobj.NativeMethod(diCurrent)},
		"key":          {Name: "key", Method: phpobj.NativeMethod(diKey)},
		"next":         {Name: "next", Method: phpobj.NativeMethod(diNext)},
		"rewind":       {Name: "rewind", Method: phpobj.NativeMethod(diRewind)},
		"valid":        {Name: "valid", Method: phpobj.NativeMethod(diValid)},
		"isdot":        {Name: "isDot", Method: phpobj.NativeMethod(diIsDot)},
		"seek":         {Name: "seek", Method: phpobj.NativeMethod(diSeek)},
		"getfilename":  {Name: "getFilename", Method: phpobj.NativeMethod(diGetFilename)},
		"getextension": {Name: "getExtension", Method: phpobj.NativeMethod(diGetExtension)},
		"getbasename":  {Name: "getBasename", Method: phpobj.NativeMethod(diGetBasename)},
		"__tostring":   {Name: "__toString", Method: phpobj.NativeMethod(diToString)},
	}
	for k, v := range diOwnMethods {
		diMethods[k] = v
	}

	DirectoryIteratorClass = &phpobj.ZClass{
		Name:            "DirectoryIterator",
		Extends:         SplFileInfoClass,
		Implementations: []*phpobj.ZClass{SeekableIterator, phpobj.Stringable},
		Methods:         diMethods,
		H:               &phpv.ZClassHandlers{},
	}

	// Build FilesystemIterator methods: DirectoryIterator + FilesystemIterator's own
	fsiMethods := make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range diMethods {
		fsiMethods[k] = v
	}
	fsiOwnMethods := map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {Name: "__construct", Method: phpobj.NativeMethod(fsiConstruct)},
		"current":     {Name: "current", Method: phpobj.NativeMethod(fsiCurrent)},
		"key":         {Name: "key", Method: phpobj.NativeMethod(fsiKey)},
		"rewind":      {Name: "rewind", Method: phpobj.NativeMethod(fsiRewind)},
		"next":        {Name: "next", Method: phpobj.NativeMethod(fsiNext)},
		"getflags":    {Name: "getFlags", Method: phpobj.NativeMethod(fsiGetFlags)},
		"setflags":    {Name: "setFlags", Method: phpobj.NativeMethod(fsiSetFlags)},
		"__tostring":  {Name: "__toString", Method: phpobj.NativeMethod(fsiToString)},
	}
	for k, v := range fsiOwnMethods {
		fsiMethods[k] = v
	}

	FilesystemIteratorClass = &phpobj.ZClass{
		Name:    "FilesystemIterator",
		Extends: DirectoryIteratorClass,
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"CURRENT_AS_PATHNAME": {Value: phpv.ZInt(fsIterCurrentAsPathname)},
			"CURRENT_AS_FILEINFO": {Value: phpv.ZInt(fsIterCurrentAsFileinfo)},
			"CURRENT_AS_SELF":     {Value: phpv.ZInt(fsIterCurrentAsSelf)},
			"KEY_AS_PATHNAME":     {Value: phpv.ZInt(fsIterKeyAsPathname)},
			"KEY_AS_FILENAME":     {Value: phpv.ZInt(fsIterKeyAsFilename)},
			"FOLLOW_SYMLINKS":     {Value: phpv.ZInt(fsIterFollowSymlinks)},
			"NEW_CURRENT_AND_KEY": {Value: phpv.ZInt(fsIterNewCurrentAndKey)},
			"SKIP_DOTS":           {Value: phpv.ZInt(fsIterSkipDots)},
			"UNIX_PATHS":          {Value: phpv.ZInt(fsIterUnixPaths)},
			"CURRENT_MODE_MASK":   {Value: phpv.ZInt(0x00F0)},
			"KEY_MODE_MASK":       {Value: phpv.ZInt(0x0F00)},
			"OTHER_MODE_MASK":     {Value: phpv.ZInt(0xF000)},
		},
		Methods: fsiMethods,
		H:       &phpv.ZClassHandlers{},
	}

	// Build GlobIterator methods: FilesystemIterator + GlobIterator's own
	globMethods := make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range fsiMethods {
		globMethods[k] = v
	}
	globMethods["__construct"] = &phpv.ZClassMethod{Name: "__construct", Method: phpobj.NativeMethod(globIterConstruct)}
	globMethods["count"] = &phpv.ZClassMethod{Name: "count", Method: phpobj.NativeMethod(globIterCount)}

	GlobIteratorClass = &phpobj.ZClass{
		Name:            "GlobIterator",
		Extends:         FilesystemIteratorClass,
		Implementations: []*phpobj.ZClass{Countable},
		Methods:         globMethods,
		H:               &phpv.ZClassHandlers{},
	}
}

func getDIData(o *phpobj.ZObject) *directoryIteratorData {
	if d, ok := o.GetOpaque(DirectoryIteratorClass).(*directoryIteratorData); ok {
		return d
	}
	return nil
}

func diConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var path phpv.ZString
	_, err := core.Expand(ctx, args, &path)
	if err != nil {
		return nil, err
	}

	dirPath := string(path)

	// Check for empty string
	if dirPath == "" {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"DirectoryIterator::__construct(): Argument #1 ($directory) must not be empty")
	}

	// Check for null bytes
	if strings.ContainsRune(dirPath, 0) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"DirectoryIterator::__construct(): Argument #1 ($directory) must not contain any null bytes")
	}

	// Resolve relative paths against PHP CWD
	if !filepath.IsAbs(dirPath) {
		cwd := string(ctx.Global().Getwd())
		if cwd != "" {
			dirPath = filepath.Join(cwd, dirPath)
		}
	}

	entries, err2 := os.ReadDir(dirPath)
	if err2 != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			"DirectoryIterator::__construct("+dirPath+"): failed to open dir")
	}

	// Add . and .. entries at the beginning
	data := &directoryIteratorData{
		path: dirPath,
		entries: append([]os.DirEntry{
			&fakeDirEntry{name: "."},
			&fakeDirEntry{name: ".."},
		}, entries...),
		pos: 0,
	}
	o.SetOpaque(DirectoryIteratorClass, data)

	// Also set up SplFileInfo for current entry
	updateDISFI(o, data)
	return nil, nil
}

func updateDISFI(o *phpobj.ZObject, d *directoryIteratorData) {
	if d.pos < len(d.entries) {
		entryPath := filepath.Join(d.path, d.entries[d.pos].Name())
		info, _ := os.Stat(entryPath)
		o.SetOpaque(SplFileInfoClass, &splFileInfoData{path: entryPath, resolvedPath: entryPath, info: info})
	}
}

func diCurrent(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Object not initialized")
	}
	return o.ZVal(), nil // DirectoryIterator returns $this
}

func diKey(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Object not initialized")
	}
	return phpv.ZInt(d.pos).ZVal(), nil
}

func diNext(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Object not initialized")
	}
	d.pos++
	updateDISFI(o, d)
	return nil, nil
}

func diRewind(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Object not initialized")
	}
	d.pos = 0
	updateDISFI(o, d)
	return nil, nil
}

func diValid(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Object not initialized")
	}
	return phpv.ZBool(d.pos < len(d.entries)).ZVal(), nil
}

func diIsDot(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Object not initialized")
	}
	if d.pos >= len(d.entries) {
		return phpv.ZBool(false).ZVal(), nil
	}
	name := d.entries[d.pos].Name()
	return phpv.ZBool(name == "." || name == "..").ZVal(), nil
}

func diSeek(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, nil
	}

	var pos phpv.ZInt
	_, err := core.Expand(ctx, args, &pos)
	if err != nil {
		return nil, err
	}

	target := int(pos)
	if target < 0 || target >= len(d.entries) {
		return nil, phpobj.ThrowError(ctx, phpobj.OutOfBoundsException,
			fmt.Sprintf("Seek position %d is out of range", target))
	}

	d.pos = target
	updateDISFI(o, d)
	return nil, nil
}

func diGetFilename(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Object not initialized")
	}
	if d.pos >= len(d.entries) {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(d.entries[d.pos].Name()), nil
}

func diGetExtension(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Object not initialized")
	}
	if d.pos >= len(d.entries) {
		return phpv.ZStr(""), nil
	}
	name := d.entries[d.pos].Name()
	ext := filepath.Ext(name)
	if len(ext) > 0 {
		ext = ext[1:]
	}
	return phpv.ZStr(ext), nil
}

func diGetBasename(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Object not initialized")
	}
	if d.pos >= len(d.entries) {
		return phpv.ZStr(""), nil
	}
	name := d.entries[d.pos].Name()
	if len(args) > 0 && args[0] != nil {
		suffix := string(args[0].AsString(ctx))
		if strings.HasSuffix(name, suffix) && len(name) > len(suffix) {
			name = name[:len(name)-len(suffix)]
		}
	}
	return phpv.ZStr(name), nil
}

func diToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Object not initialized")
	}
	if d.pos >= len(d.entries) {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(d.entries[d.pos].Name()), nil
}

// ---- FilesystemIterator ----

func fsiConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var path phpv.ZString
	_, err := core.Expand(ctx, args, &path)
	if err != nil {
		return nil, err
	}

	dirPath := string(path)

	// Handle glob:// wrapper
	if strings.HasPrefix(dirPath, "glob://") {
		dirPath = dirPath[7:]
		return globIterInit(ctx, o, dirPath, args)
	}

	// Check for empty string
	if dirPath == "" {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"FilesystemIterator::__construct(): Argument #1 ($directory) must not be empty")
	}

	// Check for null bytes
	if strings.ContainsRune(dirPath, 0) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"FilesystemIterator::__construct(): Argument #1 ($directory) must not contain any null bytes")
	}

	flags := fsIterKeyAsPathname | fsIterCurrentAsFileinfo | fsIterSkipDots
	if len(args) > 1 {
		flags = int(args[1].AsInt(ctx))
	}

	entries, err2 := os.ReadDir(dirPath)
	if err2 != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			"FilesystemIterator::__construct("+dirPath+"): failed to open dir")
	}

	// Add . and .. entries at the beginning
	allEntries := append([]os.DirEntry{
		&fakeDirEntry{name: "."},
		&fakeDirEntry{name: ".."},
	}, entries...)

	data := &directoryIteratorData{
		path:    dirPath,
		entries: allEntries,
		pos:     0,
		flags:   flags,
	}
	o.SetOpaque(DirectoryIteratorClass, data)

	// Skip dots on rewind if SKIP_DOTS is set
	if flags&fsIterSkipDots != 0 {
		for data.pos < len(data.entries) {
			name := data.entries[data.pos].Name()
			if name == "." || name == ".." {
				data.pos++
				continue
			}
			break
		}
	}

	updateDISFI(o, data)
	return nil, nil
}

func fsiCurrent(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil || d.pos >= len(d.entries) {
		return phpv.ZBool(false).ZVal(), nil
	}

	if d.flags&fsIterCurrentAsPathname != 0 {
		return phpv.ZStr(filepath.Join(d.path, d.entries[d.pos].Name())), nil
	}
	if d.flags&fsIterCurrentAsSelf != 0 {
		return o.ZVal(), nil
	}

	// Default: CURRENT_AS_FILEINFO - return a new SplFileInfo object
	entryPath := filepath.Join(d.path, d.entries[d.pos].Name())
	infoObj, err := phpobj.NewZObject(ctx, SplFileInfoClass, phpv.ZString(entryPath).ZVal())
	if err != nil {
		return nil, err
	}
	return infoObj.ZVal(), nil
}

func fsiKey(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil || d.pos >= len(d.entries) {
		return phpv.ZStr(""), nil
	}

	if d.flags&fsIterKeyAsFilename != 0 {
		return phpv.ZStr(d.entries[d.pos].Name()), nil
	}
	// Default: KEY_AS_PATHNAME
	return phpv.ZStr(filepath.Join(d.path, d.entries[d.pos].Name())), nil
}

func fsiRewind(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, nil
	}

	d.pos = 0
	if d.flags&fsIterSkipDots != 0 {
		for d.pos < len(d.entries) {
			name := d.entries[d.pos].Name()
			if name == "." || name == ".." {
				d.pos++
				continue
			}
			break
		}
	}
	updateDISFI(o, d)
	return nil, nil
}

func fsiNext(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, nil
	}
	d.pos++
	if d.flags&fsIterSkipDots != 0 {
		for d.pos < len(d.entries) {
			name := d.entries[d.pos].Name()
			if name == "." || name == ".." {
				d.pos++
				continue
			}
			break
		}
	}
	updateDISFI(o, d)
	return nil, nil
}

func fsiGetFlags(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.flags).ZVal(), nil
}

func fsiSetFlags(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, nil
	}
	var flags phpv.ZInt
	_, err := core.Expand(ctx, args, &flags)
	if err != nil {
		return nil, err
	}
	d.flags = int(flags)
	return nil, nil
}

func fsiToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil || d.pos >= len(d.entries) {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(filepath.Join(d.path, d.entries[d.pos].Name())), nil
}

// ---- GlobIterator ----

func globIterConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pattern phpv.ZString
	_, err := core.Expand(ctx, args, &pattern)
	if err != nil {
		return nil, err
	}

	patternStr := string(pattern)
	// Strip glob:// prefix if present
	if strings.HasPrefix(patternStr, "glob://") {
		patternStr = patternStr[7:]
	}

	return globIterInit(ctx, o, patternStr, args)
}

func globIterInit(ctx phpv.Context, o *phpobj.ZObject, pattern string, args []*phpv.ZVal) (*phpv.ZVal, error) {
	flags := fsIterKeyAsPathname | fsIterCurrentAsFileinfo | fsIterSkipDots
	if len(args) > 1 {
		flags = int(args[1].AsInt(ctx))
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			fmt.Sprintf("GlobIterator::__construct(): glob pattern error"))
	}

	// Convert matches to DirEntry-like entries
	var entries []os.DirEntry
	for _, m := range matches {
		info, _ := os.Stat(m)
		entries = append(entries, &globDirEntry{name: filepath.Base(m), info: info, path: m})
	}

	// For glob iterators, the "path" is the directory part of the pattern
	dirPath := filepath.Dir(pattern)

	data := &directoryIteratorData{
		path:    dirPath,
		entries: entries,
		pos:     0,
		flags:   flags,
	}
	o.SetOpaque(DirectoryIteratorClass, data)
	if len(entries) > 0 {
		updateDISFI(o, data)
	}
	return nil, nil
}

func globIterCount(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "GlobIterator is not initialized")
	}
	return phpv.ZInt(len(d.entries)).ZVal(), nil
}

// fakeDirEntry implements os.DirEntry for . and ..
type fakeDirEntry struct {
	name string
}

func (f *fakeDirEntry) Name() string               { return f.name }
func (f *fakeDirEntry) IsDir() bool                 { return true }
func (f *fakeDirEntry) Type() os.FileMode           { return os.ModeDir }
func (f *fakeDirEntry) Info() (os.FileInfo, error)  { return nil, nil }

// globDirEntry implements os.DirEntry for glob matches
type globDirEntry struct {
	name string
	info os.FileInfo
	path string
}

func (g *globDirEntry) Name() string               { return g.name }
func (g *globDirEntry) IsDir() bool                 { return g.info != nil && g.info.IsDir() }
func (g *globDirEntry) Type() os.FileMode {
	if g.info != nil {
		return g.info.Mode().Type()
	}
	return 0
}
func (g *globDirEntry) Info() (os.FileInfo, error)  { return g.info, nil }
