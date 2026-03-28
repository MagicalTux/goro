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
	reader  *bufio.Reader
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

	// Whether the first line has been read (lazy initialization)
	firstLineRead      bool
	emptyFileFirstLine bool // empty file: first line is "" but not eof yet
	isReadable         bool
	isWritable         bool
}

// readLine reads the next line from the file using the bufio.Reader.
// Unlike Scanner.Text() + "\n", this preserves the actual line ending
// (or lack thereof for the last line of a file).
func (d *splFileObjectData) readLine() (string, bool) {
	if d.reader == nil {
		d.reader = bufio.NewReader(d.file)
	}
	line, err := d.reader.ReadString('\n')
	if err != nil {
		if len(line) > 0 {
			// Got partial line at EOF
			return line, true
		}
		return "", false
	}
	return line, true
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
		"__clone": {Name: "__clone", Modifiers: phpv.ZAttrPrivate, Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Trying to clone an uncloneable object of class SplFileObject")
		})},
	}
	for k, v := range sfoOwnMethods {
		sfoMethods[k] = v
	}

	SplFileObjectClass = &phpobj.ZClass{
		Name:            "SplFileObject",
		Extends:         SplFileInfoClass,
		Implementations: []*phpobj.ZClass{RecursiveIterator, SeekableIterator},
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

// applyMaxLineLen truncates a line if maxLineLen is set (>0).
// PHP's maxLineLen limits the total number of bytes read per line (including newline).
func applyMaxLineLen(line string, maxLineLen int) string {
	if maxLineLen > 0 && len(line) > maxLineLen {
		return line[:maxLineLen]
	}
	return line
}

// ensureFirstLineRead performs the deferred first-line read for SplFileObject.
// This must be called before any operation that depends on curLine (iteration,
// current(), fgets, etc.), but NOT before raw file operations (fread, fpassthru, etc.).
func ensureFirstLineRead(d *splFileObjectData) {
	if d == nil || d.firstLineRead || !d.isReadable {
		return
	}
	d.firstLineRead = true
	line, ok := d.readLine()
	if ok {
		d.curLine = applyMaxLineLen(line, d.maxLineLen)
	} else {
		// If the file is at position 0 and truly empty, treat it as having
		// one empty line (PHP behavior: empty files produce one empty-line read)
		pos, _ := d.file.Seek(0, 1) // get current position
		if pos == 0 {
			d.curLine = ""
			// Don't set eof yet; the first read of the empty line will consume it,
			// then subsequent reads will find eof.
			// BUT we do need to flag that the next read should set eof.
			d.emptyFileFirstLine = true
		} else {
			d.eof = true
		}
	}
}

func getSFOData(o *phpobj.ZObject) *splFileObjectData {
	if d, ok := o.GetOpaque(SplFileObjectClass).(*splFileObjectData); ok {
		return d
	}
	return nil
}

func sfoConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Validate argument count first (before double construction check)
	if len(args) == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"SplFileObject::__construct() expects at least 1 argument, 0 given")
	}

	// Prevent double construction
	if getSFOData(o) != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot call constructor twice")
	}

	// Validate mode type before Expand (which would silently convert)
	if len(args) > 1 && args[1] != nil && args[1].GetType() == phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"SplFileObject::__construct(): Argument #2 ($mode) must be of type string, array given")
	}

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
		// Determine readability/writability from mode
		isWritable := openMode != "r" && openMode != "rb" && openMode != "rt"
		data := &splFileObjectData{
			splFileInfoData: splFileInfoData{path: path, resolvedPath: file.Name(), info: info},
			file:            file,
			scanner:         bufio.NewScanner(file),
			csvSep:          ',',
			csvEnc:          '"',
			csvEsc:          '\\',
			openMode:        openMode,
			isReadable:      true, // php://temp and php://memory are always readable
			isWritable:      isWritable,
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

	// Determine if file is readable/writable (for lazy first-line read and write checks).
	// Note: O_RDONLY is 0, so we cannot use flag&O_RDONLY==O_RDONLY (always true).
	// Instead, check that the access mode is not write-only.
	accessMode := flag & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR)
	data.isReadable = accessMode != os.O_WRONLY
	data.isWritable = accessMode != os.O_RDONLY

	// Don't pre-read the first line here. Reading is deferred to the first
	// access via current()/iteration/fgets to avoid advancing the file position
	// before fread/fpassthru/ftell can use it.

	return nil, nil
}

func stfoConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Prevent double construction
	if getSFOData(o) != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot call constructor twice")
	}

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
		isReadable:      true, // temp files are always read-write
		isWritable:      true,
	}
	o.SetOpaque(SplFileInfoClass, &data.splFileInfoData)
	o.SetOpaque(SplFileObjectClass, data)
	return nil, nil
}

func sfoFgets(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	ensureFirstLineRead(d)
	if d.eof {
		return phpv.ZBool(false).ZVal(), nil
	}
	line := d.curLine
	if d.emptyFileFirstLine {
		d.emptyFileFirstLine = false
		d.eof = true
		d.curLine = ""
	} else {
		nextLine, ok := d.readLine()
		if ok {
			d.curLine = applyMaxLineLen(nextLine, d.maxLineLen)
			d.line++
		} else {
			d.eof = true
			d.curLine = ""
		}
	}
	return phpv.ZStr(line), nil
}

func sfoFgetc(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	// fgetc reads a single byte directly from the file at the current raw position.
	// It does NOT use the buffered reader (which may have read ahead for line-based ops).
	// We need to seek to the actual position first if the buffered reader was used.
	if !d.firstLineRead {
		// First access: file is at position 0, read directly
		d.firstLineRead = true
	}

	// Read one byte directly from the file
	buf := make([]byte, 1)
	n, err := d.file.Read(buf)
	if err != nil || n == 0 {
		d.eof = true
		return phpv.ZBool(false).ZVal(), nil
	}

	// Track line numbers: increment after reading a newline
	if buf[0] == '\n' {
		d.line++
	}

	// Invalidate the buffered reader since we've changed the file position
	d.reader = nil

	return phpv.ZStr(string(buf[:n])), nil
}

func sfoFgetcsv(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	ensureFirstLineRead(d)
	if d.eof {
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
		ctx.Deprecated("the $escape parameter must be provided, as its default value will change, either explicitly or via SplFileObject::setCsvControl()")
	}

	line := d.curLine
	// Strip trailing newline for CSV parsing
	line = strings.TrimRight(line, "\r\n")

	// Advance to next line
	if d.emptyFileFirstLine {
		d.emptyFileFirstLine = false
		d.eof = true
		d.curLine = ""
	} else {
		nextLine, ok := d.readLine()
		if ok {
			d.curLine = applyMaxLineLen(nextLine, d.maxLineLen)
			d.line++
		} else {
			d.eof = true
			d.curLine = ""
		}
	}

	// If SKIP_EMPTY is set and the line is empty, return false
	if d.flags&sfoSkipEmpty != 0 && line == "" {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Empty line returns array(NULL) per PHP behavior
	if line == "" {
		result := phpv.NewZArray()
		result.OffsetSet(ctx, nil, phpv.ZNULL.ZVal())
		return result.ZVal(), nil
	}

	return standard.ParseCsvLine(ctx, line, sep, enc, esc)
}

func sfoFputcsv(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	if !d.isWritable {
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

	eol := "\n"
	// Check for eol parameter (5th arg)
	if len(args) > 4 && args[4] != nil {
		eol = string(args[4].AsString(ctx))
	}

	lineBytes, err := standard.BuildCsvLine(ctx, fields, sep, enc, esc, eol)
	if err != nil {
		return nil, err
	}

	n, err := d.file.Write(lineBytes)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	// After writing, eof is no longer true (position is valid, we just wrote data)
	d.eof = false

	// Invalidate scanner/reader after write - recreate at current position
	d.scanner = bufio.NewScanner(d.file)
	d.reader = bufio.NewReader(d.file)

	return phpv.ZInt(n).ZVal(), nil
}

func sfoFpassthru(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil || d.file == nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	// fpassthru reads directly from the file - don't call ensureFirstLineRead
	// so the file position stays at where it was last left.
	n, err := io.Copy(ctx, d.file)
	if err != nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	d.eof = true
	return phpv.ZInt(n).ZVal(), nil
}

func sfoFscanf(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	ensureFirstLineRead(d)
	if d.eof {
		return phpv.ZBool(false).ZVal(), nil
	}

	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"SplFileObject::fscanf() expects at least 1 argument")
	}

	// Get the current line (keep the trailing newline for PHP compatibility,
	// as PHP's fscanf passes the line including \n to the scanf logic)
	line := d.curLine

	// Advance to next line
	nextLine, ok := d.readLine()
	if ok {
		d.curLine = nextLine
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

	if length <= 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"SplFileObject::fread(): Argument #1 ($length) must be greater than 0")
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
	// After writing, eof is no longer true
	d.eof = false
	// Invalidate scanner/reader after write
	d.scanner = bufio.NewScanner(d.file)
	d.reader = bufio.NewReader(d.file)
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

	// PHP's fstat returns numeric keys 0-12 first, then string keys
	result.OffsetSet(ctx, phpv.ZInt(0).ZVal(), dev.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(1).ZVal(), ino.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(2).ZVal(), mode.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(3).ZVal(), nlink.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(4).ZVal(), uid.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(5).ZVal(), gid.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(6).ZVal(), rdev.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(7).ZVal(), size.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(8).ZVal(), mtime.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(9).ZVal(), mtime.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(10).ZVal(), mtime.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(11).ZVal(), blksize.ZVal())
	result.OffsetSet(ctx, phpv.ZInt(12).ZVal(), blocks.ZVal())
	result.OffsetSet(ctx, phpv.ZString("dev"), dev.ZVal())
	result.OffsetSet(ctx, phpv.ZString("ino"), ino.ZVal())
	result.OffsetSet(ctx, phpv.ZString("mode"), mode.ZVal())
	result.OffsetSet(ctx, phpv.ZString("nlink"), nlink.ZVal())
	result.OffsetSet(ctx, phpv.ZString("uid"), uid.ZVal())
	result.OffsetSet(ctx, phpv.ZString("gid"), gid.ZVal())
	result.OffsetSet(ctx, phpv.ZString("rdev"), rdev.ZVal())
	result.OffsetSet(ctx, phpv.ZString("size"), size.ZVal())
	result.OffsetSet(ctx, phpv.ZString("atime"), mtime.ZVal())
	result.OffsetSet(ctx, phpv.ZString("mtime"), mtime.ZVal())
	result.OffsetSet(ctx, phpv.ZString("ctime"), mtime.ZVal())
	result.OffsetSet(ctx, phpv.ZString("blksize"), blksize.ZVal())
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
	d.reader = bufio.NewReader(d.file)
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
	d.reader = bufio.NewReader(d.file)
	d.line = 0
	d.eof = false
	d.emptyFileFirstLine = false
	d.firstLineRead = true
	d.curLine = ""

	// PHP's seek loop: for (i = 0; i < line_pos; i++) { read_line(); }
	// The first read has line_add=0 (doesn't increment), subsequent reads
	// increment. PHP streams allow one empty read past the last real line
	// before truly hitting EOF. After the loop completes normally, there's
	// an additional line_num++ (post-loop increment).
	earlyExit := false
	for i := 0; i < target; i++ {
		if d.eof {
			// Stream is truly at EOF (second failed read)
			earlyExit = true
			break
		}
		line, ok := d.readLine()
		if ok {
			if i > 0 {
				d.line++
			}
			d.curLine = line
		} else {
			// First failed read: PHP treats this as a successful empty read
			// (stream eof not set until NEXT read attempt)
			if i > 0 {
				d.line++
			}
			d.curLine = ""
			d.eof = true
			// Don't break - PHP's loop continues since this read "succeeded"
		}
	}

	// Post-loop increment: PHP increments line_num after the seek loop
	// completes normally (not on early exit due to eof)
	if !earlyExit && target > 0 {
		d.line++
	}

	// After seek, read the current line for subsequent current() calls
	if !d.eof {
		line, ok := d.readLine()
		if ok {
			d.curLine = line
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
	d.reader = bufio.NewReader(d.file)
	d.scanner = bufio.NewScanner(d.file)
	d.line = 0
	d.eof = false
	d.emptyFileFirstLine = false
	d.firstLineRead = false
	ensureFirstLineRead(d)
	return nil, nil
}

func sfoCurrent(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSFOData(o)
	if d == nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	ensureFirstLineRead(d)
	if d.eof {
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
	if d == nil {
		return nil, nil
	}
	ensureFirstLineRead(d)
	if d.eof {
		// Even at EOF, advance the line counter (PHP behavior)
		d.line++
		return nil, nil
	}
	if d.emptyFileFirstLine {
		d.emptyFileFirstLine = false
		d.eof = true
		d.curLine = ""
		return nil, nil
	}
	for {
		nextLine, ok := d.readLine()
		if ok {
			d.curLine = applyMaxLineLen(nextLine, d.maxLineLen)
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
	if d != nil {
		ensureFirstLineRead(d)
	}
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
	} else {
		// If escape was not provided, emit deprecation
		ctx.Deprecated("the $escape parameter must be provided as its default value will change")
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
	if d == nil {
		return phpv.ZStr(""), nil
	}
	ensureFirstLineRead(d)
	if d.eof {
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
