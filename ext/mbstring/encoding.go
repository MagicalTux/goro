package mbstring

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
)

// encodingMap maps PHP-style encoding names (uppercased) to Go text encodings.
var encodingMap map[string]encoding.Encoding

func init() {
	encodingMap = map[string]encoding.Encoding{
		// ASCII is a subset of UTF-8 — use a custom identity encoder
		"ASCII": nil, // handled specially

		// Unicode
		"UTF-8":    encoding.Nop,
		"UTF8":     encoding.Nop,
		"UTF-16":   unicode.UTF16(unicode.BigEndian, unicode.UseBOM),
		"UTF-16BE": unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
		"UTF-16LE": unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM),
		"UTF-32":   nil, // handled specially
		"UTF-32BE": nil, // handled specially
		"UTF-32LE": nil, // handled specially
		"UCS-2":    unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
		"UCS-2BE":  unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
		"UCS-2LE":  unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM),
		"UCS-4":    nil, // handled specially (same as UTF-32)
		"UCS-4BE":  nil, // handled specially (same as UTF-32BE)
		"UCS-4LE":  nil, // handled specially (same as UTF-32LE)

		// ISO-8859 family
		"ISO-8859-1":  charmap.ISO8859_1,
		"ISO-8859-2":  charmap.ISO8859_2,
		"ISO-8859-3":  charmap.ISO8859_3,
		"ISO-8859-4":  charmap.ISO8859_4,
		"ISO-8859-5":  charmap.ISO8859_5,
		"ISO-8859-6":  charmap.ISO8859_6,
		"ISO-8859-7":  charmap.ISO8859_7,
		"ISO-8859-8":  charmap.ISO8859_8,
		"ISO-8859-9":  charmap.ISO8859_9,
		"ISO-8859-10": charmap.ISO8859_10,
		"ISO-8859-13": charmap.ISO8859_13,
		"ISO-8859-14": charmap.ISO8859_14,
		"ISO-8859-15": charmap.ISO8859_15,
		"ISO-8859-16": charmap.ISO8859_16,

		// Aliases for ISO-8859
		"LATIN1": charmap.ISO8859_1,
		"LATIN2": charmap.ISO8859_2,
		"LATIN3": charmap.ISO8859_3,
		"LATIN4": charmap.ISO8859_4,
		"LATIN5": charmap.ISO8859_9,
		"LATIN6": charmap.ISO8859_10,

		// Windows code pages
		"WINDOWS-1250": charmap.Windows1250,
		"WINDOWS-1251": charmap.Windows1251,
		"WINDOWS-1252": charmap.Windows1252,
		"WINDOWS-1253": charmap.Windows1253,
		"WINDOWS-1254": charmap.Windows1254,
		"WINDOWS-1255": charmap.Windows1255,
		"WINDOWS-1256": charmap.Windows1256,
		"WINDOWS-1257": charmap.Windows1257,
		"WINDOWS-1258": charmap.Windows1258,
		"CP1250":       charmap.Windows1250,
		"CP1251":       charmap.Windows1251,
		"CP1252":       charmap.Windows1252,
		"CP1253":       charmap.Windows1253,
		"CP1254":       charmap.Windows1254,
		"CP1255":       charmap.Windows1255,
		"CP1256":       charmap.Windows1256,
		"CP1257":       charmap.Windows1257,
		"CP1258":       charmap.Windows1258,

		// DOS code pages
		"CP850": charmap.CodePage850,
		"CP866": charmap.CodePage866,
		"IBM866": charmap.CodePage866,

		// KOI8
		"KOI8-R": charmap.KOI8R,
		"KOI8-U": charmap.KOI8U,
		"KOI8R":  charmap.KOI8R,
		"KOI8U":  charmap.KOI8U,

		// Japanese
		"EUC-JP":     japanese.EUCJP,
		"EUCJP":      japanese.EUCJP,
		"EUCJP-WIN":  japanese.EUCJP,
		"EUC-JP-WIN": japanese.EUCJP,
		"SJIS":       japanese.ShiftJIS,
		"SHIFT_JIS":  japanese.ShiftJIS,
		"SHIFT-JIS":  japanese.ShiftJIS,
		"CP932":      japanese.ShiftJIS,
		"SJIS-WIN":   japanese.ShiftJIS,
		"ISO-2022-JP": japanese.ISO2022JP,

		// Korean
		"EUC-KR": korean.EUCKR,
		"EUCKR":  korean.EUCKR,
		"UHC":    korean.EUCKR,

		// Chinese
		"BIG5":    traditionalchinese.Big5,
		"BIG-5":   traditionalchinese.Big5,
		"CP950":   traditionalchinese.Big5,
		"GB18030": simplifiedchinese.GB18030,
		"GB2312":  simplifiedchinese.HZGB2312,
		"GBK":     simplifiedchinese.GBK,
		"CP936":   simplifiedchinese.GBK,
		"EUC-CN":  simplifiedchinese.GBK,
		"EUCCN":   simplifiedchinese.GBK,
		"HZ":      simplifiedchinese.HZGB2312,

		// Mac encodings
		"MACINTOSH": charmap.Macintosh,
		"MAC":       charmap.Macintosh,

		// ARMSCII-8 (Armenian)
		// Not directly available in x/text, but we'll handle it specially if needed
	}
}

// normalizeEncodingName normalizes a PHP encoding name for lookup.
func normalizeEncodingName(name string) string {
	upper := strings.ToUpper(strings.TrimSpace(name))
	// Handle common variations
	switch {
	case upper == "UTF8":
		return "UTF-8"
	case upper == "UTF16":
		return "UTF-16"
	case upper == "UTF16BE":
		return "UTF-16BE"
	case upper == "UTF16LE":
		return "UTF-16LE"
	case upper == "UTF32":
		return "UTF-32"
	case upper == "UTF32BE":
		return "UTF-32BE"
	case upper == "UTF32LE":
		return "UTF-32LE"
	}
	return upper
}

// getEncoding returns the Go encoding for a PHP encoding name.
// Returns nil, true if the encoding is known but handled specially (ASCII, UTF-32).
// Returns nil, false if the encoding is unknown.
func getEncoding(name string) (encoding.Encoding, bool) {
	normalized := normalizeEncodingName(name)
	enc, ok := encodingMap[normalized]
	if ok {
		return enc, true
	}
	// Try without hyphens and underscores
	stripped := strings.ReplaceAll(normalized, "-", "")
	stripped = strings.ReplaceAll(stripped, "_", "")
	for k, v := range encodingMap {
		kn := strings.ReplaceAll(k, "-", "")
		kn = strings.ReplaceAll(kn, "_", "")
		if kn == stripped {
			return v, true
		}
	}
	return nil, false
}

// isValidEncoding checks if the encoding name is recognized.
func isValidEncoding(name string) bool {
	_, ok := getEncoding(name)
	return ok
}

// convertEncoding converts a byte string from one encoding to another.
// Returns the converted string and the number of illegal characters encountered.
func convertEncoding(input []byte, fromEnc, toEnc string) ([]byte, int, error) {
	fromNorm := normalizeEncodingName(fromEnc)
	toNorm := normalizeEncodingName(toEnc)

	// Same encoding, no conversion needed
	if fromNorm == toNorm {
		return input, 0, nil
	}

	// Step 1: Decode from source encoding to UTF-8
	utf8Bytes, illegalChars, err := decodeToUTF8(input, fromNorm)
	if err != nil {
		return input, 0, err
	}

	// Step 2: Encode from UTF-8 to target encoding
	result, encIllegal, err := encodeFromUTF8(utf8Bytes, toNorm)
	if err != nil {
		return input, illegalChars, err
	}

	return result, illegalChars + encIllegal, nil
}

// decodeToUTF8 decodes bytes from the given encoding to UTF-8.
func decodeToUTF8(input []byte, encName string) ([]byte, int, error) {
	if encName == "UTF-8" || encName == "UTF8" {
		return input, 0, nil
	}

	// Handle UTF-32 variants
	if strings.HasPrefix(encName, "UTF-32") || strings.HasPrefix(encName, "UCS-4") || strings.HasPrefix(encName, "UCS4") {
		return utf32ToUTF8(input, encName)
	}

	// Handle ASCII
	if encName == "ASCII" {
		illegal := 0
		result := make([]byte, 0, len(input))
		for _, b := range input {
			if b > 127 {
				illegal++
				result = append(result, '?') // substitute
			} else {
				result = append(result, b)
			}
		}
		return result, illegal, nil
	}

	enc, ok := encodingMap[encName]
	if !ok {
		// Try without hyphens
		stripped := strings.ReplaceAll(encName, "-", "")
		stripped = strings.ReplaceAll(stripped, "_", "")
		for k, v := range encodingMap {
			kn := strings.ReplaceAll(k, "-", "")
			kn = strings.ReplaceAll(kn, "_", "")
			if kn == stripped {
				enc = v
				ok = true
				break
			}
		}
	}
	if !ok || enc == nil {
		return input, 0, nil
	}

	decoder := enc.NewDecoder()
	result, err := decoder.Bytes(input)
	if err != nil {
		// Try to handle partial errors
		return input, 1, nil
	}
	return result, 0, nil
}

// encodeFromUTF8 encodes UTF-8 bytes to the given encoding.
func encodeFromUTF8(input []byte, encName string) ([]byte, int, error) {
	if encName == "UTF-8" || encName == "UTF8" {
		return input, 0, nil
	}

	// Handle UTF-32 variants
	if strings.HasPrefix(encName, "UTF-32") || strings.HasPrefix(encName, "UCS-4") || strings.HasPrefix(encName, "UCS4") {
		return utf8ToUTF32(input, encName)
	}

	// Handle ASCII
	if encName == "ASCII" {
		illegal := 0
		result := make([]byte, 0, len(input))
		str := string(input)
		for i := 0; i < len(str); {
			r, size := utf8.DecodeRuneInString(str[i:])
			if r > 127 {
				illegal++
				result = append(result, '?')
			} else {
				result = append(result, byte(r))
			}
			i += size
		}
		return result, illegal, nil
	}

	enc, ok := encodingMap[encName]
	if !ok {
		stripped := strings.ReplaceAll(encName, "-", "")
		stripped = strings.ReplaceAll(stripped, "_", "")
		for k, v := range encodingMap {
			kn := strings.ReplaceAll(k, "-", "")
			kn = strings.ReplaceAll(kn, "_", "")
			if kn == stripped {
				enc = v
				ok = true
				break
			}
		}
	}
	if !ok || enc == nil {
		return input, 0, nil
	}

	encoder := enc.NewEncoder()
	result, err := encoder.Bytes(input)
	if err != nil {
		return input, 1, nil
	}
	return result, 0, nil
}

// utf32ToUTF8 converts UTF-32 encoded bytes to UTF-8.
func utf32ToUTF8(input []byte, encName string) ([]byte, int, error) {
	bigEndian := !strings.HasSuffix(encName, "LE")
	if strings.HasSuffix(encName, "BE") {
		bigEndian = true
	}

	// Check for BOM at the beginning if generic UTF-32/UCS-4
	if (encName == "UTF-32" || encName == "UCS-4") && len(input) >= 4 {
		if input[0] == 0xFF && input[1] == 0xFE && input[2] == 0x00 && input[3] == 0x00 {
			bigEndian = false
			input = input[4:]
		} else if input[0] == 0x00 && input[1] == 0x00 && input[2] == 0xFE && input[3] == 0xFF {
			bigEndian = true
			input = input[4:]
		}
	}

	if len(input)%4 != 0 {
		// Truncate to multiple of 4
		input = input[:len(input)/4*4]
	}

	var result []byte
	illegal := 0
	for i := 0; i+3 < len(input); i += 4 {
		var codepoint uint32
		if bigEndian {
			codepoint = uint32(input[i])<<24 | uint32(input[i+1])<<16 | uint32(input[i+2])<<8 | uint32(input[i+3])
		} else {
			codepoint = uint32(input[i+3])<<24 | uint32(input[i+2])<<16 | uint32(input[i+1])<<8 | uint32(input[i])
		}

		r := rune(codepoint)
		if !utf8.ValidRune(r) || codepoint > 0x10FFFF {
			illegal++
			result = append(result, 0xEF, 0xBF, 0xBD) // U+FFFD
		} else {
			buf := make([]byte, 4)
			n := utf8.EncodeRune(buf, r)
			result = append(result, buf[:n]...)
		}
	}
	return result, illegal, nil
}

// utf8ToUTF32 converts UTF-8 to UTF-32 encoding.
func utf8ToUTF32(input []byte, encName string) ([]byte, int, error) {
	bigEndian := !strings.HasSuffix(encName, "LE")
	if strings.HasSuffix(encName, "BE") {
		bigEndian = true
	}

	str := string(input)
	result := make([]byte, 0, utf8.RuneCountInString(str)*4)
	illegal := 0

	for _, r := range str {
		cp := uint32(r)
		if bigEndian {
			result = append(result, byte(cp>>24), byte(cp>>16), byte(cp>>8), byte(cp))
		} else {
			result = append(result, byte(cp), byte(cp>>8), byte(cp>>16), byte(cp>>24))
		}
	}
	return result, illegal, nil
}

// getCanonicalEncodingName returns the canonical name for an encoding.
func getCanonicalEncodingName(name string) string {
	upper := normalizeEncodingName(name)
	// Return canonical forms
	switch upper {
	case "UTF8":
		return "UTF-8"
	case "UTF16":
		return "UTF-16"
	case "UTF16BE":
		return "UTF-16BE"
	case "UTF16LE":
		return "UTF-16LE"
	case "UTF32":
		return "UTF-32"
	case "UTF32BE":
		return "UTF-32BE"
	case "UTF32LE":
		return "UTF-32LE"
	case "EUCJP", "EUCJP-WIN", "EUC-JP-WIN":
		return "EUC-JP"
	case "EUCKR":
		return "EUC-KR"
	case "SJIS", "SHIFT-JIS", "SJIS-WIN":
		return "SJIS"
	case "SHIFT_JIS":
		return "SJIS"
	case "BIG-5":
		return "Big5"
	case "BIG5":
		return "Big5"
	case "LATIN1":
		return "ISO-8859-1"
	}
	// Check if it's a known encoding
	if _, ok := getEncoding(upper); ok {
		return upper
	}
	return name
}

// isCheckEncodingValid checks if a string is valid for the given encoding.
func isCheckEncodingValid(s string, encName string) bool {
	normalized := normalizeEncodingName(encName)

	switch normalized {
	case "UTF-8", "UTF8":
		return utf8.ValidString(s)
	case "ASCII":
		for i := 0; i < len(s); i++ {
			if s[i] > 127 {
				return false
			}
		}
		return true
	case "ISO-8859-1", "LATIN1":
		// ISO-8859-1 accepts all byte values 0-255
		return true
	default:
		// For other encodings, try to decode and re-encode
		enc, ok := getEncoding(normalized)
		if !ok {
			return false
		}
		if enc == nil {
			// Specially handled encodings (UTF-32, etc.)
			if strings.HasPrefix(normalized, "UTF-32") || strings.HasPrefix(normalized, "UCS-4") {
				return len(s)%4 == 0
			}
			return true
		}
		// Try round-trip: decode then encode
		decoder := enc.NewDecoder()
		_, err := decoder.Bytes([]byte(s))
		return err == nil
	}
}

// mbStrlen returns the character count for a string in the given encoding.
func mbStrlen(s string, encName string) int {
	normalized := normalizeEncodingName(encName)

	switch normalized {
	case "UTF-8", "UTF8":
		return utf8.RuneCountInString(s)
	case "ASCII", "ISO-8859-1", "LATIN1",
		"WINDOWS-1250", "WINDOWS-1251", "WINDOWS-1252", "WINDOWS-1253", "WINDOWS-1254",
		"WINDOWS-1255", "WINDOWS-1256", "WINDOWS-1257", "WINDOWS-1258",
		"CP1250", "CP1251", "CP1252", "CP1253", "CP1254",
		"CP1255", "CP1256", "CP1257", "CP1258",
		"ISO-8859-2", "ISO-8859-3", "ISO-8859-4", "ISO-8859-5",
		"ISO-8859-6", "ISO-8859-7", "ISO-8859-8", "ISO-8859-9",
		"ISO-8859-10", "ISO-8859-13", "ISO-8859-14", "ISO-8859-15", "ISO-8859-16",
		"KOI8-R", "KOI8-U", "CP850", "CP866":
		// Single-byte encodings: 1 byte = 1 character
		return len(s)
	case "UTF-16BE", "UTF-16LE", "UCS-2", "UCS-2BE", "UCS-2LE":
		return len(s) / 2
	case "UTF-32BE", "UTF-32LE", "UCS-4BE", "UCS-4LE", "UTF-32", "UCS-4":
		return len(s) / 4
	default:
		// For multi-byte encodings, decode to UTF-8 and count runes
		utf8Bytes, _, _ := decodeToUTF8([]byte(s), normalized)
		return utf8.RuneCount(utf8Bytes)
	}
}
