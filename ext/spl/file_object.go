package spl

import (
	"bufio"
	"fmt"
	"os"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type splFileObjectData struct {
	splFileInfoData
	file    *os.File
	scanner *bufio.Scanner
	line    int
	curLine string
	eof     bool
	flags   int
}

var SplFileObjectClass *phpobj.ZClass
var SplTempFileObjectClass *phpobj.ZClass

func initSplFileObject() {
	SplFileObjectClass = &phpobj.ZClass{
		Name: "SplFileObject",
		Extends: SplFileInfoClass,
		Implementations: []*phpobj.ZClass{},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: phpobj.NativeMethod(sfoConstruct)},
			"fgets":       {Name: "fgets", Method: phpobj.NativeMethod(sfoFgets)},
			"fgetcsv":     {Name: "fgetcsv", Method: phpobj.NativeMethod(sfoFgetcsv)},
			"eof":         {Name: "eof", Method: phpobj.NativeMethod(sfoEof)},
			"fwrite":      {Name: "fwrite", Method: phpobj.NativeMethod(sfoFwrite)},
			"fflush":      {Name: "fflush", Method: phpobj.NativeMethod(sfoFflush)},
			"ftell":       {Name: "ftell", Method: phpobj.NativeMethod(sfoFtell)},
			"fseek":       {Name: "fseek", Method: phpobj.NativeMethod(sfoFseek)},
			"rewind":      {Name: "rewind", Method: phpobj.NativeMethod(sfoRewind)},
			"current":     {Name: "current", Method: phpobj.NativeMethod(sfoCurrent)},
			"key":         {Name: "key", Method: phpobj.NativeMethod(sfoKey)},
			"next":        {Name: "next", Method: phpobj.NativeMethod(sfoNext)},
			"valid":       {Name: "valid", Method: phpobj.NativeMethod(sfoValid)},
			"setflags":    {Name: "setFlags", Method: phpobj.NativeMethod(sfoSetFlags)},
			"getflags":    {Name: "getFlags", Method: phpobj.NativeMethod(sfoGetFlags)},
			"__tostring":  {Name: "__toString", Method: phpobj.NativeMethod(sfoToString)},
		},
		H: &phpv.ZClassHandlers{},
	}

	SplTempFileObjectClass = &phpobj.ZClass{
		Name:    "SplTempFileObject",
		Extends: SplFileObjectClass,
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {Name: "__construct", Method: phpobj.NativeMethod(stfoConstruct)},
		},
		H: &phpv.ZClassHandlers{},
	}
}

func getSFOData(o *phpobj.ZObject) *splFileObjectData {
	if d, ok := o.GetOpaque(SplFileObjectClass).(*splFileObjectData); ok {
		return d
	}
	return nil
}

func sfoConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var mode *phpv.ZString
	_, err := core.Expand(ctx, args, &filename, &mode)
	if err != nil {
		return nil, err
	}

	openMode := "r"
	if mode != nil {
		openMode = string(*mode)
	}

	path := string(filename)
	var flag int
	switch {
	case openMode == "r":
		flag = os.O_RDONLY
	case openMode == "w":
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	case openMode == "a":
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	case openMode == "r+":
		flag = os.O_RDWR
	case openMode == "w+":
		flag = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	case openMode == "a+":
		flag = os.O_RDWR | os.O_CREATE | os.O_APPEND
	default:
		flag = os.O_RDONLY
	}

	file, err2 := os.OpenFile(path, flag, 0644)
	if err2 != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			fmt.Sprintf("SplFileObject::__construct(%s): Failed to open stream: %s", path, err2.Error()))
	}

	info, _ := file.Stat()
	data := &splFileObjectData{
		splFileInfoData: splFileInfoData{path: path, info: info},
		file:            file,
		scanner:         bufio.NewScanner(file),
	}
	o.SetOpaque(SplFileInfoClass, &data.splFileInfoData)
	o.SetOpaque(SplFileObjectClass, data)

	// Read first line
	if data.scanner.Scan() {
		data.curLine = data.scanner.Text() + "\n"
	} else {
		data.eof = true
	}

	return nil, nil
}

func stfoConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	file, err := os.CreateTemp("", "spl_temp_*")
	if err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, err.Error())
	}

	info, _ := file.Stat()
	data := &splFileObjectData{
		splFileInfoData: splFileInfoData{path: file.Name(), info: info},
		file:            file,
		scanner:         bufio.NewScanner(file),
	}
	o.SetOpaque(SplFileInfoClass, &data.splFileInfoData)
	o.SetOpaque(SplFileObjectClass, data)
	return nil, nil
}

func sfoFgets(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.eof {
		return phpv.ZBool(false).ZVal(), nil
	}
	line := d.curLine
	if d.scanner.Scan() {
		d.curLine = d.scanner.Text() + "\n"
		d.line++
	} else {
		d.eof = true
		d.curLine = ""
	}
	return phpv.ZStr(line), nil
}

func sfoFgetcsv(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Simplified CSV - just return the line split by comma
	d := getSFOData(o)
	if d == nil || d.eof {
		return phpv.ZBool(false).ZVal(), nil
	}
	// Just return the current line as a single-element array for now
	result := phpv.NewZArray()
	result.OffsetSet(ctx, nil, phpv.ZStr(d.curLine))
	if d.scanner.Scan() {
		d.curLine = d.scanner.Text() + "\n"
		d.line++
	} else {
		d.eof = true
	}
	return result.ZVal(), nil
}

func sfoEof(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	return phpv.ZBool(d == nil || d.eof).ZVal(), nil
}

func sfoFwrite(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	var data phpv.ZString
	_, err := core.Expand(ctx, args, &data)
	if err != nil {
		return nil, err
	}
	n, _ := d.file.Write([]byte(data))
	return phpv.ZInt(n).ZVal(), nil
}

func sfoFflush(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d != nil && d.file != nil {
		d.file.Sync()
	}
	return phpv.ZBool(true).ZVal(), nil
}

func sfoFtell(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	pos, _ := d.file.Seek(0, 1) // current position
	return phpv.ZInt(pos).ZVal(), nil
}

func sfoFseek(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return phpv.ZInt(-1).ZVal(), nil
	}
	var offset phpv.ZInt
	_, err := core.Expand(ctx, args, &offset)
	if err != nil {
		return nil, err
	}
	_, err2 := d.file.Seek(int64(offset), 0)
	if err2 != nil {
		return phpv.ZInt(-1).ZVal(), nil
	}
	d.scanner = bufio.NewScanner(d.file)
	d.eof = false
	return phpv.ZInt(0).ZVal(), nil
}

func sfoRewind(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return nil, nil
	}
	d.file.Seek(0, 0)
	d.scanner = bufio.NewScanner(d.file)
	d.line = 0
	d.eof = false
	if d.scanner.Scan() {
		d.curLine = d.scanner.Text() + "\n"
	} else {
		d.eof = true
	}
	return nil, nil
}

func sfoCurrent(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.eof {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZStr(d.curLine), nil
}

func sfoKey(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.line).ZVal(), nil
}

func sfoNext(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.eof {
		return nil, nil
	}
	if d.scanner.Scan() {
		d.curLine = d.scanner.Text() + "\n"
		d.line++
	} else {
		d.eof = true
		d.curLine = ""
	}
	return nil, nil
}

func sfoValid(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	return phpv.ZBool(d != nil && !d.eof).ZVal(), nil
}

func sfoSetFlags(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
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

func sfoGetFlags(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.flags).ZVal(), nil
}

func sfoToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.eof {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(d.curLine), nil
}
