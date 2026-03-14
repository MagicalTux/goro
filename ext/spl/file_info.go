package spl

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type splFileInfoData struct {
	path string
	info os.FileInfo
}

var SplFileInfoClass *phpobj.ZClass

func initFileInfo() {
	SplFileInfoClass = &phpobj.ZClass{
		Name: "SplFileInfo",
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct":    {Name: "__construct", Method: phpobj.NativeMethod(sfiConstruct)},
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
			"__tostring":    {Name: "__toString", Method: phpobj.NativeMethod(sfiToString)},
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
	info, _ := os.Stat(path)
	data := &splFileInfoData{path: path, info: info}
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
	abs, err := filepath.Abs(d.path)
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
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.info.Size()).ZVal(), nil
}

func sfiGetType(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return phpv.ZStr("unknown"), nil
	}
	if d.info.IsDir() {
		return phpv.ZStr("dir"), nil
	}
	if d.info.Mode()&os.ModeSymlink != 0 {
		return phpv.ZStr("link"), nil
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
	linfo, err := os.Lstat(d.path)
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
	f, err := os.Open(d.path)
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
	f, err := os.OpenFile(d.path, os.O_WRONLY, 0)
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
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.info.ModTime().Unix()).ZVal(), nil
}

func sfiGetMTime(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.info.ModTime().Unix()).ZVal(), nil
}

func sfiGetCTime(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.info.ModTime().Unix()).ZVal(), nil
}

func sfiGetPerms(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil || d.info == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.info.Mode().Perm()).ZVal(), nil
}

func sfiGetInode(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(0).ZVal(), nil
}

func sfiGetOwner(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(os.Getuid()).ZVal(), nil
}

func sfiGetGroup(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(os.Getgid()).ZVal(), nil
}

func sfiToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFIData(o)
	if d == nil {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(d.path), nil
}
