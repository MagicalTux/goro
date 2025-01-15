package stream

import (
	"net/url"
	"os"
	"path"
	"path/filepath"
	"syscall"
)

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

func (f *FileHandler) OpenFile(fname string) (*Stream, error) {
	fname, name, err := f.localPath(fname)
	if err != nil {
		return nil, err
	}
	res, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	s := NewStream(res)
	s.SetAttr("wrapper_type", "plainfile")
	s.SetAttr("stream_type", "Go")
	s.SetAttr("mode", "r")
	s.SetAttr("seekable", true)
	s.SetAttr("uri", name)

	return s, nil
}

func (f *FileHandler) Open(p *url.URL) (*Stream, error) {
	return f.OpenFile(p.Path)
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
