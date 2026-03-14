package spl

import (
	"os"
	"path/filepath"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type directoryIteratorData struct {
	path    string
	entries []os.DirEntry
	pos     int
}

var DirectoryIteratorClass *phpobj.ZClass
var FilesystemIteratorClass *phpobj.ZClass

func initDirectoryIterator() {
	DirectoryIteratorClass = &phpobj.ZClass{
		Name:    "DirectoryIterator",
		Extends: SplFileInfoClass,
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: phpobj.NativeMethod(diConstruct)},
			"current":     {Name: "current", Method: phpobj.NativeMethod(diCurrent)},
			"key":         {Name: "key", Method: phpobj.NativeMethod(diKey)},
			"next":        {Name: "next", Method: phpobj.NativeMethod(diNext)},
			"rewind":      {Name: "rewind", Method: phpobj.NativeMethod(diRewind)},
			"valid":       {Name: "valid", Method: phpobj.NativeMethod(diValid)},
			"isdot":       {Name: "isDot", Method: phpobj.NativeMethod(diIsDot)},
			"__tostring":  {Name: "__toString", Method: phpobj.NativeMethod(diToString)},
		},
		H: &phpv.ZClassHandlers{},
	}

	FilesystemIteratorClass = &phpobj.ZClass{
		Name:    "FilesystemIterator",
		Extends: DirectoryIteratorClass,
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: phpobj.NativeMethod(diConstruct)},
		},
		H: &phpv.ZClassHandlers{},
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
		o.SetOpaque(SplFileInfoClass, &splFileInfoData{path: entryPath, info: info})
	}
}

func diCurrent(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return o.ZVal(), nil // DirectoryIterator returns $this
}

func diKey(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.pos).ZVal(), nil
}

func diNext(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, nil
	}
	d.pos++
	updateDISFI(o, d)
	return nil, nil
}

func diRewind(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil {
		return nil, nil
	}
	d.pos = 0
	updateDISFI(o, d)
	return nil, nil
}

func diValid(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	return phpv.ZBool(d != nil && d.pos < len(d.entries)).ZVal(), nil
}

func diIsDot(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil || d.pos >= len(d.entries) {
		return phpv.ZBool(false).ZVal(), nil
	}
	name := d.entries[d.pos].Name()
	return phpv.ZBool(name == "." || name == "..").ZVal(), nil
}

func diToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getDIData(o)
	if d == nil || d.pos >= len(d.entries) {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(d.entries[d.pos].Name()), nil
}

// fakeDirEntry implements os.DirEntry for . and ..
type fakeDirEntry struct {
	name string
}

func (f *fakeDirEntry) Name() string               { return f.name }
func (f *fakeDirEntry) IsDir() bool                 { return true }
func (f *fakeDirEntry) Type() os.FileMode           { return os.ModeDir }
func (f *fakeDirEntry) Info() (os.FileInfo, error)  { return nil, nil }
