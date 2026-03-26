package stream

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
)

// FilterWarning is an error that carries both the filtered output and a warning message.
// The caller should emit the warning but still use the output data.
type FilterWarning struct {
	Message string
	Data    []byte
}

func (e *FilterWarning) Error() string {
	return e.Message
}

// FilterFatalError indicates a user filter returned PSFS_ERR_FATAL.
type FilterFatalError struct {
	UnprocessedBuckets bool
}

func (e *FilterFatalError) Error() string {
	return "filter returned PSFS_ERR_FATAL"
}

// PSFS return codes from filter() method
const (
	PSFS_PASS_ON    = 2
	PSFS_FEED_ME    = 0
	PSFS_ERR_FATAL  = 1
)

// FilterDirection indicates whether a filter is for reading, writing, or both
type FilterDirection int

const (
	FilterRead  FilterDirection = 1
	FilterWrite FilterDirection = 2
	FilterAll   FilterDirection = 3
)

// StreamFilter is the interface that all stream filters implement.
// Process takes input data and returns filtered output data.
// The closing flag indicates the stream is closing (flush remaining data).
type StreamFilter interface {
	// Process filters data. Returns filtered output. If closing is true,
	// the filter should flush any buffered data.
	Process(data []byte, closing bool) ([]byte, error)
}

// StreamFilterResource is a resource that represents an attached filter.
// It can be passed to stream_filter_remove().
type StreamFilterResource struct {
	ResourceID   int
	ResourceType phpv.ResourceType
	FilterName   string
	Direction    FilterDirection
	Filter       StreamFilter
	Stream       *Stream // the stream this filter is attached to
	Removed      bool    // true after stream_filter_remove()
}

func (r *StreamFilterResource) GetType() phpv.ZType           { return phpv.ZtResource }
func (r *StreamFilterResource) GetResourceType() phpv.ResourceType { return r.ResourceType }
func (r *StreamFilterResource) GetResourceID() int             { return r.ResourceID }
func (r *StreamFilterResource) ZVal() *phpv.ZVal               { return phpv.NewZVal(r) }
func (r *StreamFilterResource) Value() phpv.Val                { return r }
func (r *StreamFilterResource) String() string {
	return "Resource id #" + string(rune('0'+r.ResourceID))
}
func (r *StreamFilterResource) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	switch t {
	case phpv.ZtResource:
		return r, nil
	case phpv.ZtBool:
		return phpv.ZBool(true), nil
	case phpv.ZtString:
		return phpv.ZString(r.String()), nil
	}
	return nil, ErrNotSupported
}

// filterEntry holds a filter in the stream's filter chain
type filterEntry struct {
	resource *StreamFilterResource
	filter   StreamFilter
}

// AddReadFilter appends a read filter to the stream's read filter chain.
func (s *Stream) AddReadFilter(res *StreamFilterResource, prepend bool) {
	entry := filterEntry{resource: res, filter: res.Filter}
	if prepend {
		s.readFilters = append([]filterEntry{entry}, s.readFilters...)
	} else {
		s.readFilters = append(s.readFilters, entry)
	}
}

// AddWriteFilter appends a write filter to the stream's write filter chain.
func (s *Stream) AddWriteFilter(res *StreamFilterResource, prepend bool) {
	entry := filterEntry{resource: res, filter: res.Filter}
	if prepend {
		s.writeFilters = append([]filterEntry{entry}, s.writeFilters...)
	} else {
		s.writeFilters = append(s.writeFilters, entry)
	}
}

// RemoveFilter removes a specific filter from the stream
func (s *Stream) RemoveFilter(res *StreamFilterResource) bool {
	found := false
	// Remove from read filters
	newRead := make([]filterEntry, 0, len(s.readFilters))
	for _, e := range s.readFilters {
		if e.resource == res {
			found = true
			continue
		}
		newRead = append(newRead, e)
	}
	s.readFilters = newRead

	// Remove from write filters
	newWrite := make([]filterEntry, 0, len(s.writeFilters))
	for _, e := range s.writeFilters {
		if e.resource == res {
			found = true
			continue
		}
		newWrite = append(newWrite, e)
	}
	s.writeFilters = newWrite
	return found
}

// HasFilters returns true if the stream has any filters
func (s *Stream) HasFilters() bool {
	return len(s.readFilters) > 0 || len(s.writeFilters) > 0
}

// ApplyReadFilters runs data through the read filter chain
func (s *Stream) ApplyReadFilters(data []byte, closing bool) ([]byte, error) {
	result := data
	var err error
	for _, entry := range s.readFilters {
		if entry.resource.Removed {
			continue
		}
		result, err = entry.filter.Process(result, closing)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

// ApplyWriteFilters runs data through the write filter chain
func (s *Stream) ApplyWriteFilters(data []byte, closing bool) ([]byte, error) {
	result := data
	var err error
	for _, entry := range s.writeFilters {
		if entry.resource.Removed {
			continue
		}
		result, err = entry.filter.Process(result, closing)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

// FlushWriteFilters flushes any buffered data in write filters (called on close)
func (s *Stream) FlushWriteFilters() ([]byte, error) {
	return s.ApplyWriteFilters(nil, true)
}

// FlushReadFilters flushes any buffered data in read filters (called on close)
func (s *Stream) FlushReadFilters() ([]byte, error) {
	return s.ApplyReadFilters(nil, true)
}

// --- Built-in Filters ---

// Rot13Filter implements the string.rot13 filter
type Rot13Filter struct{}

func (f *Rot13Filter) Process(data []byte, closing bool) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	result := make([]byte, len(data))
	for i, b := range data {
		switch {
		case b >= 'a' && b <= 'z':
			result[i] = 'a' + (b-'a'+13)%26
		case b >= 'A' && b <= 'Z':
			result[i] = 'A' + (b-'A'+13)%26
		default:
			result[i] = b
		}
	}
	return result, nil
}

// ToUpperFilter implements the string.toupper filter
type ToUpperFilter struct{}

func (f *ToUpperFilter) Process(data []byte, closing bool) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	return []byte(strings.ToUpper(string(data))), nil
}

// ToLowerFilter implements the string.tolower filter
type ToLowerFilter struct{}

func (f *ToLowerFilter) Process(data []byte, closing bool) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	return []byte(strings.ToLower(string(data))), nil
}

// Base64EncodeFilter implements the convert.base64-encode filter
type Base64EncodeFilter struct {
	lineLength int
	lineBreak  string
	buf        []byte // buffered input not yet encoded
}

func NewBase64EncodeFilter(params map[string]interface{}) *Base64EncodeFilter {
	f := &Base64EncodeFilter{
		lineLength: 0, // 0 means no wrapping
		lineBreak:  "\r\n",
	}
	if ll, ok := params["line-length"]; ok {
		switch v := ll.(type) {
		case int:
			f.lineLength = v
		case int64:
			f.lineLength = int(v)
		case phpv.ZInt:
			f.lineLength = int(v)
		}
	}
	if lb, ok := params["line-break-chars"]; ok {
		switch v := lb.(type) {
		case string:
			f.lineBreak = v
		case phpv.ZString:
			f.lineBreak = string(v)
		}
	}
	return f
}

func (f *Base64EncodeFilter) Process(data []byte, closing bool) ([]byte, error) {
	if len(data) == 0 && !closing {
		return nil, nil
	}

	// Buffer input for encoding
	f.buf = append(f.buf, data...)

	if !closing {
		// For non-closing, encode full 3-byte groups
		fullGroups := (len(f.buf) / 3) * 3
		if fullGroups == 0 {
			return nil, nil
		}
		encoded := base64.StdEncoding.EncodeToString(f.buf[:fullGroups])
		f.buf = f.buf[fullGroups:]
		if f.lineLength > 0 {
			encoded = wrapLines(encoded, f.lineLength, f.lineBreak)
		}
		return []byte(encoded), nil
	}

	// Closing: encode everything with padding
	if len(f.buf) == 0 {
		return nil, nil
	}
	encoded := base64.StdEncoding.EncodeToString(f.buf)
	f.buf = nil
	if f.lineLength > 0 {
		encoded = wrapLines(encoded, f.lineLength, f.lineBreak)
	}
	return []byte(encoded), nil
}

func wrapLines(s string, lineLen int, lineBreak string) string {
	if lineLen <= 0 || len(s) <= lineLen {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); i += lineLen {
		end := i + lineLen
		if end > len(s) {
			end = len(s)
		}
		if i > 0 {
			sb.WriteString(lineBreak)
		}
		sb.WriteString(s[i:end])
	}
	return sb.String()
}

// Base64DecodeFilter implements the convert.base64-decode filter
type Base64DecodeFilter struct {
	buf []byte
}

func (f *Base64DecodeFilter) Process(data []byte, closing bool) ([]byte, error) {
	if len(data) == 0 && !closing {
		return nil, nil
	}

	// Strip whitespace from input and buffer it
	for _, b := range data {
		if b == '\n' || b == '\r' || b == ' ' || b == '\t' {
			continue
		}
		f.buf = append(f.buf, b)
	}

	if !closing {
		// Decode full 4-byte groups
		fullGroups := (len(f.buf) / 4) * 4
		if fullGroups == 0 {
			return nil, nil
		}
		decoded, err := base64.StdEncoding.DecodeString(string(f.buf[:fullGroups]))
		if err != nil {
			return nil, err
		}
		f.buf = f.buf[fullGroups:]
		return decoded, nil
	}

	// Closing: decode everything
	if len(f.buf) == 0 {
		return nil, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(string(f.buf))
	f.buf = nil
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

// QuotedPrintableEncodeFilter implements convert.quoted-printable-encode
type QuotedPrintableEncodeFilter struct {
	binary bool
}

func NewQuotedPrintableEncodeFilter(params map[string]interface{}) *QuotedPrintableEncodeFilter {
	f := &QuotedPrintableEncodeFilter{}
	if b, ok := params["binary"]; ok {
		switch v := b.(type) {
		case bool:
			f.binary = v
		}
	}
	return f
}

func (f *QuotedPrintableEncodeFilter) Process(data []byte, closing bool) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	var sb strings.Builder
	for _, b := range data {
		if f.binary {
			// In binary mode, encode everything except printable ASCII
			if b >= 33 && b <= 126 && b != '=' {
				sb.WriteByte(b)
			} else {
				sb.WriteString("=" + strings.ToUpper(hexByte(b)))
			}
		} else {
			// Normal mode: pass through printable ASCII and spaces/tabs
			// Encode CR, LF, and non-printable characters
			if b == '\r' || b == '\n' {
				sb.WriteString("=" + strings.ToUpper(hexByte(b)))
			} else if b == '\t' && f.binary {
				sb.WriteString("=" + strings.ToUpper(hexByte(b)))
			} else if b >= 32 && b <= 126 && b != '=' {
				sb.WriteByte(b)
			} else if b == '\t' {
				sb.WriteByte(b)
			} else {
				sb.WriteString("=" + strings.ToUpper(hexByte(b)))
			}
		}
	}
	return []byte(sb.String()), nil
}

func hexByte(b byte) string {
	const hex = "0123456789ABCDEF"
	return string([]byte{hex[b>>4], hex[b&0x0f]})
}

// QuotedPrintableDecodeFilter implements convert.quoted-printable-decode
type QuotedPrintableDecodeFilter struct {
	buf []byte // partial escape sequence buffer
}

func (f *QuotedPrintableDecodeFilter) Process(data []byte, closing bool) ([]byte, error) {
	if len(data) == 0 && !closing {
		return data, nil
	}

	input := append(f.buf, data...)
	f.buf = nil

	var result []byte
	hasInvalid := false
	i := 0
	for i < len(input) {
		if input[i] == '=' {
			if i+2 < len(input) {
				h1 := input[i+1]
				h2 := input[i+2]
				// Check for valid hex-encoded byte first
				v1 := unhex(h1)
				v2 := unhex(h2)
				if v1 >= 0 && v2 >= 0 {
					result = append(result, byte(v1<<4|v2))
					i += 3
					continue
				}
				// Check for soft line break =\r\n
				if h1 == '\r' && h2 == '\n' {
					i += 3
					continue
				}
				// Invalid sequence - pass through the = and flag warning
				hasInvalid = true
				result = append(result, input[i])
				i++
			} else if i+1 < len(input) && input[i+1] == '\n' {
				// Soft line break (bare LF)
				i += 2
				continue
			} else if !closing {
				// Not enough data, buffer for next call
				f.buf = input[i:]
				if hasInvalid {
					return result, &FilterWarning{
						Message: fmt.Sprintf("Stream filter (convert.quoted-printable-decode): invalid byte sequence"),
						Data:    result,
					}
				}
				return result, nil
			} else {
				// Closing with incomplete sequence - treat as soft line break
				i = len(input) // skip remaining
			}
		} else {
			result = append(result, input[i])
			i++
		}
	}
	if hasInvalid {
		return result, &FilterWarning{
			Message: fmt.Sprintf("Stream filter (convert.quoted-printable-decode): invalid byte sequence"),
			Data:    result,
		}
	}
	return result, nil
}

func unhex(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b - 'a' + 10)
	case b >= 'A' && b <= 'F':
		return int(b - 'A' + 10)
	}
	return -1
}

// CreateBuiltinFilter creates a built-in filter by name, or returns nil if not found.
func CreateBuiltinFilter(name string, params map[string]interface{}) StreamFilter {
	switch name {
	case "string.rot13":
		return &Rot13Filter{}
	case "string.toupper":
		return &ToUpperFilter{}
	case "string.tolower":
		return &ToLowerFilter{}
	case "convert.base64-encode":
		return NewBase64EncodeFilter(params)
	case "convert.base64-decode":
		return &Base64DecodeFilter{}
	case "convert.quoted-printable-encode":
		return NewQuotedPrintableEncodeFilter(params)
	case "convert.quoted-printable-decode":
		return &QuotedPrintableDecodeFilter{}
	}
	return nil
}

// IsBuiltinFilter returns true if the filter name is a known built-in filter
func IsBuiltinFilter(name string) bool {
	switch name {
	case "string.rot13", "string.toupper", "string.tolower", "string.strip_tags",
		"convert.base64-encode", "convert.base64-decode",
		"convert.quoted-printable-encode", "convert.quoted-printable-decode",
		"dechunk":
		return true
	}
	return false
}
