package standard

import (
	"strings"

	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/stream"
)

// getUserStreamHandler checks if a path uses a registered user stream wrapper.
func getUserStreamHandler(ctx phpv.Context, path string) *stream.UserStreamHandler {
	idx := strings.Index(path, "://")
	if idx < 1 {
		return nil
	}
	scheme := path[:idx]
	switch scheme {
	case "file", "php", "http", "https", "data", "glob", "phar", "ftp", "ftps",
		"zlib", "compress.zlib", "compress.bzip2":
		return nil
	}
	g := ctx.Global().(*phpctx.Global)
	if h, ok := g.GetStreamHandler(scheme); ok {
		if ush, ok := h.(*stream.UserStreamHandler); ok {
			return ush
		}
	}
	return nil
}
