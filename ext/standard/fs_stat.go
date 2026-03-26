package standard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// phpErrMsg extracts just the human-readable error message from a Go error,
// stripping the syscall name and path information that Go includes.
// For example, converts "symlink /path/a /path/b: file exists" to "File exists"
// and "readlink /path/a: no such file or directory" to "No such file or directory".
func phpErrMsg(err error) string {
	// Try to extract the underlying syscall.Errno or PathError
	var pathErr *os.PathError
	var linkErr *os.LinkError
	var errno syscall.Errno

	if errors.As(err, &linkErr) {
		errno, _ = linkErr.Err.(syscall.Errno)
	} else if errors.As(err, &pathErr) {
		errno, _ = pathErr.Err.(syscall.Errno)
	} else if errors.As(err, &errno) {
		// direct errno
	} else {
		return err.Error()
	}

	// Map common errno values to PHP-style capitalized messages
	switch errno {
	case syscall.ENOENT:
		return "No such file or directory"
	case syscall.EEXIST:
		return "File exists"
	case syscall.EACCES:
		return "Permission denied"
	case syscall.ENOTDIR:
		return "Not a directory"
	case syscall.EISDIR:
		return "Is a directory"
	case syscall.ENOTEMPTY:
		return "Directory not empty"
	case syscall.EPERM:
		return "Operation not permitted"
	case syscall.EINVAL:
		return "Invalid argument"
	case syscall.ENAMETOOLONG:
		return "File name too long"
	case syscall.ELOOP:
		return "Too many levels of symbolic links"
	case syscall.ENOSPC:
		return "No space left on device"
	case syscall.EROFS:
		return "Read-only file system"
	case syscall.EXDEV:
		return "Invalid cross-device link"
	case syscall.EMLINK:
		return "Too many links"
	default:
		return errno.Error()
	}
}

// resolveFilePath resolves a filename relative to the cwd from the global context.
func resolveFilePath(ctx phpv.Context, filename string) string {
	if !filepath.IsAbs(filename) {
		return filepath.Join(string(ctx.Global().Getwd()), filename)
	}
	return filename
}

// checkStatFilename validates a filename for stat-family functions.
// Returns true if the filename is valid, false if it should return false early.
// When false is returned, appropriate warnings have already been emitted.
func checkStatFilename(ctx phpv.Context, filename string, funcName string) bool {
	if filename == "" {
		return false
	}
	if strings.ContainsRune(filename, 0) {
		ctx.Warn("%s(): Filename contains null byte", funcName, logopt.NoFuncName(true))
		return false
	}
	return true
}

// > func array stat ( string $filename )
func fncStat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if !checkStatFilename(ctx, filename, "stat") {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "stat"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, filename)
	fi, err := os.Stat(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("stat failed for %s", filename)
	}

	return buildStatArray(ctx, fi), nil
}

// > func array lstat ( string $filename )
func fncLstat(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if !checkStatFilename(ctx, filename, "lstat") {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "lstat"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, filename)
	fi, err := os.Lstat(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("Lstat failed for %s", filename)
	}

	return buildStatArray(ctx, fi), nil
}

// > func int fileatime ( string $filename )
func fncFileatime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if !checkStatFilename(ctx, filename, "fileatime") {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "fileatime"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, filename)
	fi, err := os.Stat(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("stat failed for %s", filename)
	}

	st := fi.Sys().(*syscall.Stat_t)
	return phpv.ZInt(st.Atim.Sec).ZVal(), nil
}

// > func int filectime ( string $filename )
func fncFilectime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if !checkStatFilename(ctx, filename, "filectime") {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "filectime"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, filename)
	fi, err := os.Stat(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("stat failed for %s", filename)
	}

	st := fi.Sys().(*syscall.Stat_t)
	return phpv.ZInt(st.Ctim.Sec).ZVal(), nil
}

// > func int filemtime ( string $filename )
func fncFilemtime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if !checkStatFilename(ctx, filename, "filemtime") {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "filemtime"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, filename)
	fi, err := os.Stat(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("stat failed for %s", filename)
	}

	return phpv.ZInt(fi.ModTime().Unix()).ZVal(), nil
}

// > func int filesize ( string $filename )
func fncFilesize(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if !checkStatFilename(ctx, filename, "filesize") {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "filesize"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, filename)
	fi, err := os.Stat(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("filesize(): stat failed for %s", filename, logopt.NoFuncName(true))
	}

	return phpv.ZInt(fi.Size()).ZVal(), nil
}

// > func string filetype ( string $filename )
func fncFiletype(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if !checkStatFilename(ctx, filename, "filetype") {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "filetype"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, filename)
	fi, err := os.Lstat(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("Lstat failed for %s", filename)
	}

	mode := fi.Mode()
	switch {
	case mode.IsRegular():
		return phpv.ZStr("file"), nil
	case mode.IsDir():
		return phpv.ZStr("dir"), nil
	case mode&os.ModeSymlink != 0:
		return phpv.ZStr("link"), nil
	case mode&os.ModeCharDevice != 0:
		return phpv.ZStr("char"), nil
	case mode&os.ModeDevice != 0:
		return phpv.ZStr("block"), nil
	case mode&os.ModeNamedPipe != 0:
		return phpv.ZStr("fifo"), nil
	case mode&os.ModeSocket != 0:
		return phpv.ZStr("socket"), nil
	default:
		return phpv.ZStr("unknown"), nil
	}
}

// > func int fileperms ( string $filename )
func fncFileperms(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if !checkStatFilename(ctx, filename, "fileperms") {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "fileperms"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, filename)
	fi, err := os.Stat(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("stat failed for %s", filename)
	}

	st := fi.Sys().(*syscall.Stat_t)
	return phpv.ZInt(st.Mode).ZVal(), nil
}

// > func int fileowner ( string $filename )
func fncFileowner(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if !checkStatFilename(ctx, filename, "fileowner") {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "fileowner"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, filename)
	fi, err := os.Stat(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("stat failed for %s", filename)
	}

	st := fi.Sys().(*syscall.Stat_t)
	return phpv.ZInt(st.Uid).ZVal(), nil
}

// > func int filegroup ( string $filename )
func fncFilegroup(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if !checkStatFilename(ctx, filename, "filegroup") {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "filegroup"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, filename)
	fi, err := os.Stat(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("stat failed for %s", filename)
	}

	st := fi.Sys().(*syscall.Stat_t)
	return phpv.ZInt(st.Gid).ZVal(), nil
}

// > func int fileinode ( string $filename )
func fncFileinode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}

	if !checkStatFilename(ctx, filename, "fileinode") {
		return phpv.ZFalse.ZVal(), nil
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "fileinode"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, filename)
	fi, err := os.Stat(p)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("stat failed for %s", filename)
	}

	st := fi.Sys().(*syscall.Stat_t)
	return phpv.ZInt(int64(st.Ino)).ZVal(), nil
}

// > func bool touch ( string $filename [, int $time = time() [, int $atime ]] )
func fncTouch(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename string
	var mtime *phpv.ZInt
	var atime *phpv.ZInt
	_, err := core.Expand(ctx, args, &filename, &mtime, &atime)
	if err != nil {
		return nil, err
	}

	// PHP: touch("") and touch(false) return false
	if filename == "" {
		return phpv.ZFalse.ZVal(), nil
	}

	// PHP 8.x: if mtime is null but atime is provided, throw ValueError
	// Check raw args to distinguish "not passed" from "passed as null"
	if len(args) >= 3 && args[2] != nil && args[2].GetType() != phpv.ZtNull {
		// atime was provided
		if len(args) >= 2 && args[1] != nil && args[1].GetType() == phpv.ZtNull {
			// mtime was explicitly null
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "touch(): Argument #2 ($mtime) cannot be null when argument #3 ($atime) is an integer")
		}
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, filename, "touch"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, filename)

	// Create file if it doesn't exist
	if _, err := os.Stat(p); os.IsNotExist(err) {
		f, err := os.Create(p)
		if err != nil {
			return phpv.ZFalse.ZVal(), ctx.Warn("touch(): Unable to create file %s because %s", filename, phpErrMsg(err), logopt.NoFuncName(true))
		}
		f.Close()
	}

	// If no times specified, use current time
	if mtime == nil && atime == nil {
		now := time.Now()
		err = os.Chtimes(p, now, now)
	} else {
		mt := time.Now()
		at := mt
		if mtime != nil {
			mt = time.Unix(int64(*mtime), 0)
		}
		if atime != nil {
			at = time.Unix(int64(*atime), 0)
		} else if mtime != nil {
			at = mt
		}
		err = os.Chtimes(p, at, mt)
	}

	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("%s", err)
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func string tempnam ( string $dir , string $prefix )
func fncTempnam(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dir, prefix string
	_, err := core.Expand(ctx, args, &dir, &prefix)
	if err != nil {
		return nil, err
	}

	// Check for null bytes in prefix
	if strings.ContainsRune(prefix, 0) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "tempnam(): Argument #2 ($prefix) must not contain any null bytes")
	}

	// PHP uses only the basename of the prefix (strips directory component)
	if strings.ContainsRune(prefix, '/') {
		idx := strings.LastIndexByte(prefix, '/')
		prefix = prefix[idx+1:]
	}

	// PHP truncates prefix to 63 characters
	if len(prefix) > 63 {
		prefix = prefix[:63]
	}

	// Empty dir means system temp directory
	if dir == "" {
		dir = os.TempDir()
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, dir, "tempnam"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, dir)

	// Check if directory exists, fall back to system temp dir if not
	if st, err := os.Stat(p); err != nil || !st.IsDir() {
		ctx.Notice("tempnam(): file created in the system's temporary directory", logopt.NoFuncName(true))
		p = os.TempDir()
	}

	f, err := os.CreateTemp(p, prefix)
	if err != nil {
		// Fall back to system temp dir
		ctx.Notice("tempnam(): file created in the system's temporary directory", logopt.NoFuncName(true))
		f, err = os.CreateTemp(os.TempDir(), prefix)
		if err != nil {
			return phpv.ZFalse.ZVal(), ctx.Warn("%s", err)
		}
	}
	name := f.Name()
	f.Close()

	// Set file permissions to 0600 (PHP's default for tempnam)
	os.Chmod(name, 0600)

	return phpv.ZString(name).ZVal(), nil
}

// > func bool link ( string $target , string $link )
func fncLink(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var target, link string
	_, err := core.Expand(ctx, args, &target, &link)
	if err != nil {
		return nil, err
	}

	// PHP: empty target or link should fail with "No such file or directory"
	if target == "" {
		return phpv.ZFalse.ZVal(), ctx.Warn("%s(): No such file or directory", ctx.GetFuncName(), logopt.NoFuncName(true))
	}
	if link == "" {
		return phpv.ZFalse.ZVal(), ctx.Warn("%s(): No such file or directory", ctx.GetFuncName(), logopt.NoFuncName(true))
	}

	// link() resolves paths before basedir check (PHP shows absolute paths in warnings)
	// Check link (dest) first, then target (source), matching PHP's order
	target = resolveFilePath(ctx, target)
	link = resolveFilePath(ctx, link)

	if err := ctx.Global().CheckOpenBasedir(ctx, link, "link"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	if err := ctx.Global().CheckOpenBasedir(ctx, target, "link"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	err = os.Link(target, link)
	if err != nil {
		return phpv.ZFalse.ZVal(), ctx.Warn("%s(): %s", ctx.GetFuncName(), phpErrMsg(err), logopt.NoFuncName(true))
	}
	return phpv.ZTrue.ZVal(), nil
}

// > func array file ( string $filename [, int $flags = 0 [, resource $context ]] )
func fncFile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var flags *phpv.ZInt
	_, err := core.Expand(ctx, args, &filename, &flags)
	if err != nil {
		return nil, err
	}

	// Validate flags: only FILE_USE_INCLUDE_PATH, FILE_IGNORE_NEW_LINES, FILE_SKIP_EMPTY_LINES, FILE_NO_DEFAULT_CONTEXT are valid
	validFlags := FILE_USE_INCLUDE_PATH | FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES | FILE_NO_DEFAULT_CONTEXT
	if flags != nil && (*flags & ^validFlags) != 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "file(): Argument #2 ($flags) must be a valid flag value")
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, string(filename), "file"); err != nil {
		ctx.Warn("file(%s): Failed to open stream: Operation not permitted", filename, logopt.NoFuncName(true))
		return phpv.ZFalse.ZVal(), nil
	}

	useIncludePath := flags != nil && *flags&FILE_USE_INCLUDE_PATH != 0
	f, err := ctx.Global().Open(ctx, filename, "r", useIncludePath)
	if err != nil {
		errMsg := err.Error()
		if os.IsNotExist(err) {
			errMsg = "No such file or directory"
		}
		return phpv.ZFalse.ZVal(), ctx.Warn("file(%s): Failed to open stream: %s", filename, errMsg, logopt.NoFuncName(true))
	}
	defer f.Close()

	result := phpv.NewZArray()

	ignoreNewLines := flags != nil && *flags&FILE_IGNORE_NEW_LINES != 0
	skipEmpty := flags != nil && *flags&FILE_SKIP_EMPTY_LINES != 0

	var buf []byte
	b := make([]byte, 1)
	for {
		n, readErr := f.Read(b)
		if n > 0 {
			buf = append(buf, b[0])
			if b[0] == '\n' {
				line := string(buf)
				if ignoreNewLines {
					// Strip trailing \n and also \r before it
					line = strings.TrimRight(line, "\r\n")
				}
				if skipEmpty && line == "" {
					// Skip lines that are empty after processing
					buf = buf[:0]
					continue
				}
				result.OffsetSet(ctx, nil, phpv.ZString(line).ZVal())
				buf = buf[:0]
			}
		}
		if readErr != nil {
			break
		}
	}

	// Handle last line without trailing newline
	if len(buf) > 0 {
		line := string(buf)
		if ignoreNewLines {
			line = strings.TrimRight(line, "\r\n")
		}
		if !(skipEmpty && line == "") {
			result.OffsetSet(ctx, nil, phpv.ZString(line).ZVal())
		}
	}

	return result.ZVal(), nil
}

// > func float disk_free_space ( string $directory )
// > alias diskfreespace
func fncDiskFreeSpace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dir string
	_, err := core.Expand(ctx, args, &dir)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, dir, "disk_free_space"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, dir)
	var stat syscall.Statfs_t
	err = syscall.Statfs(p, &stat)
	if err != nil {
		if os.IsNotExist(err) {
			return phpv.ZFalse.ZVal(), ctx.Warn("No such file or directory")
		}
		return phpv.ZFalse.ZVal(), ctx.Warn("%s", err)
	}

	return phpv.ZFloat(float64(stat.Bavail) * float64(stat.Bsize)).ZVal(), nil
}

// > func float disk_total_space ( string $directory )
func fncDiskTotalSpace(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dir string
	_, err := core.Expand(ctx, args, &dir)
	if err != nil {
		return nil, err
	}

	if err := ctx.Global().CheckOpenBasedir(ctx, dir, "disk_total_space"); err != nil {
		return phpv.ZFalse.ZVal(), nil
	}

	p := resolveFilePath(ctx, dir)
	var stat syscall.Statfs_t
	err = syscall.Statfs(p, &stat)
	if err != nil {
		if os.IsNotExist(err) {
			return phpv.ZFalse.ZVal(), ctx.Warn("No such file or directory")
		}
		return phpv.ZFalse.ZVal(), ctx.Warn("%s", err)
	}

	return phpv.ZFloat(float64(stat.Blocks) * float64(stat.Bsize)).ZVal(), nil
}

// buildStatArray creates a PHP stat() result array from os.FileInfo.
func buildStatArray(ctx phpv.Context, fi os.FileInfo) *phpv.ZVal {
	st := fi.Sys().(*syscall.Stat_t)

	result := phpv.NewZArray()

	// Numeric indices 0-12
	result.OffsetSet(ctx, phpv.ZInt(0).ZVal(), phpv.ZInt(int64(st.Dev)).ZVal())      // dev
	result.OffsetSet(ctx, phpv.ZInt(1).ZVal(), phpv.ZInt(int64(st.Ino)).ZVal())      // ino
	result.OffsetSet(ctx, phpv.ZInt(2).ZVal(), phpv.ZInt(int64(st.Mode)).ZVal())     // mode
	result.OffsetSet(ctx, phpv.ZInt(3).ZVal(), phpv.ZInt(int64(st.Nlink)).ZVal())    // nlink
	result.OffsetSet(ctx, phpv.ZInt(4).ZVal(), phpv.ZInt(int64(st.Uid)).ZVal())      // uid
	result.OffsetSet(ctx, phpv.ZInt(5).ZVal(), phpv.ZInt(int64(st.Gid)).ZVal())      // gid
	result.OffsetSet(ctx, phpv.ZInt(6).ZVal(), phpv.ZInt(int64(st.Rdev)).ZVal())     // rdev
	result.OffsetSet(ctx, phpv.ZInt(7).ZVal(), phpv.ZInt(st.Size).ZVal())            // size
	result.OffsetSet(ctx, phpv.ZInt(8).ZVal(), phpv.ZInt(st.Atim.Sec).ZVal())        // atime
	result.OffsetSet(ctx, phpv.ZInt(9).ZVal(), phpv.ZInt(st.Mtim.Sec).ZVal())        // mtime
	result.OffsetSet(ctx, phpv.ZInt(10).ZVal(), phpv.ZInt(st.Ctim.Sec).ZVal())       // ctime
	result.OffsetSet(ctx, phpv.ZInt(11).ZVal(), phpv.ZInt(int64(st.Blksize)).ZVal()) // blksize
	result.OffsetSet(ctx, phpv.ZInt(12).ZVal(), phpv.ZInt(st.Blocks).ZVal())         // blocks

	// Named indices (same data, different keys)
	result.OffsetSet(ctx, phpv.ZStr("dev"), phpv.ZInt(int64(st.Dev)).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("ino"), phpv.ZInt(int64(st.Ino)).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("mode"), phpv.ZInt(int64(st.Mode)).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("nlink"), phpv.ZInt(int64(st.Nlink)).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("uid"), phpv.ZInt(int64(st.Uid)).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("gid"), phpv.ZInt(int64(st.Gid)).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("rdev"), phpv.ZInt(int64(st.Rdev)).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("size"), phpv.ZInt(st.Size).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("atime"), phpv.ZInt(st.Atim.Sec).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("mtime"), phpv.ZInt(st.Mtim.Sec).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("ctime"), phpv.ZInt(st.Ctim.Sec).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("blksize"), phpv.ZInt(int64(st.Blksize)).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("blocks"), phpv.ZInt(st.Blocks).ZVal())

	return result.ZVal()
}
