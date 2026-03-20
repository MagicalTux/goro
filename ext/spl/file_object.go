package spl

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/ext/standard"
)

// SplFileObject flag constants (matching PHP)
const (
	sfoDropNewLine = 1
	sfoReadAhead   = 2
	sfoSkipEmpty   = 4
	sfoReadCsv     = 8
)

type splFileObjectData struct {
	splFileInfoData
	file    *os.File
	scanner *bufio.Scanner
	line    int
	curLine string
	eof     bool
	flags   int

	// CSV control
	csvSep       byte
	csvEnc       byte
	csvEsc       byte
	csvEscapeSet bool // whether escape was explicitly set (for deprecation warning)

	// Max line len (0 = unlimited)
	maxLineLen int

	// Open mode
	openMode string
}

var SplFileObjectClass *phpobj.ZClass
var SplTempFileObjectClass *phpobj.ZClass

func initSplFileObject() {
	// Build methods by first copying all SplFileInfo methods, then adding SplFileObject's own
	sfoMethods := make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range SplFileInfoClass.Methods {
		sfoMethods[k] = v
	}
	// Override/add SplFileObject methods
	sfoOwnMethods := map[phpv.ZString]*phpv.ZClassMethod{
		"__construct":    {Name: "__construct", Method: phpobj.NativeMethod(sfoConstruct)},
		"fgets":          {Name: "fgets", Method: phpobj.NativeMethod(sfoFgets)},
		"fgetc":          {Name: "fgetc", Method: phpobj.NativeMethod(sfoFgetc)},
		"fgetcsv": {Name: "fgetcsv", Method: &phpobj.NativeMethodNamed{
			Fn: sfoFgetcsv,
			Args: []*phpv.FuncArg{
				{VarName: "separator"},
				{VarName: "enclosure"},
				{VarName: "escape"},
			},
		}},
		"fputcsv": {Name: "fputcsv", Method: &phpobj.NativeMethodNamed{
			Fn: sfoFputcsv,
			Args: []*phpv.FuncArg{
				{VarName: "fields"},
				{VarName: "separator"},
				{VarName: "enclosure"},
				{VarName: "escape"},
				{VarName: "eol"},
			},
		}},
		"fpassthru":      {Name: "fpassthru", Method: phpobj.NativeMethod(sfoFpassthru)},
		"fscanf":         {Name: "fscanf", Method: phpobj.NativeMethod(sfoFscanf)},
		"fread":          {Name: "fread", Method: phpobj.NativeMethod(sfoFread)},
		"eof":            {Name: "eof", Method: phpobj.NativeMethod(sfoEof)},
		"fwrite":         {Name: "fwrite", Method: phpobj.NativeMethod(sfoFwrite)},
		"fflush":         {Name: "fflush", Method: phpobj.NativeMethod(sfoFflush)},
		"ftruncate":      {Name: "ftruncate", Method: phpobj.NativeMethod(sfoFtruncate)},
		"fstat":          {Name: "fstat", Method: phpobj.NativeMethod(sfoFstat)},
		"ftell":          {Name: "ftell", Method: phpobj.NativeMethod(sfoFtell)},
		"fseek":          {Name: "fseek", Method: phpobj.NativeMethod(sfoFseek)},
		"flock":          {Name: "flock", Method: phpobj.NativeMethod(sfoFlock)},
		"seek":           {Name: "seek", Method: phpobj.NativeMethod(sfoSeek)},
		"rewind":         {Name: "rewind", Method: phpobj.NativeMethod(sfoRewind)},
		"current":        {Name: "current", Method: phpobj.NativeMethod(sfoCurrent)},
		"getcurrentline": {Name: "getCurrentLine", Method: phpobj.NativeMethod(sfoFgets)},
		"key":            {Name: "key", Method: phpobj.NativeMethod(sfoKey)},
		"next":           {Name: "next", Method: phpobj.NativeMethod(sfoNext)},
		"valid":          {Name: "valid", Method: phpobj.NativeMethod(sfoValid)},
		"setflags":       {Name: "setFlags", Method: phpobj.NativeMethod(sfoSetFlags)},
		"getflags":       {Name: "getFlags", Method: phpobj.NativeMethod(sfoGetFlags)},
		"setcsvcontrol": {Name: "setCsvControl", Method: &phpobj.NativeMethodNamed{
			Fn: sfoSetCsvControl,
			Args: []*phpv.FuncArg{
				{VarName: "separator"},
				{VarName: "enclosure"},
				{VarName: "escape"},
			},
		}},
		"getcsvcontrol":  {Name: "getCsvControl", Method: phpobj.NativeMethod(sfoGetCsvControl)},
		"setmaxlinelen":  {Name: "setMaxLineLen", Method: phpobj.NativeMethod(sfoSetMaxLineLen)},
		"getmaxlinelen":  {Name: "getMaxLineLen", Method: phpobj.NativeMethod(sfoGetMaxLineLen)},
		"haschildren":    {Name: "hasChildren", Method: phpobj.NativeMethod(sfoHasChildren)},
		"getchildren":    {Name: "getChildren", Method: phpobj.NativeMethod(sfoGetChildren)},
		"__tostring":     {Name: "__toString", Method: phpobj.NativeMethod(sfoToString)},
		"__debuginfo":    {Name: "__debugInfo", Method: phpobj.NativeMethod(sfoDebugInfo)},
	}
	for k, v := range sfoOwnMethods {
		sfoMethods[k] = v
	}

	SplFileObjectClass = &phpobj.ZClass{
		Name:            "SplFileObject",
		Extends:         SplFileInfoClass,
		Implementations: []*phpobj.ZClass{},
		Const: map[phpv.ZString]*phpv.ZClassConst{
			"DROP_NEW_LINE": {Value: phpv.ZInt(sfoDropNewLine)},
			"READ_AHEAD":    {Value: phpv.ZInt(sfoReadAhead)},
			"SKIP_EMPTY":    {Value: phpv.ZInt(sfoSkipEmpty)},
			"READ_CSV":      {Value: phpv.ZInt(sfoReadCsv)},
		},
		Methods: sfoMethods,
		H:       &phpv.ZClassHandlers{},
	}

	// SplTempFileObject inherits everything from SplFileObject
	stfoMethods := make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range sfoMethods {
		stfoMethods[k] = v
	}
	stfoMethods["__construct"] = &phpv.ZClassMethod{Name: "__construct", Method: phpobj.NativeMethod(stfoConstruct)}

	SplTempFileObjectClass = &phpobj.ZClass{
		Name:    "SplTempFileObject",
		Extends: SplFileObjectClass,
		Methods: stfoMethods,
		H:       &phpv.ZClassHandlers{},
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

	// Check for null bytes
	if strings.ContainsRune(path, 0) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"SplFileObject::__construct(): Argument #1 ($filename) must not contain any null bytes")
	}

	// Handle stream wrappers
	if strings.HasPrefix(path, "php://temp") || strings.HasPrefix(path, "php://memory") {
		// Create a temp file for php://temp and php://memory
		file, err := os.CreateTemp("", "spl_*")
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
				fmt.Sprintf("SplFileObject::__construct(%s): Failed to open stream: %s", path, err))
		}
		info, _ := file.Stat()
		data := &splFileObjectData{
			splFileInfoData: splFileInfoData{path: path, resolvedPath: file.Name(), info: info},
			file:            file,
			scanner:         bufio.NewScanner(file),
			csvSep:          ',',
			csvEnc:          '"',
			csvEsc:          '\\',
			openMode:        openMode,
		}
		o.SetOpaque(SplFileInfoClass, &data.splFileInfoData)
		o.SetOpaque(SplFileObjectClass, data)
		return nil, nil
	}

	// Resolve relative paths against PHP CWD
	resolvedPath := path
	if !filepath.IsAbs(resolvedPath) && !strings.Contains(path, "://") {
		cwd := string(ctx.Global().Getwd())
		if cwd != "" {
			resolvedPath = filepath.Join(cwd, resolvedPath)
		}
	}

	var flag int
	switch openMode {
	case "r":
		flag = os.O_RDONLY
	case "rb":
		flag = os.O_RDONLY
	case "w":
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	case "wb":
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	case "a":
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	case "ab":
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	case "r+", "r+b", "r+t":
		flag = os.O_RDWR
	case "w+", "w+b", "w+t":
		flag = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	case "a+", "a+b", "a+t":
		flag = os.O_RDWR | os.O_CREATE | os.O_APPEND
	case "x+", "x+b", "x+t":
		flag = os.O_RDWR | os.O_CREATE | os.O_EXCL
	case "x", "xb":
		flag = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	case "c", "cb":
		flag = os.O_WRONLY | os.O_CREATE
	case "c+", "c+b", "c+t":
		flag = os.O_RDWR | os.O_CREATE
	case "wt":
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	case "at":
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	case "rt":
		flag = os.O_RDONLY
	default:
		flag = os.O_RDONLY
	}

	file, err2 := os.OpenFile(resolvedPath, flag, 0644)
	if err2 != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			fmt.Sprintf("SplFileObject::__construct(%s): Failed to open stream: %s", path, err2.Error()))
	}

	info, _ := file.Stat()
	data := &splFileObjectData{
		splFileInfoData: splFileInfoData{path: path, resolvedPath: resolvedPath, info: info},
		file:            file,
		scanner:         bufio.NewScanner(file),
		csvSep:          ',',
		csvEnc:          '"',
		csvEsc:          '\\',
		openMode:        openMode,
	}
	o.SetOpaque(SplFileInfoClass, &data.splFileInfoData)
	o.SetOpaque(SplFileObjectClass, data)

	// Only read first line for readable modes
	isReadable := flag&os.O_RDONLY == os.O_RDONLY || flag&os.O_RDWR != 0
	if isReadable {
		if data.scanner.Scan() {
			data.curLine = data.scanner.Text() + "\n"
		} else {
			data.eof = true
		}
	}

	return nil, nil
}

func stfoConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// SplTempFileObject accepts an optional maxMemory argument
	maxMemory := 2 * 1024 * 1024 // default: 2MB
	if len(args) > 0 && args[0] != nil {
		if args[0].GetType() == phpv.ZtString {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				"SplTempFileObject::__construct(): Argument #1 ($maxMemory) must be of type int, string given")
		}
		maxMemory = int(args[0].AsInt(ctx))
	}

	file, err := os.CreateTemp("", "spl_temp_*")
	if err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, err.Error())
	}

	// Determine the path name based on maxMemory
	var displayPath string
	if maxMemory < 0 {
		displayPath = "php://memory"
	} else if len(args) > 0 && args[0] != nil {
		displayPath = fmt.Sprintf("php://temp/maxmemory:%d", maxMemory)
	} else {
		displayPath = "php://temp"
	}

	info, _ := file.Stat()
	data := &splFileObjectData{
		splFileInfoData: splFileInfoData{path: displayPath, resolvedPath: file.Name(), info: info},
		file:            file,
		scanner:         bufio.NewScanner(file),
		csvSep:          ',',
		csvEnc:          '"',
		csvEsc:          '\\',
		openMode:        "wb",
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

func sfoFgetc(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Read one byte from the file
	buf := make([]byte, 1)
	n, err := d.file.Read(buf)
	if err != nil || n == 0 {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZStr(string(buf[:n])), nil
}

func sfoFgetcsv(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.eof {
		return phpv.ZBool(false).ZVal(), nil
	}

	sep := d.csvSep
	enc := d.csvEnc
	esc := d.csvEsc
	escapeSet := d.csvEscapeSet

	// Parse optional arguments (nil args from named-arg gaps use defaults)
	if len(args) > 0 && args[0] != nil {
		s := args[0].AsString(ctx)
		if len(s) != 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"SplFileObject::fgetcsv(): Argument #1 ($separator) must be a single character")
		}
		sep = s[0]
	}
	if len(args) > 1 && args[1] != nil {
		s := args[1].AsString(ctx)
		if len(s) != 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"SplFileObject::fgetcsv(): Argument #2 ($enclosure) must be a single character")
		}
		enc = s[0]
	}
	if len(args) > 2 && args[2] != nil {
		s := args[2].AsString(ctx)
		if len(s) > 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"SplFileObject::fgetcsv(): Argument #3 ($escape) must be empty or a single character")
		}
		if len(s) == 0 {
			esc = 0
		} else {
			esc = s[0]
		}
		escapeSet = true
	}

	// If escape was never set (not via setCsvControl, not via argument), emit deprecation
	if !escapeSet {
		ctx.Deprecated("SplFileObject::fgetcsv(): the $escape parameter must be provided, as its default value will change, either explicitly or via SplFileObject::setCsvControl()")
	}

	line := d.curLine
	// Strip trailing newline for CSV parsing
	line = strings.TrimRight(line, "\r\n")

	// Advance to next line
	if d.scanner.Scan() {
		d.curLine = d.scanner.Text() + "\n"
		d.line++
	} else {
		d.eof = true
		d.curLine = ""
	}

	return standard.ParseCsvLine(ctx, line, sep, enc, esc)
}

func sfoFputcsv(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Find the fields argument (first non-nil)
	var fields *phpv.ZArray
	if len(args) > 0 && args[0] != nil {
		fields = args[0].AsArray(ctx)
	}
	if fields == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	sep := d.csvSep
	enc := d.csvEnc
	esc := d.csvEsc

	if len(args) > 1 && args[1] != nil {
		s := args[1].AsString(ctx)
		if len(s) != 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"SplFileObject::fputcsv(): Argument #2 ($separator) must be a single character")
		}
		sep = s[0]
	}
	if len(args) > 2 && args[2] != nil {
		s := args[2].AsString(ctx)
		if len(s) != 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"SplFileObject::fputcsv(): Argument #3 ($enclosure) must be a single character")
		}
		enc = s[0]
	}
	if len(args) > 3 && args[3] != nil {
		s := args[3].AsString(ctx)
		if len(s) > 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"SplFileObject::fputcsv(): Argument #4 ($escape) must be empty or a single character")
		}
		if len(s) == 0 {
			esc = 0
		} else {
			esc = s[0]
		}
	}

	lineBytes, err := standard.BuildCsvLine(ctx, fields, sep, enc, esc)
	if err != nil {
		return nil, err
	}

	// Check for eol parameter (5th arg)
	if len(args) > 4 && args[4] != nil {
		eol := string(args[4].AsString(ctx))
		// Replace the trailing \n with the custom eol
		if len(lineBytes) > 0 && lineBytes[len(lineBytes)-1] == '\n' {
			lineBytes = append(lineBytes[:len(lineBytes)-1], []byte(eol)...)
		}
	}

	n, err := d.file.Write(lineBytes)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZInt(n).ZVal(), nil
}

func sfoFpassthru(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	n, err := io.Copy(ctx, d.file)
	if err != nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(n).ZVal(), nil
}

func sfoFscanf(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.eof {
		return phpv.ZBool(false).ZVal(), nil
	}

	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"SplFileObject::fscanf() expects at least 1 argument")
	}

	// Get the current line
	line := d.curLine
	line = strings.TrimRight(line, "\r\n")

	// Advance to next line
	if d.scanner.Scan() {
		d.curLine = d.scanner.Text() + "\n"
		d.line++
	} else {
		d.eof = true
		d.curLine = ""
	}

	format := args[0].AsString(ctx)
	r := strings.NewReader(string(line))
	output, err := core.Zscanf(ctx, r, format, args[1:]...)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func sfoFread(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var length phpv.ZInt
	_, err := core.Expand(ctx, args, &length)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, int(length))
	n, err2 := d.file.Read(buf)
	if err2 != nil && n == 0 {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZStr(string(buf[:n])), nil
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
	var length *phpv.ZInt
	_, err := core.Expand(ctx, args, &data, &length)
	if err != nil {
		return nil, err
	}

	writeData := []byte(data)
	if length != nil && int(*length) < len(writeData) {
		writeData = writeData[:int(*length)]
	}

	n, _ := d.file.Write(writeData)
	return phpv.ZInt(n).ZVal(), nil
}

func sfoFflush(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d != nil && d.file != nil {
		d.file.Sync()
	}
	return phpv.ZBool(true).ZVal(), nil
}

func sfoFtruncate(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	var size phpv.ZInt
	_, err := core.Expand(ctx, args, &size)
	if err != nil {
		return nil, err
	}

	if size < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"SplFileObject::ftruncate(): Argument #1 ($size) must be greater than or equal to 0")
	}

	err2 := d.file.Truncate(int64(size))
	if err2 != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	return phpv.ZBool(true).ZVal(), nil
}

func sfoFstat(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	info, err := d.file.Stat()
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	result := phpv.NewZArray()
	sys := info.Sys()
	var stat *syscall.Stat_t
	if sys != nil {
		stat, _ = sys.(*syscall.Stat_t)
	}

	dev := phpv.ZInt(0)
	ino := phpv.ZInt(0)
	nlink := phpv.ZInt(0)
	uid := phpv.ZInt(0)
	gid := phpv.ZInt(0)
	rdev := phpv.ZInt(0)
	blksize := phpv.ZInt(0)
	blocks := phpv.ZInt(0)
	if stat != nil {
		dev = phpv.ZInt(stat.Dev)
		ino = phpv.ZInt(stat.Ino)
		nlink = phpv.ZInt(stat.Nlink)
		uid = phpv.ZInt(stat.Uid)
		gid = phpv.ZInt(stat.Gid)
		rdev = phpv.ZInt(stat.Rdev)
		blksize = phpv.ZInt(stat.Blksize)
		blocks = phpv.ZInt(stat.Blocks)
	}

	mode := phpv.ZInt(info.Mode())
	size := phpv.ZInt(info.Size())
	mtime := phpv.ZInt(info.ModTime().Unix())

	result.OffsetSet(ctx, phpv.ZInt(0).ZVal(), dev.ZVal())
	result.OffsetSet(ctx, phpv.ZString("dev"), dev.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(1).ZVal(), ino.ZVal())
	result.OffsetSet(ctx, phpv.ZString("ino"), ino.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(2).ZVal(), mode.ZVal())
	result.OffsetSet(ctx, phpv.ZString("mode"), mode.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(3).ZVal(), nlink.ZVal())
	result.OffsetSet(ctx, phpv.ZString("nlink"), nlink.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(4).ZVal(), uid.ZVal())
	result.OffsetSet(ctx, phpv.ZString("uid"), uid.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(5).ZVal(), gid.ZVal())
	result.OffsetSet(ctx, phpv.ZString("gid"), gid.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(6).ZVal(), rdev.ZVal())
	result.OffsetSet(ctx, phpv.ZString("rdev"), rdev.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(7).ZVal(), size.ZVal())
	result.OffsetSet(ctx, phpv.ZString("size"), size.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(8).ZVal(), mtime.ZVal())
	result.OffsetSet(ctx, phpv.ZString("atime"), mtime.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(9).ZVal(), mtime.ZVal())
	result.OffsetSet(ctx, phpv.ZString("mtime"), mtime.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(10).ZVal(), mtime.ZVal())
	result.OffsetSet(ctx, phpv.ZString("ctime"), mtime.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(11).ZVal(), blksize.ZVal())
	result.OffsetSet(ctx, phpv.ZString("blksize"), blksize.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(12).ZVal(), blocks.ZVal())
	result.OffsetSet(ctx, phpv.ZString("blocks"), blocks.ZVal())

	return result.ZVal(), nil
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
	var whence *phpv.ZInt
	_, err := core.Expand(ctx, args, &offset, &whence)
	if err != nil {
		return nil, err
	}

	w := 0 // SEEK_SET
	if whence != nil {
		w = int(*whence)
	}

	_, err2 := d.file.Seek(int64(offset), w)
	if err2 != nil {
		return phpv.ZInt(-1).ZVal(), nil
	}
	d.scanner = bufio.NewScanner(d.file)
	d.eof = false
	return phpv.ZInt(0).ZVal(), nil
}

func sfoFlock(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Stub - flock is not easily portable
	return phpv.ZBool(true).ZVal(), nil
}

func sfoSeek(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return nil, nil
	}

	var lineNum phpv.ZInt
	_, err := core.Expand(ctx, args, &lineNum)
	if err != nil {
		return nil, err
	}

	target := int(lineNum)
	if target < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"SplFileObject::seek(): Argument #1 ($line) must be greater than or equal to 0")
	}

	// Rewind to beginning
	d.file.Seek(0, 0)
	d.scanner = bufio.NewScanner(d.file)
	d.line = 0
	d.eof = false

	if d.scanner.Scan() {
		d.curLine = d.scanner.Text() + "\n"
	} else {
		d.eof = true
		d.curLine = ""
	}

	for d.line < target && !d.eof {
		if d.scanner.Scan() {
			d.curLine = d.scanner.Text() + "\n"
			d.line++
		} else {
			d.eof = true
			d.curLine = ""
		}
	}

	return nil, nil
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
	if d.flags&sfoReadCsv != 0 {
		// In CSV mode, return the parsed CSV array
		line := strings.TrimRight(d.curLine, "\r\n")
		return standard.ParseCsvLine(ctx, line, d.csvSep, d.csvEnc, d.csvEsc)
	}
	line := d.curLine
	if d.flags&sfoDropNewLine != 0 {
		line = strings.TrimRight(line, "\r\n")
	}
	return phpv.ZStr(line), nil
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
	for {
		if d.scanner.Scan() {
			d.curLine = d.scanner.Text() + "\n"
			d.line++
		} else {
			d.eof = true
			d.curLine = ""
			break
		}
		// Handle SKIP_EMPTY flag
		if d.flags&sfoSkipEmpty != 0 {
			line := d.curLine
			if d.flags&sfoDropNewLine != 0 {
				line = strings.TrimRight(line, "\r\n")
			}
			if line == "" || (line == "\n") || (line == "\r\n") {
				continue
			}
		}
		break
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

func sfoSetCsvControl(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil {
		return nil, nil
	}

	// Parse arguments: separator, enclosure, escape
	// nil args indicate named-arg gaps (use default)
	if len(args) > 0 && args[0] != nil {
		s := args[0].AsString(ctx)
		if len(s) != 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"SplFileObject::setCsvControl(): Argument #1 ($separator) must be a single character")
		}
		d.csvSep = s[0]
	}
	if len(args) > 1 && args[1] != nil {
		s := args[1].AsString(ctx)
		if len(s) != 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"SplFileObject::setCsvControl(): Argument #2 ($enclosure) must be a single character")
		}
		d.csvEnc = s[0]
	}
	if len(args) > 2 && args[2] != nil {
		s := args[2].AsString(ctx)
		if len(s) > 1 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
				"SplFileObject::setCsvControl(): Argument #3 ($escape) must be empty or a single character")
		}
		if len(s) == 0 {
			d.csvEsc = 0
		} else {
			d.csvEsc = s[0]
		}
		d.csvEscapeSet = true
	}

	return nil, nil
}

func sfoGetCsvControl(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil {
		result := phpv.NewZArray()
		result.OffsetSet(ctx, nil, phpv.ZStr(","))
		result.OffsetSet(ctx, nil, phpv.ZStr("\""))
		result.OffsetSet(ctx, nil, phpv.ZStr("\\"))
		return result.ZVal(), nil
	}

	result := phpv.NewZArray()
	result.OffsetSet(ctx, nil, phpv.ZStr(string(d.csvSep)))
	result.OffsetSet(ctx, nil, phpv.ZStr(string(d.csvEnc)))
	if d.csvEsc == 0 {
		result.OffsetSet(ctx, nil, phpv.ZStr(""))
	} else {
		result.OffsetSet(ctx, nil, phpv.ZStr(string(d.csvEsc)))
	}
	return result.ZVal(), nil
}

func sfoSetMaxLineLen(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil {
		return nil, nil
	}
	var maxLen phpv.ZInt
	_, err := core.Expand(ctx, args, &maxLen)
	if err != nil {
		return nil, err
	}
	if maxLen < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"SplFileObject::setMaxLineLen(): Argument #1 ($maxLength) must be greater than or equal to 0")
	}
	d.maxLineLen = int(maxLen)
	return nil, nil
}

func sfoGetMaxLineLen(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	return phpv.ZInt(d.maxLineLen).ZVal(), nil
}

func sfoHasChildren(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZBool(false).ZVal(), nil
}

func sfoGetChildren(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZNULL.ZVal(), nil
}

func sfoToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.eof {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(d.curLine), nil
}

func sfoDebugInfo(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	arr := phpv.NewZArray()
	if d != nil {
		arr.OffsetSet(ctx, phpv.ZString("\x00SplFileInfo\x00pathName"), phpv.ZStr(d.path))
		arr.OffsetSet(ctx, phpv.ZString("\x00SplFileInfo\x00fileName"), phpv.ZStr(sfiBaseName(d.path)))
		openMode := d.openMode
		if openMode == "" {
			openMode = "r"
		}
		arr.OffsetSet(ctx, phpv.ZString("\x00SplFileObject\x00openMode"), phpv.ZStr(openMode))
		arr.OffsetSet(ctx, phpv.ZString("\x00SplFileObject\x00delimiter"), phpv.ZStr(string(d.csvSep)))
		arr.OffsetSet(ctx, phpv.ZString("\x00SplFileObject\x00enclosure"), phpv.ZStr(string(d.csvEnc)))
	}
	return arr.ZVal(), nil
}

// Ensure the unused import is consumed
var _ = bytes.NewReader
