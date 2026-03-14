package phpctx

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

//functions for parsing request, including GET, POST, etc

func (g *Global) parsePost(p, f *phpv.ZArray) error {
	if g.req.Body == nil {
		return errors.New("missing form body")
	}

	// Check post_max_size enforcement
	postMaxSize := parseIniSize(g.GetConfig("post_max_size", phpv.ZString("8M").ZVal()).String())
	if postMaxSize > 0 {
		// Determine actual content length from the stored raw body or Content-Length header
		var contentLength int64
		if g.rawRequestBody != nil {
			contentLength = int64(len(g.rawRequestBody))
		} else if g.req.ContentLength > 0 {
			contentLength = g.req.ContentLength
		}
		if contentLength > postMaxSize {
			g.WriteStartupWarning(fmt.Sprintf("\nWarning: PHP Request Startup: POST Content-Length of %d bytes exceeds the limit of %d bytes in Unknown on line 0\n", contentLength, postMaxSize))
			return nil // skip parsing, leave $_POST empty
		}
	}
	ct := g.req.Header.Get("Content-Type")
	// RFC 7231, section 3.1.1.5 - empty type MAY be treated as application/octet-stream
	if ct == "" {
		ct = "application/octet-stream"
	}
	ct, params, parseErr := mime.ParseMediaType(ct)
	// If Go's parser fails (e.g., comma in boundary), manually extract boundary
	// PHP truncates boundary at comma: "boundary=foo, charset=..." → "foo"
	if parseErr != nil && params == nil {
		params = make(map[string]string)
	}
	if _, ok := params["boundary"]; !ok && strings.Contains(strings.ToLower(g.req.Header.Get("Content-Type")), "boundary") {
		rawCT := g.req.Header.Get("Content-Type")
		for _, part := range strings.Split(rawCT, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), "boundary") {
				if eqIdx := strings.IndexByte(part, '='); eqIdx != -1 {
					b := strings.TrimSpace(part[eqIdx+1:])
					// PHP truncates at comma
					if commaIdx := strings.IndexByte(b, ','); commaIdx != -1 {
						b = b[:commaIdx]
					}
					// Strip surrounding quotes
					if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
						b = b[1 : len(b)-1]
					}
					params["boundary"] = b
					break
				}
			}
		}
	}

	switch {
	case ct == "application/x-www-form-urlencoded":
		var reader io.Reader = g.req.Body
		maxFormSize := int64(10 << 20) // 10 MB is a lot of text.
		reader = io.LimitReader(g.req.Body, maxFormSize+1)
		b, err := ioutil.ReadAll(reader)
		if err != nil {
			return err
		}
		if int64(len(b)) > maxFormSize {
			return errors.New("http: POST too large")
		}
		err = g.MemAlloc(g, uint64(len(b)))
		if err != nil {
			return err
		}
		return ParseQueryToArray(g, string(b), p)
	case ct == "multipart/form-data": //, "multipart/mixed": // should we allow mixed?
		boundary, ok := params["boundary"]
		if !ok {
			// No boundary parameter at all
			g.WriteStartupWarning("\nWarning: PHP Request Startup: Missing boundary in multipart/form-data POST data in Unknown on line 0\n")
			return nil
		}
		if parseErr != nil && strings.HasPrefix(boundary, "\"") {
			// Go's parser failed and fallback extracted a boundary with unclosed quote — malformed
			g.WriteStartupWarning("\nWarning: PHP Request Startup: Invalid boundary in multipart/form-data POST data in Unknown on line 0\n")
			return nil
		}
		read := multipart.NewReader(io.LimitReader(g.req.Body, 64*1024*1024), boundary) // max 64MB body size, TODO use php.ini to set this value

		// File upload settings
		fileUploads := g.GetConfig("file_uploads", phpv.ZString("1").ZVal()).String() != "0"
		uploadMaxFilesize := parseIniSize(g.GetConfig("upload_max_filesize", phpv.ZString("2M").ZVal()).String())
		maxFileUploads := parseIniSize(g.GetConfig("max_file_uploads", phpv.ZString("20").ZVal()).String())
		uploadTmpDir := g.GetConfig("upload_tmp_dir", phpv.ZString("").ZVal()).String()
		if uploadTmpDir == "" {
			uploadTmpDir = os.TempDir()
		}
		var fileCount int64
		var anonFileIdx int
		var maxFileSize int64 = -1 // MAX_FILE_SIZE form field, -1 = not set

		for {
			part, err := read.NextPart()
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}

			k := phpFormName(part)
			cd := part.Header.Get("Content-Disposition")
			fn := phpPartFilename(part)
			hasFilenameParam := strings.Contains(strings.ToLower(cd), "filename=")

			if hasFilenameParam {
				// This is a file upload part
				if !fileUploads {
					// file_uploads=0: silently skip file parts, consume body
					io.Copy(io.Discard, part)
					continue
				}

				// Validate name for proper bracket structure
				if k != "" && !isValidFileUploadName(k) {
					io.Copy(io.Discard, part)
					continue
				}

				// Empty filename = UPLOAD_ERR_NO_FILE (doesn't count towards limit)
				if fn == "" {
					io.Copy(io.Discard, part)
					fileEntry := phpv.NewZArray()
					fileEntry.OffsetSet(g, phpv.ZString("name").ZVal(), phpv.ZString("").ZVal())
					fileEntry.OffsetSet(g, phpv.ZString("full_path").ZVal(), phpv.ZString("").ZVal())
					fileEntry.OffsetSet(g, phpv.ZString("type").ZVal(), phpv.ZString("").ZVal())
					fileEntry.OffsetSet(g, phpv.ZString("tmp_name").ZVal(), phpv.ZString("").ZVal())
					fileEntry.OffsetSet(g, phpv.ZString("error").ZVal(), phpv.ZInt(4).ZVal()) // UPLOAD_ERR_NO_FILE
					fileEntry.OffsetSet(g, phpv.ZString("size").ZVal(), phpv.ZInt(0).ZVal())
					if k == "" {
						f.OffsetSet(g, phpv.ZInt(anonFileIdx).ZVal(), fileEntry.ZVal())
						anonFileIdx++
					} else {
						setFileToArray(g, k, fileEntry, f)
					}
					continue
				}

				fileCount++
				if maxFileUploads > 0 && fileCount > maxFileUploads {
					io.Copy(io.Discard, part)
					continue
				}

				// Extract basename for "name" field, keep raw for "full_path"
				rawFilename := fn
				basename := phpBasename(rawFilename)

				// Read file data to temp file
				tmpFile, err := os.CreateTemp(uploadTmpDir, "php")
				if err != nil {
					io.Copy(io.Discard, part)
					continue
				}

				size, copyErr := io.Copy(tmpFile, part)
				tmpFile.Close()

				// Determine upload error
				uploadErr := phpv.ZInt(0) // UPLOAD_ERR_OK
				if copyErr != nil {
					// Partial upload (truncated/missing boundary)
					os.Remove(tmpFile.Name())
					uploadErr = phpv.ZInt(3) // UPLOAD_ERR_PARTIAL
				} else if uploadMaxFilesize > 0 && size > uploadMaxFilesize {
					os.Remove(tmpFile.Name())
					uploadErr = phpv.ZInt(1) // UPLOAD_ERR_INI_SIZE
				} else if maxFileSize >= 0 && size > maxFileSize {
					os.Remove(tmpFile.Name())
					uploadErr = phpv.ZInt(2) // UPLOAD_ERR_FORM_SIZE
				}

				// Register temp file for cleanup (even on error, in case it wasn't removed)
				if uploadErr == 0 {
					g.RegisterTempFile(tmpFile.Name())
					g.RegisterUploadedFile(tmpFile.Name())
				}

				// Clean Content-Type (trim trailing semicolons/whitespace)
				contentType := strings.TrimRight(part.Header.Get("Content-Type"), "; \t")

				// Build the $_FILES entry
				fileEntry := phpv.NewZArray()
				fileEntry.OffsetSet(g, phpv.ZString("name").ZVal(), phpv.ZString(basename).ZVal())
				fileEntry.OffsetSet(g, phpv.ZString("full_path").ZVal(), phpv.ZString(rawFilename).ZVal())
				if uploadErr == 0 {
					fileEntry.OffsetSet(g, phpv.ZString("type").ZVal(), phpv.ZString(contentType).ZVal())
					fileEntry.OffsetSet(g, phpv.ZString("tmp_name").ZVal(), phpv.ZString(tmpFile.Name()).ZVal())
					fileEntry.OffsetSet(g, phpv.ZString("error").ZVal(), uploadErr.ZVal())
					fileEntry.OffsetSet(g, phpv.ZString("size").ZVal(), phpv.ZInt(size).ZVal())
				} else {
					fileEntry.OffsetSet(g, phpv.ZString("type").ZVal(), phpv.ZString("").ZVal())
					fileEntry.OffsetSet(g, phpv.ZString("tmp_name").ZVal(), phpv.ZString("").ZVal())
					fileEntry.OffsetSet(g, phpv.ZString("error").ZVal(), uploadErr.ZVal())
					fileEntry.OffsetSet(g, phpv.ZString("size").ZVal(), phpv.ZInt(0).ZVal())
				}

				// Add to $_FILES
				if k == "" {
					f.OffsetSet(g, phpv.ZInt(anonFileIdx).ZVal(), fileEntry.ZVal())
					anonFileIdx++
				} else {
					setFileToArray(g, k, fileEntry, f)
				}
				continue
			}

			if k == "" {
				// No name and no filename — garbled MIME headers
				io.Copy(io.Discard, part)
				g.WriteStartupWarning("\nWarning: PHP Request Startup: File Upload Mime headers garbled in Unknown on line 0\n")
				continue
			}

			b := &bytes.Buffer{}
			_, err = g.mem.Copy(b, part) // count size against memory usage
			if err != nil {
				return err
			}

			// Track MAX_FILE_SIZE form field
			if k == "MAX_FILE_SIZE" {
				if n, parseErr := strconv.ParseInt(strings.TrimSpace(b.String()), 10, 64); parseErr == nil {
					maxFileSize = n
				}
			}

			err = setUrlValueToArray(g, k, phpv.ZString(b.Bytes()), p)
			if err != nil {
				return err
			}
		}
		return nil
	default:
		return errors.New("Failed to parse POST: unknown content type")
	}
}

// phpURLDecode decodes a URL-encoded string like PHP's urldecode:
// - Converts %XX hex sequences to bytes
// - Leaves malformed percent sequences (like %&' or trailing %) as-is
// - Does NOT convert '+' to space (unlike query strings)
func phpURLDecode(s string) string {
	var buf strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi := unhex(s[i+1])
			lo := unhex(s[i+2])
			if hi >= 0 && lo >= 0 {
				buf.WriteByte(byte(hi<<4 | lo))
				i += 2
				continue
			}
		}
		buf.WriteByte(s[i])
	}
	return buf.String()
}

func unhex(c byte) int {
	switch {
	case '0' <= c && c <= '9':
		return int(c - '0')
	case 'a' <= c && c <= 'f':
		return int(c - 'a' + 10)
	case 'A' <= c && c <= 'F':
		return int(c - 'A' + 10)
	}
	return -1
}

// phpFormName extracts the "name" parameter from a multipart part's
// Content-Disposition header using PHP-compatible quoting rules:
//   - Double-quoted: strip quotes, \\ → \, \" → "
//   - Single-quoted: strip quotes, \\ → \, \' → '
//   - Unquoted: read until ; or end, \\ → \, keep other backslashes as-is
func phpFormName(part *multipart.Part) string {
	cd := part.Header.Get("Content-Disposition")
	if cd == "" {
		return ""
	}
	// Find name= parameter (case-insensitive), not inside "filename="
	lower := strings.ToLower(cd)
	idx := strings.Index(lower, "name=")
	for idx >= 0 {
		// Ensure this is a standalone "name=" parameter, not part of "filename="
		if idx == 0 || lower[idx-1] == ';' || lower[idx-1] == ' ' || lower[idx-1] == '\t' {
			break // good match
		}
		// Try next occurrence
		nextIdx := strings.Index(lower[idx+5:], "name=")
		if nextIdx < 0 {
			idx = -1
			break
		}
		idx = idx + 5 + nextIdx
	}
	if idx < 0 {
		return ""
	}
	rest := cd[idx+5:] // after "name="
	if len(rest) == 0 {
		return ""
	}

	switch rest[0] {
	case '"':
		// Double-quoted: read until unescaped closing "
		var buf strings.Builder
		for i := 1; i < len(rest); i++ {
			if rest[i] == '\\' && i+1 < len(rest) {
				next := rest[i+1]
				if next == '"' || next == '\\' {
					buf.WriteByte(next)
					i++
					continue
				}
				// Other backslash sequences kept as-is
				buf.WriteByte(rest[i])
				continue
			}
			if rest[i] == '"' {
				break
			}
			buf.WriteByte(rest[i])
		}
		return buf.String()
	case '\'':
		// Single-quoted: read until unescaped closing '
		var buf strings.Builder
		for i := 1; i < len(rest); i++ {
			if rest[i] == '\\' && i+1 < len(rest) {
				next := rest[i+1]
				if next == '\'' || next == '\\' {
					buf.WriteByte(next)
					i++
					continue
				}
				// Other backslash sequences kept as-is
				buf.WriteByte(rest[i])
				continue
			}
			if rest[i] == '\'' {
				break
			}
			buf.WriteByte(rest[i])
		}
		return buf.String()
	default:
		// Unquoted: read until ; or end of string
		var buf strings.Builder
		for i := 0; i < len(rest); i++ {
			if rest[i] == ';' {
				break
			}
			if rest[i] == '\\' && i+1 < len(rest) {
				next := rest[i+1]
				if next == '\\' {
					buf.WriteByte('\\')
					i++
					continue
				}
				// Other backslash sequences kept as-is
				buf.WriteByte(rest[i])
				continue
			}
			buf.WriteByte(rest[i])
		}
		return strings.TrimRight(buf.String(), " \t")
	}
}

// parseCookiesToArray parses a Cookie header value into a ZArray following PHP's rules:
// - Cookies separated by ';'
// - Cookie names are NOT URL-decoded
// - Cookie values ARE URL-decoded
// - Dots and spaces in cookie names are replaced with underscores
// - Empty cookies (no name) are skipped
// - First occurrence wins for duplicate cookie names
func parseCookiesToArray(ctx phpv.Context, cookieHeader string, a *phpv.ZArray) {
	if cookieHeader == "" {
		return
	}

	for _, cookie := range strings.Split(cookieHeader, ";") {
		cookie = strings.TrimLeft(cookie, " \t")
		if cookie == "" {
			continue
		}

		eqIdx := strings.IndexByte(cookie, '=')
		var name, value string
		if eqIdx == -1 {
			// Cookie without '=' — PHP treats this as name="" (empty value)
			name = cookie
			value = ""
		} else {
			name = cookie[:eqIdx]
			value = cookie[eqIdx+1:]
		}

		// Trim leading/trailing spaces from name
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// URL-decode the value using PHP-compatible decoding that handles
		// malformed percent sequences gracefully (leaves them as-is)
		if eqIdx != -1 {
			value = phpURLDecode(value)
		}

		// Normalize the name: replace dots and spaces with underscores
		normalizedName := strings.NewReplacer(".", "_", " ", "_").Replace(name)

		// First occurrence wins: check if key already exists
		if exists, _ := a.OffsetExists(ctx, phpv.ZString(normalizedName).ZVal()); exists {
			continue
		}

		// Use setUrlValueToArray which handles nested array syntax (name[key])
		setUrlValueToArray(ctx, name, phpv.ZString(value), a)
	}
}

// ParseQueryToArray will parse a given query string into a ZArray with PHP parsing rules
func ParseQueryToArray(ctx phpv.Context, q string, a *phpv.ZArray) error {
	// parse this ourselves instead of using url.Values so we can keep the order right
	for len(q) > 0 {
		p := strings.IndexByte(q, '&')
		if p == -1 {
			return parseQueryFragmentToArray(ctx, q, a)
		} else {
			err := parseQueryFragmentToArray(ctx, q[:p], a)
			if err != nil {
				return err
			}
			q = q[p+1:]
		}
	}
	return nil
}

func parseQueryFragmentToArray(ctx phpv.Context, f string, a *phpv.ZArray) error {
	p := strings.IndexByte(f, '=')
	if p == -1 {
		f, _ = url.QueryUnescape(f) // ignore errors
		return setUrlValueToArray(ctx, f, phpv.ZNULL, a)
	}
	k, _ := url.QueryUnescape(f[:p])
	f, _ = url.QueryUnescape(f[p+1:])
	return setUrlValueToArray(ctx, k, phpv.ZString(f), a)
}

func setUrlValueToArray(ctx phpv.Context, k string, v phpv.Val, a *phpv.ZArray) error {
	// Check max_input_nesting_level: count bracket pairs to determine nesting depth.
	// Root variable counts as level 1, each bracket pair adds one level.
	if maxLevel := parseIniSize(ctx.Global().GetConfig("max_input_nesting_level", phpv.ZString("64").ZVal()).String()); maxLevel > 0 {
		depth := int64(1) // root variable name is level 1
		for _, c := range k {
			if c == '[' {
				depth++
			}
		}
		if depth > maxLevel {
			return nil // silently drop variables that exceed nesting level
		}
	}

	// Normalize dots and spaces in the first key component (PHP behavior)
	normalizeKey := func(s string) string {
		return strings.NewReplacer(".", "_", " ", "_").Replace(s)
	}

	p := strings.IndexByte(k, '[')
	if p == -1 {
		// simple
		return a.OffsetSet(ctx, phpv.ZString(normalizeKey(k)).ZVal(), v.ZVal())
	}
	if p == 0 {
		// failure
		return errors.New("invalid key")
	}

	// Check if there's a matching ] after the [
	q := strings.IndexByte(k[p:], ']')
	if q == -1 {
		// No matching ], treat entire key as flat name
		// Replace [, ., and space with _ (PHP behavior)
		flat := strings.NewReplacer(".", "_", " ", "_", "[", "_").Replace(k)
		return a.OffsetSet(ctx, phpv.ZString(flat).ZVal(), v.ZVal())
	}

	n := a
	zk := phpv.ZString(normalizeKey(k[:p])).ZVal()

	// loop through what remains of k
	k = k[p:]

	for {
		if len(k) == 0 {
			break
		}
		if k[0] != '[' {
			// php will ignore data after last bracket
			break
		}
		k = k[1:]
		p = strings.IndexByte(k, ']')
		if p == -1 {
			break // php will ignore data after last bracket
		}

		// use zk
		if zk == nil {
			xn := phpv.NewZArray()
			err := n.OffsetSet(ctx, zk, xn.ZVal())
			if err != nil {
				return err
			}
			n = xn
		} else if has, err := n.OffsetExists(ctx, zk); err != nil {
			return err
		} else if has {
			z, err := n.OffsetGet(ctx, zk)
			if err != nil {
				return err
			}
			z, err = z.As(ctx, phpv.ZtArray)
			if err != nil {
				return err
			}
			n = z.Value().(*phpv.ZArray)
		} else {
			xn := phpv.NewZArray()
			err = n.OffsetSet(ctx, zk, xn.ZVal())
			if err != nil {
				return err
			}
			n = xn
		}

		// update zk
		if p == 0 {
			zk = nil
			k = k[1:]
			continue
		}

		zk = phpv.ZString(k[:p]).ZVal()
		k = k[p+1:]
	}
	return n.OffsetSet(ctx, zk, v.ZVal())
}

// parseIniSize parses a PHP INI size value with optional K, M, G suffix.
// Returns the size in bytes. "1K" → 1024, "8M" → 8388608, "0" → 0.
func parseIniSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Strip surrounding quotes (INI defaults use quoted values like `"8M"`)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		return 0
	}
	last := s[len(s)-1]
	var multiplier int64 = 1
	switch last {
	case 'k', 'K':
		multiplier = 1024
		s = s[:len(s)-1]
	case 'm', 'M':
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case 'g', 'G':
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n * multiplier
}

// setFileToArray sets a file entry in $_FILES following PHP's array structure.
// For simple names like "file", it sets $_FILES["file"] = entry.
// For array names like "file[]" or "file[key]", it builds nested arrays
// with separate sub-arrays for each field (name, type, tmp_name, error, size).
func setFileToArray(ctx phpv.Context, k string, entry *phpv.ZArray, f *phpv.ZArray) error {
	p := strings.IndexByte(k, '[')
	if p == -1 {
		// Simple name: $_FILES["k"] = entry
		return f.OffsetSet(ctx, phpv.ZString(k).ZVal(), entry.ZVal())
	}

	// Array name: $_FILES["file"]["name"][...] = ..., etc.
	baseName := k[:p]
	suffix := k[p:]

	// Get or create the base array
	baseKey := phpv.ZString(baseName).ZVal()
	var base *phpv.ZArray
	if existing, err := f.OffsetGet(ctx, baseKey); err == nil && existing.GetType() == phpv.ZtArray {
		base = existing.Value().(*phpv.ZArray)
	} else {
		base = phpv.NewZArray()
		f.OffsetSet(ctx, baseKey, base.ZVal())
	}

	// For each field, set the value at field+suffix path within base.
	// e.g., for suffix="[]", this creates base["name"][] = val, base["type"][] = val, etc.
	fields := []string{"name", "full_path", "type", "tmp_name", "error", "size"}
	for _, field := range fields {
		fieldKey := phpv.ZString(field).ZVal()
		val, _ := entry.OffsetGet(ctx, fieldKey)
		setUrlValueToArray(ctx, field+suffix, val.Value(), base)
	}

	return nil
}

// phpBasename returns the filename component of a path, handling both
// forward slashes and backslashes (like PHP's basename for uploaded files).
func phpBasename(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '/' || filename[i] == '\\' {
			return filename[i+1:]
		}
	}
	return filename
}

// phpPartFilename extracts the raw filename from a multipart part's
// Content-Disposition header without applying filepath.Base() (which Go's
// part.FileName() does). This preserves directory paths for full_path support.
func phpPartFilename(part *multipart.Part) string {
	cd := part.Header.Get("Content-Disposition")
	if cd == "" {
		return ""
	}
	lower := strings.ToLower(cd)
	idx := strings.Index(lower, "filename=")
	if idx < 0 {
		return ""
	}
	// Make sure this is "filename=" not "filename*="
	if idx > 0 && lower[idx-1] != ' ' && lower[idx-1] != ';' && lower[idx-1] != '\t' {
		return ""
	}
	rest := cd[idx+9:] // after "filename="
	if len(rest) == 0 {
		return ""
	}
	if rest[0] == '"' {
		// Quoted value — PHP does NOT process backslash escapes in filename values,
		// so backslashes are kept as-is (important for Windows paths)
		end := strings.IndexByte(rest[1:], '"')
		if end < 0 {
			return rest[1:] // unclosed quote, take rest
		}
		return rest[1 : 1+end]
	}
	// Unquoted value - read until ; or end
	end := strings.IndexByte(rest, ';')
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}

// isValidFileUploadName validates that a file upload name has proper bracket
// structure. Returns false for malformed names like "foo[]bar" or "foo[[key]".
func isValidFileUploadName(k string) bool {
	p := strings.IndexByte(k, '[')
	if p == -1 {
		return true // simple name
	}
	if p == 0 {
		return false // starts with [
	}
	rest := k[p:]
	for len(rest) > 0 {
		if rest[0] != '[' {
			return false // text between bracket pairs
		}
		close := strings.IndexByte(rest, ']')
		if close == -1 {
			return false // unclosed bracket
		}
		// Check for nested [ before ]
		if strings.IndexByte(rest[1:close], '[') != -1 {
			return false // nested bracket
		}
		rest = rest[close+1:]
	}
	return true
}
