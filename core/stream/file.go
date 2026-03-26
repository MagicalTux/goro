package stream

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"syscall"

	"github.com/MagicalTux/goro/core/phpv"
)

// appendFile wraps an *os.File opened in append mode.
// PHP's append mode writes always go to the end of file, but the reported
// position (ftell) tracks from 0 and advances by the number of bytes written.
// This differs from OS O_APPEND which reports position at end-of-file after writes.
type appendFile struct {
	f   *os.File
	pos int64 // virtual position for ftell
}

func (a *appendFile) Read(p []byte) (int, error) {
	// Seek to the virtual position before reading
	a.f.Seek(a.pos, io.SeekStart)
	n, err := a.f.Read(p)
	a.pos += int64(n)
	return n, err
}

func (a *appendFile) Write(p []byte) (int, error) {
	// Seek to end for the actual write
	a.f.Seek(0, io.SeekEnd)
	n, err := a.f.Write(p)
	// Advance virtual position by bytes written
	a.pos += int64(n)
	return n, err
}

func (a *appendFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		if offset < 0 {
			return a.pos, ErrNotSupported
		}
		a.pos = offset
	case io.SeekCurrent:
		newPos := a.pos + offset
		if newPos < 0 {
			return a.pos, ErrNotSupported
		}
		a.pos = newPos
	case io.SeekEnd:
		// Get actual file size
		info, err := a.f.Stat()
		if err != nil {
			return a.pos, err
		}
		newPos := info.Size() + offset
		if newPos < 0 {
			return a.pos, ErrNotSupported
		}
		a.pos = newPos
	default:
		return a.pos, ErrNotSupported
	}
	return a.pos, nil
}

func (a *appendFile) Close() error {
	return a.f.Close()
}

func (a *appendFile) Stat() (os.FileInfo, error) {
	return a.f.Stat()
}

func (a *appendFile) Truncate(size int64) error {
	return a.f.Truncate(size)
}

func (a *appendFile) Flush() error {
	return a.f.Sync()
}

func (a *appendFile) Sync() error {
	return a.f.Sync()
}

// TODO: remove cwd state here
type FileHandler struct {
	Cwd  string
	Root string
}

func NewFileHandler(root string) (*FileHandler, error) {
	// make sure root is absolute
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}

	if root[len(root)-1] != filepath.Separator {
		root = root + string([]byte{filepath.Separator})
	}

	fh := &FileHandler{
		Root: root,
		Cwd:  "/",
	}

	// try to get current working directory if within root
	wd, err := os.Getwd()
	if err != nil {
		return fh, nil // ignore error
	}

	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		return fh, nil // ignore error
	}

	if len(wd) >= len(root) && wd[:len(root)] == root {
		localwd := filepath.Join("/", filepath.ToSlash(wd[len(root):]))
		localwd = filepath.Clean(localwd)
		fh.Cwd = localwd
	}

	return fh, nil
}

func (f *FileHandler) localPath(name string) (string, string, error) {
	if !path.IsAbs(name) {
		name = path.Join(f.Cwd, name)
	}
	name = path.Clean(name)

	// go to fname
	fname := filepath.Join(f.Root, filepath.FromSlash(name))

	// resolve symlinks
	fname2, err := filepath.EvalSymlinks(fname)
	if err != nil {
		if !os.IsNotExist(err) {
			// this might be about creating a file, so no error if not exists
			return "", "", err
		}
	} else {
		fname = fname2
	}

	// check if OK
	if fname[:len(f.Root)] != f.Root {
		// not ok
		return "", "", os.ErrNotExist
	}

	return fname, name, nil
}

func (f *FileHandler) OpenFile(ctx phpv.Context, fname string, mode string, _ ...phpv.Resource) (*Stream, error) {
	fname, name, err := f.localPath(fname)
	if err != nil {
		return nil, err
	}

	flags := 0

	// Strip binary/text flag from mode string.
	// PHP accepts b/t at any position: "rb", "wb+", "r+b", "w+t", etc.
	flag := ""
	cleaned := ""
	for i := 0; i < len(mode); i++ {
		if mode[i] == 'b' || mode[i] == 't' {
			if flag == "" {
				flag = string(mode[i])
			}
		} else {
			cleaned += string(mode[i])
		}
	}
	mode = cleaned

	appendMode := false
	switch mode {
	case "r":
		flags = os.O_RDONLY
	case "w":
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	case "a":
		flags = os.O_WRONLY | os.O_CREATE
		appendMode = true
	case "r+":
		flags = os.O_RDWR
	case "w+":
		flags = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	case "a+":
		flags = os.O_RDWR | os.O_CREATE
		appendMode = true
	case "x":
		flags = os.O_CREATE | os.O_EXCL | os.O_WRONLY
	case "x+":
		flags = os.O_CREATE | os.O_EXCL | os.O_RDWR
	case "c":
		flags = os.O_WRONLY | os.O_CREATE
	case "c+":
		flags = os.O_RDWR | os.O_CREATE
	case "e":
		flags = os.O_RDONLY | os.O_CREATE
	default:
		// Invalid mode - determine the first invalid character
		invalidChar := mode
		if len(invalidChar) > 0 {
			invalidChar = string(invalidChar[0])
		}
		return nil, fmt.Errorf("`%s' is not a valid mode for fopen", invalidChar)
	}

	res, err := os.OpenFile(fname, flags, 0666)
	if err != nil {
		return nil, err
	}

	var streamBackend interface{}
	if appendMode {
		// Wrap in appendFile for PHP-compatible position tracking.
		// PHP's append mode reports position from 0, advancing by bytes written,
		// while actual writes go to end of file.
		streamBackend = &appendFile{f: res, pos: 0}
	} else {
		streamBackend = res
	}

	s := NewStream(streamBackend)
	s.SetAttr("wrapper_type", "plainfile")
	s.SetAttr("stream_type", "Go")
	s.SetAttr("mode", mode)
	s.SetAttr("flag", flag)
	s.SetAttr("seekable", true)
	s.SetAttr("uri", name)

	s.ResourceType = phpv.ResourceStream
	s.ResourceID = ctx.Global().NextResourceID()

	return s, nil
}

func (f *FileHandler) Open(ctx phpv.Context, p *url.URL, mode string, streamCtx ...phpv.Resource) (*Stream, error) {
	// Check for remote host in file:// URIs - only localhost and empty host are allowed
	if p.Scheme == "file" && p.Host != "" && p.Host != "localhost" {
		ctx.Warn("Remote host file access not supported, %s", p.String())
		return nil, fmt.Errorf("no suitable wrapper could be found")
	}
	return f.OpenFile(ctx, p.Path, mode, streamCtx...)
}

func (f *FileHandler) Exists(p *url.URL) (bool, error) {
	fname, _, err := f.localPath(p.Path)
	if err != nil {
		return false, err
	}

	_, err = os.Lstat(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (f *FileHandler) Stat(p *url.URL) (os.FileInfo, error) {
	fname, _, err := f.localPath(p.Path)
	if err != nil {
		return nil, err
	}

	return os.Stat(fname) // TODO use Lstat instead, and resolve link locally
}

func (f *FileHandler) Lstat(p *url.URL) (os.FileInfo, error) {
	fname, _, err := f.localPath(p.Path)
	if err != nil {
		return nil, err
	}

	return os.Lstat(fname) // TODO use Lstat instead, and resolve link locally
}

func (f *FileHandler) Chdir(p string) error {
	fname, name, err := f.localPath(p)
	if err != nil {
		return err
	}

	s, err := os.Lstat(fname)
	if err != nil {
		return err
	}

	if !s.IsDir() {
		return &os.PathError{Op: "chdir", Path: p, Err: syscall.ENOTDIR}
	}

	f.Cwd = name
	return nil
}

func (f *FileHandler) Getwd() string {
	return f.Cwd
}
