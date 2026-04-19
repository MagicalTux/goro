package session

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/KarpelesLab/goro/core/phpv"
)

// phpSerialize calls the PHP serialize() function to serialize a value.
func phpSerialize(ctx phpv.Context, v *phpv.ZVal) (string, error) {
	fn, err := ctx.Global().GetFunction(ctx, phpv.ZString("serialize"))
	if err != nil {
		return "", fmt.Errorf("session: serialize function not available: %w", err)
	}
	result, err := ctx.CallZVal(ctx, fn, []*phpv.ZVal{v})
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

// phpUnserialize parses a PHP-serialized string starting at offset,
// returning the parsed value and the offset after the parsed data.
func phpUnserialize(ctx phpv.Context, data string, offset int) (*phpv.ZVal, int, error) {
	if offset >= len(data) {
		return phpv.ZNULL.ZVal(), offset, fmt.Errorf("session: unserialize: end of data")
	}

	sub := data[offset:]

	// Determine exactly how many bytes the single serialized value occupies,
	// so we pass only that to unserialize() and avoid "Extra data" warnings.
	consumed := measureSerializedLength(sub)
	if consumed <= 0 || consumed > len(sub) {
		consumed = len(sub)
	}
	valueData := sub[:consumed]

	fn, err := ctx.Global().GetFunction(ctx, phpv.ZString("unserialize"))
	if err != nil {
		return nil, offset, fmt.Errorf("session: unserialize function not available: %w", err)
	}

	result, err := ctx.CallZVal(ctx, fn, []*phpv.ZVal{phpv.ZString(valueData).ZVal()})
	if err != nil {
		return nil, offset, err
	}

	return result, offset + consumed, nil
}

// measureSerializedLength estimates how many bytes a single serialized PHP value occupies.
// This is needed to advance the offset when deserializing session data (key|value... format).
func measureSerializedLength(s string) int {
	if len(s) == 0 {
		return 0
	}
	switch s[0] {
	case 'N':
		// N;
		if len(s) >= 2 && s[1] == ';' {
			return 2
		}
	case 'b':
		// b:0; or b:1;
		if len(s) >= 4 && s[1] == ':' && s[3] == ';' {
			return 4
		}
	case 'i':
		// i:N;
		end := strings.IndexByte(s[2:], ';')
		if end >= 0 {
			return 2 + end + 1
		}
	case 'd':
		// d:N.N;
		end := strings.IndexByte(s[2:], ';')
		if end >= 0 {
			return 2 + end + 1
		}
	case 's':
		// s:N:"...";
		colonIdx := strings.IndexByte(s[2:], ':')
		if colonIdx < 0 {
			break
		}
		colonIdx += 2
		strLenStr := s[2:colonIdx]
		strLen, err := strconv.Atoi(strLenStr)
		if err != nil || strLen < 0 {
			break
		}
		// s:N:"...";
		// 2 + colonIdx-2 + 1 + 1 + strLen + 1 + 1 = colonIdx + strLen + 4
		end := colonIdx + 1 + 1 + strLen + 1 + 1 // :"<strLen bytes>";
		if end <= len(s) {
			return end
		}
	case 'S':
		// S:N:"..."; (escaped, deprecated)
		colonIdx := strings.IndexByte(s[2:], ':')
		if colonIdx < 0 {
			break
		}
		colonIdx += 2
		strLenStr := s[2:colonIdx]
		strLen, err := strconv.Atoi(strLenStr)
		if err != nil || strLen < 0 {
			break
		}
		end := colonIdx + 1 + 1 + strLen + 1 + 1
		if end <= len(s) {
			return end
		}
	case 'a':
		// a:N:{...}
		// We need to parse through the entire array to find its end
		return measureArrayOrObject(s, 'a')
	case 'O':
		// O:classLen:"ClassName":propCount:{...}
		return measureArrayOrObject(s, 'O')
	case 'C':
		// C:classLen:"ClassName":dataLen:{...}
		return measureCObject(s)
	case 'R', 'r':
		// R:N; or r:N;
		end := strings.IndexByte(s[2:], ';')
		if end >= 0 {
			return 2 + end + 1
		}
	case 'E':
		// E:N:"...";
		colonIdx := strings.IndexByte(s[2:], ':')
		if colonIdx < 0 {
			break
		}
		colonIdx += 2
		strLenStr := s[2:colonIdx]
		strLen, err := strconv.Atoi(strLenStr)
		if err != nil || strLen < 0 {
			break
		}
		end := colonIdx + 1 + 1 + strLen + 1 + 1
		if end <= len(s) {
			return end
		}
	}
	return len(s)
}

// measureArrayOrObject measures the length of an array (a:N:{...}) or object (O:...:N:{...}).
func measureArrayOrObject(s string, typ byte) int {
	// Find the '{' that starts the body
	braceIdx := strings.IndexByte(s, '{')
	if braceIdx < 0 {
		return len(s)
	}
	// Count nested braces to find matching '}'
	depth := 0
	for i := braceIdx; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(s)
}

// measureCObject measures a C:classLen:"ClassName":dataLen:{...} object.
func measureCObject(s string) int {
	// Find the last '{' + dataLen
	braceIdx := strings.IndexByte(s, '{')
	if braceIdx < 0 {
		return len(s)
	}
	// The data length is just before the '{'
	// Actually scan for matching brace
	depth := 0
	for i := braceIdx; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(s)
}
