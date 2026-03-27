package spl

import (
	"os"
	"path/filepath"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type recursiveDirectoryIteratorData struct {
	path    string
	entries []os.DirEntry
	pos     int
	flags   int
	subPath string // relative sub-path from original root
}

var RecursiveDirectoryIteratorClass *phpobj.ZClass

func initRecursiveDirectoryIterator() {
	RecursiveDirectoryIteratorClass = &phpobj.ZClass{
		Name:            "RecursiveDirectoryIterator",
		Extends:         FilesystemIteratorClass,
		Implementations: []*phpobj.ZClass{RecursiveIterator},
		H:               &phpv.ZClassHandlers{},
	}

	// Copy methods from parent chain: SplFileInfo -> DirectoryIterator -> FilesystemIterator
	RecursiveDirectoryIteratorClass.Methods = make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range SplFileInfoClass.Methods {
		RecursiveDirectoryIteratorClass.Methods[k] = v
	}
	for k, v := range DirectoryIteratorClass.Methods {
		RecursiveDirectoryIteratorClass.Methods[k] = v
	}
	for k, v := range FilesystemIteratorClass.Methods {
		RecursiveDirectoryIteratorClass.Methods[k] = v
	}

	// Override with RecursiveDirectoryIterator-specific methods
	RecursiveDirectoryIteratorClass.Methods["__construct"] = &phpv.ZClassMethod{Name: "__construct", Method: phpobj.NativeMethod(rdiConstruct)}
	RecursiveDirectoryIteratorClass.Methods["haschildren"] = &phpv.ZClassMethod{Name: "hasChildren", Method: phpobj.NativeMethod(rdiHasChildren)}
	RecursiveDirectoryIteratorClass.Methods["getchildren"] = &phpv.ZClassMethod{Name: "getChildren", Method: phpobj.NativeMethod(rdiGetChildren)}
	RecursiveDirectoryIteratorClass.Methods["getsubpath"] = &phpv.ZClassMethod{Name: "getSubPath", Method: phpobj.NativeMethod(rdiGetSubPath)}
	RecursiveDirectoryIteratorClass.Methods["getsubpathname"] = &phpv.ZClassMethod{Name: "getSubPathname", Method: phpobj.NativeMethod(rdiGetSubPathname)}
	RecursiveDirectoryIteratorClass.Methods["current"] = &phpv.ZClassMethod{Name: "current", Method: phpobj.NativeMethod(rdiCurrent)}
	RecursiveDirectoryIteratorClass.Methods["key"] = &phpv.ZClassMethod{Name: "key", Method: phpobj.NativeMethod(rdiKey)}
	RecursiveDirectoryIteratorClass.Methods["next"] = &phpv.ZClassMethod{Name: "next", Method: phpobj.NativeMethod(rdiNext)}
	RecursiveDirectoryIteratorClass.Methods["rewind"] = &phpv.ZClassMethod{Name: "rewind", Method: phpobj.NativeMethod(rdiRewind)}
	RecursiveDirectoryIteratorClass.Methods["valid"] = &phpv.ZClassMethod{Name: "valid", Method: phpobj.NativeMethod(rdiValid)}
	RecursiveDirectoryIteratorClass.Methods["isdot"] = &phpv.ZClassMethod{Name: "isDot", Method: phpobj.NativeMethod(rdiIsDot)}
	RecursiveDirectoryIteratorClass.Methods["__tostring"] = &phpv.ZClassMethod{Name: "__toString", Method: phpobj.NativeMethod(rdiToString)}
	RecursiveDirectoryIteratorClass.Methods["getflags"] = &phpv.ZClassMethod{Name: "getFlags", Method: phpobj.NativeMethod(rdiGetFlags)}
	RecursiveDirectoryIteratorClass.Methods["setflags"] = &phpv.ZClassMethod{Name: "setFlags", Method: phpobj.NativeMethod(rdiSetFlags)}
}

func getRDIData(o *phpobj.ZObject) *recursiveDirectoryIteratorData {
	if d, ok := o.GetOpaque(RecursiveDirectoryIteratorClass).(*recursiveDirectoryIteratorData); ok {
		return d
	}
	return nil
}

func rdiConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var path phpv.ZString
	_, err := core.Expand(ctx, args, &path)
	if err != nil {
		return nil, err
	}

	flags := fsIterKeyAsPathname | fsIterCurrentAsFileinfo | fsIterSkipDots
	if len(args) > 1 {
		flags = int(args[1].AsInt(ctx))
	}

	dirPath := string(path)
	entries, err2 := os.ReadDir(dirPath)
	if err2 != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.UnexpectedValueException,
			"RecursiveDirectoryIterator::__construct("+dirPath+"): failed to open dir")
	}

	// Add . and .. entries at the beginning
	allEntries := append([]os.DirEntry{
		&fakeDirEntry{name: "."},
		&fakeDirEntry{name: ".."},
	}, entries...)

	data := &recursiveDirectoryIteratorData{
		path:    dirPath,
		entries: allEntries,
		pos:     0,
		flags:   flags,
		subPath: "",
	}
	o.SetOpaque(RecursiveDirectoryIteratorClass, data)

	// Also set up DirectoryIterator data so parent methods work
	diData := &directoryIteratorData{
		path:    dirPath,
		entries: allEntries,
		pos:     0,
		flags:   flags,
	}
	o.SetOpaque(DirectoryIteratorClass, diData)

	// Skip dots on initial position if SKIP_DOTS is set
	if flags&fsIterSkipDots != 0 {
		for data.pos < len(data.entries) {
			name := data.entries[data.pos].Name()
			if name == "." || name == ".." {
				data.pos++
				continue
			}
			break
		}
		diData.pos = data.pos
	}

	// Set up SplFileInfo
	updateRDISFI(o, data)
	return nil, nil
}

func updateRDISFI(o *phpobj.ZObject, d *recursiveDirectoryIteratorData) {
	if d.pos < len(d.entries) {
		entryPath := entryFullPath(d.path, d.entries[d.pos].Name())
		resolvedPath := filepath.Join(d.path, d.entries[d.pos].Name())
		info, _ := os.Stat(resolvedPath)
		o.SetOpaque(SplFileInfoClass, &splFileInfoData{path: entryPath, resolvedPath: resolvedPath, info: info})
	}
}

func rdiCurrent(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getRDIData(o)
	if d == nil || d.pos >= len(d.entries) {
		return phpv.ZBool(false).ZVal(), nil
	}

	if d.flags&fsIterCurrentAsPathname != 0 {
		return phpv.ZStr(entryFullPath(d.path, d.entries[d.pos].Name())), nil
	}
	if d.flags&fsIterCurrentAsSelf != 0 {
		return o.ZVal(), nil
	}

	// Default: CURRENT_AS_FILEINFO
	entryPath := entryFullPath(d.path, d.entries[d.pos].Name())
	infoObj, err := phpobj.NewZObject(ctx, SplFileInfoClass, phpv.ZString(entryPath).ZVal())
	if err != nil {
		return nil, err
	}
	return infoObj.ZVal(), nil
}

func rdiKey(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getRDIData(o)
	if d == nil || d.pos >= len(d.entries) {
		return phpv.ZStr(""), nil
	}

	if d.flags&fsIterKeyAsFilename != 0 {
		return phpv.ZStr(d.entries[d.pos].Name()), nil
	}
	return phpv.ZStr(entryFullPath(d.path, d.entries[d.pos].Name())), nil
}

func rdiNext(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getRDIData(o)
	if d == nil {
		return nil, nil
	}
	d.pos++
	// Skip dots if SKIP_DOTS is set
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
	updateRDISFI(o, d)
	// Keep DirectoryIterator data in sync
	if diData, ok := o.GetOpaque(DirectoryIteratorClass).(*directoryIteratorData); ok {
		diData.pos = d.pos
	}
	return nil, nil
}

func rdiRewind(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getRDIData(o)
	if d == nil {
		return nil, nil
	}
	d.pos = 0
	// Skip dots if SKIP_DOTS is set
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
	updateRDISFI(o, d)
	if diData, ok := o.GetOpaque(DirectoryIteratorClass).(*directoryIteratorData); ok {
		diData.pos = d.pos
	}
	return nil, nil
}

func rdiValid(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getRDIData(o)
	return phpv.ZBool(d != nil && d.pos < len(d.entries)).ZVal(), nil
}

func rdiIsDot(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getRDIData(o)
	if d == nil || d.pos >= len(d.entries) {
		return phpv.ZBool(false).ZVal(), nil
	}
	name := d.entries[d.pos].Name()
	return phpv.ZBool(name == "." || name == "..").ZVal(), nil
}

func rdiToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getRDIData(o)
	if d == nil || d.pos >= len(d.entries) {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(entryFullPath(d.path, d.entries[d.pos].Name())), nil
}

func rdiHasChildren(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getRDIData(o)
	if d == nil || d.pos >= len(d.entries) {
		return phpv.ZFalse.ZVal(), nil
	}
	entry := d.entries[d.pos]
	name := entry.Name()
	if name == "." || name == ".." {
		return phpv.ZFalse.ZVal(), nil
	}

	entryPath := filepath.Join(d.path, name) // resolved path for OS operations

	// If FOLLOW_SYMLINKS is set, use os.Stat (follows symlinks)
	// Otherwise, use os.Lstat (does not follow)
	if d.flags&fsIterFollowSymlinks != 0 {
		info, err := os.Stat(entryPath)
		if err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
		return phpv.ZBool(info.IsDir()).ZVal(), nil
	}

	// Without FOLLOW_SYMLINKS, symlinks to dirs are not considered children
	linfo, err := os.Lstat(entryPath)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZBool(linfo.IsDir()).ZVal(), nil
}

func rdiGetChildren(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getRDIData(o)
	if d == nil || d.pos >= len(d.entries) {
		return phpv.ZNULL.ZVal(), nil
	}
	entry := d.entries[d.pos]
	childPath := filepath.Join(d.path, entry.Name())

	child, err := phpobj.NewZObject(ctx, RecursiveDirectoryIteratorClass,
		phpv.ZString(childPath).ZVal(), phpv.ZInt(d.flags).ZVal())
	if err != nil {
		return nil, err
	}

	// Set the subPath on the child
	childRDI := getRDIDataFromObj(child)
	if childRDI != nil {
		if d.subPath == "" {
			childRDI.subPath = entry.Name()
		} else {
			childRDI.subPath = d.subPath + string(filepath.Separator) + entry.Name()
		}
	}

	return child.ZVal(), nil
}

func getRDIDataFromObj(o *phpobj.ZObject) *recursiveDirectoryIteratorData {
	if d, ok := o.GetOpaque(RecursiveDirectoryIteratorClass).(*recursiveDirectoryIteratorData); ok {
		return d
	}
	return nil
}

func rdiGetSubPath(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getRDIData(o)
	if d == nil {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(d.subPath), nil
}

func rdiGetSubPathname(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getRDIData(o)
	if d == nil || d.pos >= len(d.entries) {
		return phpv.ZStr(""), nil
	}
	name := d.entries[d.pos].Name()
	if d.subPath == "" {
		return phpv.ZStr(name), nil
	}
	return phpv.ZStr(d.subPath + string(filepath.Separator) + name), nil
}

func rdiGetFlags(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getRDIData(o)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.flags).ZVal(), nil
}

func rdiSetFlags(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getRDIData(o)
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
