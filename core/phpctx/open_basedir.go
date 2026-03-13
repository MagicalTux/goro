package phpctx

import (
	"path/filepath"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpv"
)

// CheckOpenBasedir checks if the given path is within the open_basedir restriction.
// Returns nil if access is allowed, or a sentinel error (ErrOpenBasedir) if restricted.
// A warning is emitted when access is blocked.
// The funcName parameter is used in the warning message (e.g. "file_exists").
func (g *Global) CheckOpenBasedir(ctx phpv.Context, path string, funcName string) error {
	basedir := g.GetConfig("open_basedir", phpv.ZNULL.ZVal()).String()
	if basedir == "" {
		return nil // no restriction
	}

	// Parse the basedir list (colon-separated on Unix, semicolon on Windows)
	dirs := strings.Split(basedir, string(filepath.ListSeparator))

	// Resolve the target path relative to cwd
	targetPath := path
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(string(g.Getwd()), targetPath)
	}
	// Clean but don't resolve symlinks for the check path
	targetPath = filepath.Clean(targetPath)

	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}

		// Resolve the basedir relative to cwd
		baseDir := dir
		if !filepath.IsAbs(baseDir) {
			baseDir = filepath.Join(string(g.Getwd()), baseDir)
		}
		baseDir = filepath.Clean(baseDir)

		// Ensure baseDir ends with separator for prefix matching
		if !strings.HasSuffix(baseDir, string(filepath.Separator)) {
			baseDir += string(filepath.Separator)
		}

		// Check if targetPath is within baseDir or equals baseDir (without trailing separator)
		targetCheck := targetPath
		if !strings.HasSuffix(targetCheck, string(filepath.Separator)) {
			targetCheck += string(filepath.Separator)
		}

		if strings.HasPrefix(targetCheck, baseDir) {
			return nil // allowed
		}
	}

	// Not allowed - emit warning with original path and return sentinel error
	ctx.Warn("%s(): open_basedir restriction in effect. File(%s) is not within the allowed path(s): (%s)",
		funcName, path, basedir, logopt.NoFuncName(true))
	return ErrOpenBasedir
}

// ErrOpenBasedir is returned when an open_basedir restriction blocks file access.
var ErrOpenBasedir = &phpv.PhpError{Code: phpv.E_WARNING}
