package spl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type splFileInfoData struct {
	path         string      // original path as passed (for display)
	resolvedPath string      // resolved absolute path (for file operations)
	info         os.FileInfo
	fileClass    *phpobj.ZClass // class for openFile(), default SplFileObject
	infoClass    *phpobj.ZClass // class for getFileInfo()/getPathInfo(), default SplFileInfo
}

var SplFileInfoClass *phpobj.ZClass

func initFileInfo() {
	SplFileInfoClass = &phpobj.ZClass{
		Name: "SplFileInfo",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct":   {Name: "__construct", Method: phpobj.NativeMethod(sfiConstruct)},
			"getfilename":   {Name: "getFilename", Method: phpobj.NativeMethod(sfiGetFilename)},
			"getextension":  {Name: "getExtension", Method: phpobj.NativeMethod(sfiGetExtension)},
			"getbasename":   {Name: "getBasename", Method: phpobj.NativeMethod(sfiGetBasename)},
			"getpathname":   {Name: "getPathname", Method: phpobj.NativeMethod(sfiGetPathname)},
			"getpath":       {Name: "getPath", Method: phpobj.NativeMethod(sfiGetPath)},
			"getrealpath":   {Name: "getRealPath", Method: phpobj.NativeMethod(sfiGetRealPath)},
			"getsize":       {Name: "getSize", Method: phpobj.NativeMethod(sfiGetSize)},
			"gettype":       {Name: "getType", Method: phpobj.NativeMethod(sfiGetType)},
			"isdir":         {Name: "isDir", Method: phpobj.NativeMethod(sfiIsDir)},
			"isfile":        {Name: "isFile", Method: phpobj.NativeMethod(sfiIsFile)},
			"islink":        {Name: "isLink", Method: phpobj.NativeMethod(sfiIsLink)},
			"isreadable":    {Name: "isReadable", Method: phpobj.NativeMethod(sfiIsReadable)},
			"iswritable":    {Name: "isWritable", Method: phpobj.NativeMethod(sfiIsWritable)},
			"isexecutable":  {Name: "isExecutable", Method: phpobj.NativeMethod(sfiIsExecutable)},
			"getatime":      {Name: "getATime", Method: phpobj.NativeMethod(sfiGetATime)},
			"getmtime":      {Name: "getMTime", Method: phpobj.NativeMethod(sfiGetMTime)},
			"getctime":      {Name: "getCTime", Method: phpobj.NativeMethod(sfiGetCTime)},
			"getperms":      {Name: "getPerms", Method: phpobj.NativeMethod(sfiGetPerms)},
			"getinode":      {Name: "getInode", Method: phpobj.NativeMethod(sfiGetInode)},
			"getowner":      {Name: "getOwner", Method: phpobj.NativeMethod(sfiGetOwner)},
			"getgroup":      {Name: "getGroup", Method: phpobj.NativeMethod(sfiGetGroup)},
			"getlinktarget": {Name: "getLinkTarget", Method: phpobj.NativeMethod(sfiGetLinkTarget)},
			"openfile":      {Name: "openFile", Method: phpobj.NativeMethod(sfiOpenFile)},
			"setfileclass":  {Name: "setFileClass", Method: phpobj.NativeMethod(sfiSetFileClass)},
			"setinfoclass":  {Name: "setInfoClass", Method: phpobj.NativeMethod(sfiSetInfoClass)},
			"getfileinfo":   {Name: "getFileInfo", Method: phpobj.NativeMethod(sfiGetFileInfo)},
			"getpathinfo":   {Name: "getPathInfo", Method: phpobj.NativeMethod(sfiGetPathInfo)},
			"__tostring":    {Name: "__toString", Method: phpobj.NativeMethod(sfiToString)},
			"__debuginfo":   {Name: "__debugInfo", Method: phpobj.NativeMethod(sfiDebugInfo)},
		},
		H: &phpv.ZClassHandlers{},
	}
}

func getSFIData(o *phpobj.ZObject) *splFileInfoData {
	if d, ok := o.GetOpaque(SplFileInfoClass).(*splFileInfoData); ok {
		return d
	}
	return nil
}

func sfiConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	path := string(filename)

	// Check for null bytes
	if strings.ContainsRune(path, 0) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"SplFileInfo::__construct(): Argument #1 ($filename) must not contain any null bytes")
	}

	// Resolve relative paths against PHP CWD for file operations
	resolvedPath := path
	if !filepath.IsAbs(resolvedPath) {
		cwd := string(ctx.Global().Getwd())
		if cwd != "" {
			resolvedPath = filepath.Join(cwd, resolvedPath)
		}
	}

	info, _ := os.Stat(resolvedPath)
	data := &splFileInfoData{path: path, resolvedPath: resolvedPath, info: info}
	o.SetOpaque(SplFileInfoClass, data)
	return nil, nil
}

func sfiGetFilename(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(filepath.Base(d.path)), nil
}

func sfiGetExtension(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return phpv.ZStr(""), nil
	}
	ext := filepath.Ext(d.path)
	if len(ext) > 0 {
		ext = ext[1:] // strip leading dot
	}
	return phpv.ZStr(ext), nil
}

func sfiGetBasename(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return phpv.ZStr(""), nil
	}
	base := filepath.Base(d.path)
	if len(args) > 0 {
		suffix := string(args[0].AsString(ctx))
		if strings.HasSuffix(base, suffix) {
			base = base[:len(base)-len(suffix)]
		}
	}
	return phpv.ZStr(base), nil
}

func sfiGetPathname(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(d.path), nil
}

func sfiGetPath(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(filepath.Dir(d.path)), nil
}

func sfiGetRealPath(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	abs, err := filepath.Abs(sfiResolved(d))
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZStr(real), nil
}

func sfiGetSize(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			fmt.Sprintf("SplFileInfo::getSize(): stat failed for %s", sfiPathOrEmpty(d)))
	}
	return phpv.ZInt(d.info.Size()).ZVal(), nil
}

func sfiGetType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return phpv.ZStr("unknown"), nil
	}
	// Check for symlink using Lstat
	linfo, err := os.Lstat(sfiResolved(d))
	if err == nil && linfo.Mode()&os.ModeSymlink != 0 {
		return phpv.ZStr("link"), nil
	}
	if d.info.IsDir() {
		return phpv.ZStr("dir"), nil
	}
	return phpv.ZStr("file"), nil
}

func sfiIsDir(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	return phpv.ZBool(d != nil && d.info != nil && d.info.IsDir()).ZVal(), nil
}

func sfiIsFile(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	return phpv.ZBool(d != nil && d.info != nil && d.info.Mode().IsRegular()).ZVal(), nil
}

func sfiIsLink(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	linfo, err := os.Lstat(sfiResolved(d))
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(linfo.Mode()&os.ModeSymlink != 0).ZVal(), nil
}

func sfiIsReadable(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	f, err := os.Open(sfiResolved(d))
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	f.Close()
	return phpv.ZBool(true).ZVal(), nil
}

func sfiIsWritable(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	f, err := os.OpenFile(sfiResolved(d), os.O_WRONLY, 0)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	f.Close()
	return phpv.ZBool(true).ZVal(), nil
}

func sfiIsExecutable(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(d.info.Mode()&0111 != 0).ZVal(), nil
}

func sfiGetATime(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			fmt.Sprintf("SplFileInfo::getATime(): stat failed for %s", sfiPathOrEmpty(d)))
	}
	return phpv.ZInt(d.info.ModTime().Unix()).ZVal(), nil
}

func sfiGetMTime(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			fmt.Sprintf("SplFileInfo::getMTime(): stat failed for %s", sfiPathOrEmpty(d)))
	}
	return phpv.ZInt(d.info.ModTime().Unix()).ZVal(), nil
}

func sfiGetCTime(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			fmt.Sprintf("SplFileInfo::getCTime(): stat failed for %s", sfiPathOrEmpty(d)))
	}
	return phpv.ZInt(d.info.ModTime().Unix()).ZVal(), nil
}

func sfiGetPerms(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			fmt.Sprintf("SplFileInfo::getPerms(): stat failed for %s", sfiPathOrEmpty(d)))
	}
	return phpv.ZInt(d.info.Mode().Perm()).ZVal(), nil
}

func sfiGetInode(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			fmt.Sprintf("SplFileInfo::getInode(): stat failed for %s", sfiPathOrEmpty(d)))
	}
	sys := d.info.Sys()
	if stat, ok := sys.(*syscall.Stat_t); ok {
		return phpv.ZInt(stat.Ino).ZVal(), nil
	}
	return phpv.ZInt(0).ZVal(), nil
}

func sfiGetOwner(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			fmt.Sprintf("SplFileInfo::getOwner(): stat failed for %s", sfiPathOrEmpty(d)))
	}
	sys := d.info.Sys()
	if stat, ok := sys.(*syscall.Stat_t); ok {
		return phpv.ZInt(stat.Uid).ZVal(), nil
	}
	return phpv.ZInt(os.Getuid()).ZVal(), nil
}

func sfiGetGroup(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			fmt.Sprintf("SplFileInfo::getGroup(): stat failed for %s", sfiPathOrEmpty(d)))
	}
	sys := d.info.Sys()
	if stat, ok := sys.(*syscall.Stat_t); ok {
		return phpv.ZInt(stat.Gid).ZVal(), nil
	}
	return phpv.ZInt(os.Getgid()).ZVal(), nil
}

func sfiGetLinkTarget(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			"SplFileInfo::getLinkTarget(): Empty filename")
	}
	target, err := os.Readlink(sfiResolved(d))
	if err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			fmt.Sprintf("SplFileInfo::getLinkTarget(): Unable to read link %s, error: %s", d.path, err.Error()))
	}
	return phpv.ZStr(target), nil
}

func sfiOpenFile(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			"SplFileInfo::openFile(): Empty filename")
	}

	fileClass := SplFileObjectClass
	if d.fileClass != nil {
		fileClass = d.fileClass
	}

	// Forward all args to the SplFileObject constructor, but first arg is the filename
	constructArgs := []*phpv.ZVal{phpv.ZString(sfiResolved(d)).ZVal()}
	constructArgs = append(constructArgs, args...)

	obj, err := phpobj.NewZObject(ctx, fileClass, constructArgs...)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func sfiSetFileClass(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return nil, nil
	}

	className := phpv.ZString("SplFileObject")
	if len(args) > 0 {
		className = args[0].AsString(ctx)
	}

	cls, err := ctx.Global().GetClass(ctx, className, true)
	if err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("SplFileInfo::setFileClass(): Argument #1 ($class) must be a class name derived from SplFileObject, %s given", className))
	}

	// Check it's derived from SplFileObject
	zc, ok := cls.(*phpobj.ZClass)
	if !ok || (!zc.InstanceOf(SplFileObjectClass) && zc != SplFileObjectClass) {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("SplFileInfo::setFileClass(): Argument #1 ($class) must be a class name derived from SplFileObject, %s given", className))
	}

	d.fileClass = zc
	return nil, nil
}

func sfiSetInfoClass(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return nil, nil
	}

	className := phpv.ZString("SplFileInfo")
	if len(args) > 0 {
		className = args[0].AsString(ctx)
	}

	cls, err := ctx.Global().GetClass(ctx, className, true)
	if err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("SplFileInfo::setInfoClass(): Argument #1 ($class) must be a class name derived from SplFileInfo, %s given", className))
	}

	// Check it's derived from SplFileInfo
	zc, ok := cls.(*phpobj.ZClass)
	if !ok || (!zc.InstanceOf(SplFileInfoClass) && zc != SplFileInfoClass) {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("SplFileInfo::setInfoClass(): Argument #1 ($class) must be a class name derived from SplFileInfo, %s given", className))
	}

	d.infoClass = zc
	return nil, nil
}

func sfiGetFileInfo(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	infoClass := SplFileInfoClass
	if d.infoClass != nil {
		infoClass = d.infoClass
	}
	if len(args) > 0 {
		className := args[0].AsString(ctx)
		cls, err := ctx.Global().GetClass(ctx, className, true)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("SplFileInfo::getFileInfo(): Argument #1 ($class) must be a class name derived from SplFileInfo or null, %s given", className))
		}
		zc, ok := cls.(*phpobj.ZClass)
		if !ok || (!zc.InstanceOf(SplFileInfoClass) && zc != SplFileInfoClass) {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("SplFileInfo::getFileInfo(): Argument #1 ($class) must be a class name derived from SplFileInfo or null, %s given", className))
		}
		infoClass = zc
	}

	obj, err := phpobj.NewZObject(ctx, infoClass, phpv.ZString(d.path).ZVal())
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func sfiGetPathInfo(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return phpv.ZNULL.ZVal(), nil
	}

	infoClass := SplFileInfoClass
	if d.infoClass != nil {
		infoClass = d.infoClass
	}
	if len(args) > 0 {
		className := args[0].AsString(ctx)
		cls, err := ctx.Global().GetClass(ctx, className, true)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("SplFileInfo::getPathInfo(): Argument #1 ($class) must be a class name derived from SplFileInfo or null, %s given", className))
		}
		zc, ok := cls.(*phpobj.ZClass)
		if !ok || (!zc.InstanceOf(SplFileInfoClass) && zc != SplFileInfoClass) {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("SplFileInfo::getPathInfo(): Argument #1 ($class) must be a class name derived from SplFileInfo or null, %s given", className))
		}
		infoClass = zc
	}

	dirPath := filepath.Dir(d.path)
	obj, err := phpobj.NewZObject(ctx, infoClass, phpv.ZString(dirPath).ZVal())
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func sfiToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(d.path), nil
}

func sfiDebugInfo(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	arr := phpv.NewZArray()
	if d != nil {
		// Use \0ClassName\0propName format for private property display
		arr.OffsetSet(ctx, phpv.ZString("\x00SplFileInfo\x00pathName"), phpv.ZStr(d.path))
		arr.OffsetSet(ctx, phpv.ZString("\x00SplFileInfo\x00fileName"), phpv.ZStr(filepath.Base(d.path)))
	}
	return arr.ZVal(), nil
}

func sfiPathOrEmpty(d *splFileInfoData) string {
	if d == nil {
		return ""
	}
	return d.path
}

// sfiResolved returns the resolved (absolute) path for file operations.
func sfiResolved(d *splFileInfoData) string {
	if d.resolvedPath != "" {
		return d.resolvedPath
	}
	return d.path
}
