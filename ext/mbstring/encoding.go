package mbstring

import (
	"encoding/base64"
	"fmt"
	"html"
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

// Special encoding sentinel values (we use nil for these in the map and handle them in code)
// These are recognized encoding names that need special handling rather than using x/text codecs.

// encodingMap maps PHP-style encoding names (uppercased) to Go text encodings.
// nil entries are handled specially (ASCII, UTF-32, deprecated encodings, etc.)
var encodingMap map[string]encoding.Encoding

func init() {
	encodingMap = map[string]encoding.Encoding{
		// ASCII is a subset of UTF-8 -- use special handling
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

		// 7bit and 8bit pseudo-encodings
		"7BIT": nil, // handled specially (like ASCII)
		"8BIT": nil, // handled specially (pass-through)
		"BYTE": nil, // alias for 8bit

		// Deprecated pseudo-encodings
		"HTML-ENTITIES": nil, // handled specially
		"QPRINT":        nil, // handled specially (Quoted-Printable)
		"BASE64":        nil, // handled specially
		"UUENCODE":      nil, // handled specially

		// UTF-7 variants
		"UTF-7":      nil, // handled specially
		"UTF-7-IMAP": nil, // handled specially
		"UTF7-IMAP":  nil, // handled specially
		"UTF7":       nil, // handled specially

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
		"LATIN1":    charmap.ISO8859_1,
		"LATIN2":    charmap.ISO8859_2,
		"LATIN3":    charmap.ISO8859_3,
		"LATIN4":    charmap.ISO8859_4,
		"LATIN5":    charmap.ISO8859_9,
		"LATIN6":    charmap.ISO8859_10,
		"CYRILLIC":  charmap.ISO8859_5,
		"ARABIC":    charmap.ISO8859_6,
		"GREEK":     charmap.ISO8859_7,
		"HEBREW":    charmap.ISO8859_8,
		"LATIN-9":   charmap.ISO8859_15,
		"LATIN9":    charmap.ISO8859_15,
		"ISO8859-1": charmap.ISO8859_1,
		"ISO88591":  charmap.ISO8859_1,

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
		"CP-1250":      charmap.Windows1250,
		"CP-1251":      charmap.Windows1251,
		"CP-1252":      charmap.Windows1252,
		"CP-1253":      charmap.Windows1253,
		"CP-1254":      charmap.Windows1254,
		"CP-1255":      charmap.Windows1255,
		"CP-1256":      charmap.Windows1256,
		"CP-1257":      charmap.Windows1257,
		"CP-1258":      charmap.Windows1258,

		// DOS code pages
		"CP850":  charmap.CodePage850,
		"CP866":  charmap.CodePage866,
		"IBM866": charmap.CodePage866,

		// KOI8
		"KOI8-R": charmap.KOI8R,
		"KOI8-U": charmap.KOI8U,
		"KOI8R":  charmap.KOI8R,
		"KOI8U":  charmap.KOI8U,

		// Japanese
		"EUC-JP":       japanese.EUCJP,
		"EUCJP":        japanese.EUCJP,
		"EUCJP-WIN":    japanese.EUCJP,
		"EUC-JP-WIN":   japanese.EUCJP,
		"SJIS":         japanese.ShiftJIS,
		"SHIFT_JIS":    japanese.ShiftJIS,
		"SHIFT-JIS":    japanese.ShiftJIS,
		"CP932":        japanese.ShiftJIS,
		"SJIS-WIN":     japanese.ShiftJIS,
		"ISO-2022-JP":  japanese.ISO2022JP,
		"JIS":          japanese.ISO2022JP,
		"EUC-JP-2004":  japanese.EUCJP,  // approximate
		"EUCJP-2004":   japanese.EUCJP,  // approximate
		"SJIS-2004":    japanese.ShiftJIS, // approximate
		"EUCJP-MS":     japanese.EUCJP,  // approximate
		"EUC-JP-MS":    japanese.EUCJP,  // approximate
		"SJIS-MAC":     japanese.ShiftJIS, // approximate
		"MACJAPANESE":  japanese.ShiftJIS, // approximate
		"SJIS-MOBILE#DOCOMO":   japanese.ShiftJIS, // approximate
		"SJIS-MOBILE#KDDI":     japanese.ShiftJIS, // approximate
		"SJIS-MOBILE#SOFTBANK": japanese.ShiftJIS, // approximate
		"CP51932":      japanese.EUCJP,  // approximate
		"CP50220":      japanese.ISO2022JP, // approximate
		"CP50221":      japanese.ISO2022JP, // approximate
		"CP50222":      japanese.ISO2022JP, // approximate
		"CP5022X":      japanese.ISO2022JP, // approximate
		"ISO-2022-JP-2004":       japanese.ISO2022JP, // approximate
		"ISO-2022-JP-MS":         japanese.ISO2022JP, // approximate
		"ISO-2022-JP-KDDI":       japanese.ISO2022JP, // approximate
		"ISO-2022-JP-MOBILE#KDDI": japanese.ISO2022JP, // approximate
		"JIS-MS":                  japanese.ISO2022JP, // approximate

		// Korean
		"EUC-KR":       korean.EUCKR,
		"EUCKR":        korean.EUCKR,
		"UHC":          korean.EUCKR,
		"ISO-2022-KR":  nil, // handled specially (approximate as EUC-KR for now)

		// Chinese
		"BIG5":       traditionalchinese.Big5,
		"BIG-5":      traditionalchinese.Big5,
		"CP950":      traditionalchinese.Big5,
		"GB18030":    simplifiedchinese.GB18030,
		"GB18030-2022": simplifiedchinese.GB18030,
		"GB2312":     simplifiedchinese.HZGB2312,
		"GBK":        simplifiedchinese.GBK,
		"CP936":      simplifiedchinese.GBK,
		"EUC-CN":     simplifiedchinese.GBK,
		"EUCCN":      simplifiedchinese.GBK,
		"HZ":         simplifiedchinese.HZGB2312,

		// Mac encodings
		"MACINTOSH": charmap.Macintosh,
		"MAC":       charmap.Macintosh,

		// ARMSCII-8 (Armenian) - not directly available in x/text
		"ARMSCII-8": nil, // handled specially
		"ARMSCII8":  nil, // handled specially

		// US-ASCII alias
		"US-ASCII":              nil, // same as ASCII
		"ANSI_X3.4-1968":       nil, // same as ASCII
		"ANSI_X3.4-1986":       nil, // same as ASCII
		"ISO_646.IRV:1991":     nil, // same as ASCII
		"ISO646-US":            nil, // same as ASCII

		// UTF-8 mobile variants (approximate as UTF-8)
		"UTF-8-MOBILE#DOCOMO":   encoding.Nop,
		"UTF-8-MOBILE#KDDI-A":   encoding.Nop,
		"UTF-8-MOBILE#KDDI-B":   encoding.Nop,
		"UTF-8-MOBILE#SOFTBANK": encoding.Nop,
	}
}

// isSpeciallyHandledEncoding checks if an encoding name needs special handling
// (not just a simple nil = ASCII case)
func isSpeciallyHandledEncoding(name string) bool {
	switch name {
	case "ASCII", "US-ASCII", "ANSI_X3.4-1968", "ANSI_X3.4-1986",
		"ISO_646.IRV:1991", "ISO646-US":
		return true
	case "7BIT":
		return true
	case "8BIT", "BYTE":
		return true
	case "HTML-ENTITIES":
		return true
	case "QPRINT":
		return true
	case "BASE64":
		return true
	case "UUENCODE":
		return true
	case "UTF-7", "UTF7", "UTF-7-IMAP", "UTF7-IMAP":
		return true
	case "ISO-2022-KR":
		return true
	case "ARMSCII-8", "ARMSCII8":
		return true
	}
	if strings.HasPrefix(name, "UTF-32") || strings.HasPrefix(name, "UCS-4") || strings.HasPrefix(name, "UCS4") {
		return true
	}
	return false
}

// isDeprecatedEncoding checks if the encoding is deprecated in PHP 8.x
// Note: 7bit, 8bit are deprecated for mb_check_encoding but not for mb_convert_encoding.
func isDeprecatedEncoding(name string) bool {
	switch name {
	case "HTML-ENTITIES", "QPRINT", "BASE64", "UUENCODE":
		return true
	}
	return false
}

// isDeprecatedEncodingForCheck checks if the encoding triggers deprecation for mb_check_encoding.
// In PHP 8.x, only HTML-ENTITIES triggers deprecation warnings for mb_check_encoding.
// 7bit, 8bit, BASE64, etc. are accepted without deprecation warnings.
func isDeprecatedEncodingForCheck(name string) bool {
	switch name {
	case "HTML-ENTITIES":
		return true
	}
	return false
}

// deprecationMessage returns the deprecation message for a deprecated encoding.
// Note: do NOT include the function name prefix -- the ctx.Deprecated() framework adds it.
func deprecationMessage(encName string) string {
	switch encName {
	case "HTML-ENTITIES":
		return "Handling HTML entities via mbstring is deprecated; use htmlspecialchars, htmlentities, or mb_encode_numericentity/mb_decode_numericentity instead"
	case "BASE64":
		return "Handling Base64 via mbstring is deprecated; use base64_encode/base64_decode instead"
	case "QPRINT":
		return "Handling QPrint via mbstring is deprecated; use quoted_printable_encode/quoted_printable_decode instead"
	case "UUENCODE":
		return "Handling Uuencode via mbstring is deprecated; use convert_uuencode/convert_uudecode instead"
	case "7BIT", "8BIT", "BYTE":
		return "Handling " + encName + " via mbstring is deprecated; use mb_convert_encoding()/mb_detect_encoding() with a more specific encoding instead"
	}
	return ""
}

// normalizeEncodingName normalizes a PHP encoding name for lookup.
func normalizeEncodingName(name string) string {
	upper := strings.ToUpper(strings.TrimSpace(name))
	// Handle common variations
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
	case "UTF7":
		return "UTF-7"
	case "UTF7-IMAP", "UTF-7IMAP":
		return "UTF-7-IMAP"
	case "US-ASCII", "USASCII", "ANSI_X3.4-1968", "ANSI_X3.4-1986",
		"ISO_646.IRV:1991", "ISO646-US":
		return "ASCII"
	case "HTML", "HTML-ENTITIES", "HTMLENTITIES":
		return "HTML-ENTITIES"
	case "QUOTED-PRINTABLE", "QUOTEDPRINTABLE", "QPRINT":
		return "QPRINT"
	case "UUENCODE", "UU":
		return "UUENCODE"
	case "7BIT":
		return "7BIT"
	case "8BIT":
		return "8BIT"
	case "BYTE", "BINARY":
		return "8BIT"
	case "CP-1251":
		return "CP1251"
	case "CP-1252":
		return "CP1252"
	case "CP-1250":
		return "CP1250"
	case "CP-1253":
		return "CP1253"
	case "CP-1254":
		return "CP1254"
	case "CP-1255":
		return "CP1255"
	case "CP-1256":
		return "CP1256"
	case "CP-1257":
		return "CP1257"
	case "CP-1258":
		return "CP1258"
	case "CYRILLIC":
		return "ISO-8859-5"
	case "ARABIC":
		return "ISO-8859-6"
	case "GREEK":
		return "ISO-8859-7"
	case "HEBREW":
		return "ISO-8859-8"
	case "LATIN-9", "LATIN9":
		return "ISO-8859-15"
	case "JIS":
		return "ISO-2022-JP"
	case "EUCJP-MS", "EUC-JP-MS":
		return "EUCJP-MS"
	case "EUCJP-2004", "EUC-JP-2004":
		return "EUC-JP-2004"
	case "SJIS-2004":
		return "SJIS-2004"
	case "SJIS-MAC", "MACJAPANESE":
		return "SJIS-MAC"
	case "CP5022X":
		return "CP5022X"
	case "GB18030-2022":
		return "GB18030-2022"
	}
	return upper
}

// getEncoding returns the Go encoding for a PHP encoding name.
// Returns nil, true if the encoding is known but handled specially (ASCII, UTF-32, etc.).
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

	// Same encoding, no conversion needed (with special handling for UTF-8 self-conversion)
	if fromNorm == toNorm {
		// For UTF-8 to UTF-8, we should still validate and fix invalid sequences
		if fromNorm == "UTF-8" || fromNorm == "UTF8" {
			if utf8.Valid(input) {
				return input, 0, nil
			}
			// Fix invalid UTF-8 sequences
			return fixInvalidUTF8(input)
		}
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

// fixInvalidUTF8 replaces invalid UTF-8 sequences with the replacement character
func fixInvalidUTF8(input []byte) ([]byte, int, error) {
	var result []byte
	illegal := 0
	i := 0
	for i < len(input) {
		r, size := utf8.DecodeRune(input[i:])
		if r == utf8.RuneError && size == 1 {
			illegal++
			result = append(result, 0xEF, 0xBF, 0xBD) // U+FFFD
			i++
		} else {
			result = append(result, input[i:i+size]...)
			i += size
		}
	}
	return result, illegal, nil
}

// decodeToUTF8 decodes bytes from the given encoding to UTF-8.
func decodeToUTF8(input []byte, encName string) ([]byte, int, error) {
	if encName == "UTF-8" || encName == "UTF8" {
		if utf8.Valid(input) {
			return input, 0, nil
		}
		return fixInvalidUTF8(input)
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

	// Handle 7bit (same as ASCII)
	if encName == "7BIT" {
		illegal := 0
		result := make([]byte, 0, len(input))
		for _, b := range input {
			if b > 127 {
				illegal++
				result = append(result, '?')
			} else {
				result = append(result, b)
			}
		}
		return result, illegal, nil
	}

	// Handle 8bit (pass-through, all byte values are valid)
	// 8bit means raw bytes -- no conversion to UTF-8.
	// We return the bytes as-is; they'll be passed directly to the encoder.
	if encName == "8BIT" || encName == "BYTE" {
		return input, 0, nil
	}

	// Handle HTML-ENTITIES
	if encName == "HTML-ENTITIES" {
		return htmlEntitiesToUTF8(input)
	}

	// Handle BASE64
	if encName == "BASE64" {
		decoded, err := base64.StdEncoding.DecodeString(string(input))
		if err != nil {
			// Try with padding tolerance
			s := strings.TrimRight(string(input), "\r\n")
			decoded, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "="))
			if err != nil {
				return input, 1, nil
			}
		}
		return decoded, 0, nil
	}

	// Handle QPRINT (Quoted-Printable)
	if encName == "QPRINT" {
		return quotedPrintableDecode(input), 0, nil
	}

	// Handle UUENCODE
	if encName == "UUENCODE" {
		return uudecode(input), 0, nil
	}

	// Handle UTF-7
	if encName == "UTF-7" || encName == "UTF7" {
		return utf7Decode(input), 0, nil
	}

	// Handle UTF-7-IMAP
	if encName == "UTF-7-IMAP" || encName == "UTF7-IMAP" {
		return utf7ImapDecode(input), 0, nil
	}

	// Handle ISO-2022-KR (approximate as EUC-KR)
	if encName == "ISO-2022-KR" {
		return decodeToUTF8(input, "EUC-KR")
	}

	// Handle ARMSCII-8 (pass through for now, since there's no Go support)
	if encName == "ARMSCII-8" || encName == "ARMSCII8" {
		return input, 0, nil
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

	// Handle 7bit (same as ASCII)
	if encName == "7BIT" {
		return encodeFromUTF8(input, "ASCII")
	}

	// Handle 8bit (pass-through, raw bytes)
	if encName == "8BIT" || encName == "BYTE" {
		return input, 0, nil
	}

	// Handle HTML-ENTITIES
	if encName == "HTML-ENTITIES" {
		return utf8ToHTMLEntities(input), 0, nil
	}

	// Handle BASE64
	if encName == "BASE64" {
		encoded := base64.StdEncoding.EncodeToString(input)
		// PHP wraps base64 at 76 characters with CRLF
		if len(encoded) > 76 {
			var wrapped strings.Builder
			for i := 0; i < len(encoded); i += 76 {
				end := i + 76
				if end > len(encoded) {
					end = len(encoded)
				}
				if i > 0 {
					wrapped.WriteString("\r\n")
				}
				wrapped.WriteString(encoded[i:end])
			}
			return []byte(wrapped.String()), 0, nil
		}
		return []byte(encoded), 0, nil
	}

	// Handle QPRINT (Quoted-Printable)
	if encName == "QPRINT" {
		return quotedPrintableEncode(input), 0, nil
	}

	// Handle UUENCODE
	if encName == "UUENCODE" {
		return uuencode(input), 0, nil
	}

	// Handle UTF-7
	if encName == "UTF-7" || encName == "UTF7" {
		return utf7Encode(input), 0, nil
	}

	// Handle UTF-7-IMAP
	if encName == "UTF-7-IMAP" || encName == "UTF7-IMAP" {
		return utf7ImapEncode(input), 0, nil
	}

	// Handle ISO-2022-KR (approximate as EUC-KR)
	if encName == "ISO-2022-KR" {
		return encodeFromUTF8(input, "EUC-KR")
	}

	// Handle ARMSCII-8 (pass through for now)
	if encName == "ARMSCII-8" || encName == "ARMSCII8" {
		return input, 0, nil
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

// htmlEntitiesToUTF8 converts HTML entities to UTF-8
func htmlEntitiesToUTF8(input []byte) ([]byte, int, error) {
	s := string(input)
	// Use Go's html.UnescapeString which handles named and numeric entities
	result := html.UnescapeString(s)
	return []byte(result), 0, nil
}

// utf8ToHTMLEntities converts UTF-8 to HTML numeric entities for non-ASCII chars
func utf8ToHTMLEntities(input []byte) []byte {
	var result strings.Builder
	str := string(input)
	for _, r := range str {
		if r > 127 {
			result.WriteString(fmt.Sprintf("&#%d;", r))
		} else {
			result.WriteRune(r)
		}
	}
	return []byte(result.String())
}

// quotedPrintableDecode decodes Quoted-Printable encoding
func quotedPrintableDecode(input []byte) []byte {
	var result []byte
	for i := 0; i < len(input); i++ {
		if input[i] == '=' && i+2 < len(input) {
			if input[i+1] == '\r' && i+2 < len(input) && input[i+2] == '\n' {
				i += 2 // soft line break
				continue
			}
			if input[i+1] == '\n' {
				i++ // soft line break
				continue
			}
			h1 := hexVal(input[i+1])
			h2 := hexVal(input[i+2])
			if h1 >= 0 && h2 >= 0 {
				result = append(result, byte(h1<<4|h2))
				i += 2
				continue
			}
		}
		result = append(result, input[i])
	}
	return result
}

func hexVal(b byte) int {
	if b >= '0' && b <= '9' {
		return int(b - '0')
	}
	if b >= 'a' && b <= 'f' {
		return int(b - 'a' + 10)
	}
	if b >= 'A' && b <= 'F' {
		return int(b - 'A' + 10)
	}
	return -1
}

// quotedPrintableEncode encodes to Quoted-Printable
func quotedPrintableEncode(input []byte) []byte {
	var result []byte
	lineLen := 0
	for _, b := range input {
		if b == '\r' || b == '\n' {
			result = append(result, b)
			lineLen = 0
		} else if (b >= 33 && b <= 126 && b != '=') || b == '\t' || b == ' ' {
			if lineLen >= 75 {
				result = append(result, '=', '\r', '\n')
				lineLen = 0
			}
			result = append(result, b)
			lineLen++
		} else {
			if lineLen >= 73 {
				result = append(result, '=', '\r', '\n')
				lineLen = 0
			}
			result = append(result, '=')
			result = append(result, "0123456789ABCDEF"[b>>4])
			result = append(result, "0123456789ABCDEF"[b&0xf])
			lineLen += 3
		}
	}
	return result
}

// uudecode decodes uuencoded data
func uudecode(input []byte) []byte {
	lines := strings.Split(string(input), "\n")
	var result []byte
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if len(line) == 0 {
			continue
		}
		if strings.HasPrefix(line, "begin ") {
			continue
		}
		if line == "end" || line == "`" {
			break
		}
		n := int(line[0]) - 32
		if n <= 0 || n > 45 {
			continue
		}
		chars := line[1:]
		for i := 0; i < n; i++ {
			idx := i / 3 * 4
			pos := i % 3
			if idx+3 >= len(chars) {
				break
			}
			a := (int(chars[idx]) - 32) & 0x3F
			b := (int(chars[idx+1]) - 32) & 0x3F
			c := (int(chars[idx+2]) - 32) & 0x3F
			d := (int(chars[idx+3]) - 32) & 0x3F
			switch pos {
			case 0:
				result = append(result, byte((a<<2)|(b>>4)))
			case 1:
				result = append(result, byte(((b&0xF)<<4)|(c>>2)))
			case 2:
				result = append(result, byte(((c&0x3)<<6)|d))
			}
		}
	}
	return result
}

// uuencode encodes data in uuencode format
func uuencode(input []byte) []byte {
	var result strings.Builder
	for i := 0; i < len(input); i += 45 {
		end := i + 45
		if end > len(input) {
			end = len(input)
		}
		chunk := input[i:end]
		result.WriteByte(byte(len(chunk) + 32))
		for j := 0; j < len(chunk); j += 3 {
			b := make([]byte, 3)
			copy(b, chunk[j:])
			result.WriteByte(byte((int(b[0])>>2)&0x3F) + 32)
			result.WriteByte(byte(((int(b[0])&0x3)<<4)|((int(b[1])>>4)&0xF)) + 32)
			result.WriteByte(byte(((int(b[1])&0xF)<<2)|((int(b[2])>>6)&0x3)) + 32)
			result.WriteByte(byte(int(b[2])&0x3F) + 32)
		}
		result.WriteByte('\n')
	}
	result.WriteString("`\n")
	return []byte(result.String())
}

// UTF-7 encode/decode (simplified)
func utf7Decode(input []byte) []byte {
	var result []byte
	i := 0
	for i < len(input) {
		if input[i] == '+' {
			j := i + 1
			for j < len(input) && input[j] != '-' {
				j++
			}
			if j == i+1 {
				// +- means literal +
				result = append(result, '+')
			} else {
				// Decode modified base64
				b64 := string(input[i+1 : j])
				decoded := modifiedBase64Decode(b64)
				// decoded is UTF-16BE
				for k := 0; k+1 < len(decoded); k += 2 {
					cp := uint16(decoded[k])<<8 | uint16(decoded[k+1])
					if cp >= 0xD800 && cp <= 0xDBFF && k+3 < len(decoded) {
						// Surrogate pair
						lo := uint16(decoded[k+2])<<8 | uint16(decoded[k+3])
						r := rune(((uint32(cp) - 0xD800) << 10) | (uint32(lo) - 0xDC00) + 0x10000)
						buf := make([]byte, 4)
						n := utf8.EncodeRune(buf, r)
						result = append(result, buf[:n]...)
						k += 2
					} else {
						buf := make([]byte, 4)
						n := utf8.EncodeRune(buf, rune(cp))
						result = append(result, buf[:n]...)
					}
				}
			}
			if j < len(input) && input[j] == '-' {
				j++
			}
			i = j
		} else {
			result = append(result, input[i])
			i++
		}
	}
	return result
}

func utf7Encode(input []byte) []byte {
	var result []byte
	str := string(input)
	var pendingUTF16 []byte

	flushPending := func() {
		if len(pendingUTF16) > 0 {
			b64 := modifiedBase64Encode(pendingUTF16)
			result = append(result, '+')
			result = append(result, []byte(b64)...)
			result = append(result, '-')
			pendingUTF16 = nil
		}
	}

	for _, r := range str {
		if r >= 0x20 && r <= 0x7E && r != '+' {
			flushPending()
			result = append(result, byte(r))
		} else if r == '+' {
			flushPending()
			result = append(result, '+', '-')
		} else if r == '\r' || r == '\n' || r == '\t' {
			flushPending()
			result = append(result, byte(r))
		} else {
			// Encode as UTF-16BE
			if r <= 0xFFFF {
				pendingUTF16 = append(pendingUTF16, byte(r>>8), byte(r))
			} else {
				// Surrogate pair
				r -= 0x10000
				hi := 0xD800 + (r>>10)&0x3FF
				lo := 0xDC00 + r&0x3FF
				pendingUTF16 = append(pendingUTF16, byte(hi>>8), byte(hi), byte(lo>>8), byte(lo))
			}
		}
	}
	flushPending()
	return result
}

// UTF-7-IMAP encode/decode
func utf7ImapDecode(input []byte) []byte {
	var result []byte
	i := 0
	for i < len(input) {
		if input[i] == '&' {
			j := i + 1
			for j < len(input) && input[j] != '-' {
				j++
			}
			if j == i+1 {
				// &- means literal &
				result = append(result, '&')
			} else {
				// Decode modified UTF-7 base64 (uses , instead of /)
				b64 := strings.ReplaceAll(string(input[i+1:j]), ",", "/")
				decoded := modifiedBase64Decode(b64)
				// decoded is UTF-16BE
				for k := 0; k+1 < len(decoded); k += 2 {
					cp := uint16(decoded[k])<<8 | uint16(decoded[k+1])
					if cp >= 0xD800 && cp <= 0xDBFF && k+3 < len(decoded) {
						lo := uint16(decoded[k+2])<<8 | uint16(decoded[k+3])
						r := rune(((uint32(cp) - 0xD800) << 10) | (uint32(lo) - 0xDC00) + 0x10000)
						buf := make([]byte, 4)
						n := utf8.EncodeRune(buf, r)
						result = append(result, buf[:n]...)
						k += 2
					} else {
						buf := make([]byte, 4)
						n := utf8.EncodeRune(buf, rune(cp))
						result = append(result, buf[:n]...)
					}
				}
			}
			if j < len(input) && input[j] == '-' {
				j++
			}
			i = j
		} else {
			result = append(result, input[i])
			i++
		}
	}
	return result
}

func utf7ImapEncode(input []byte) []byte {
	var result []byte
	str := string(input)
	var pendingUTF16 []byte

	flushPending := func() {
		if len(pendingUTF16) > 0 {
			b64 := modifiedBase64Encode(pendingUTF16)
			b64 = strings.ReplaceAll(b64, "/", ",")
			result = append(result, '&')
			result = append(result, []byte(b64)...)
			result = append(result, '-')
			pendingUTF16 = nil
		}
	}

	for _, r := range str {
		if r >= 0x20 && r <= 0x7E && r != '&' {
			flushPending()
			result = append(result, byte(r))
		} else if r == '&' {
			flushPending()
			result = append(result, '&', '-')
		} else {
			// Encode as UTF-16BE
			if r <= 0xFFFF {
				pendingUTF16 = append(pendingUTF16, byte(r>>8), byte(r))
			} else {
				r -= 0x10000
				hi := 0xD800 + (r>>10)&0x3FF
				lo := 0xDC00 + r&0x3FF
				pendingUTF16 = append(pendingUTF16, byte(hi>>8), byte(hi), byte(lo>>8), byte(lo))
			}
		}
	}
	flushPending()
	return result
}

// modifiedBase64Decode decodes modified base64 (no padding) used in UTF-7
func modifiedBase64Decode(s string) []byte {
	// Add padding
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	return decoded
}

// modifiedBase64Encode encodes to modified base64 (no padding) used in UTF-7
func modifiedBase64Encode(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	// Remove trailing padding
	return strings.TrimRight(encoded, "=")
}

// getCanonicalEncodingName returns the canonical name for an encoding.
func getCanonicalEncodingName(name string) string {
	upper := normalizeEncodingName(name)
	// Return canonical forms matching PHP's naming conventions
	switch upper {
	case "UTF8", "UTF-8":
		return "UTF-8"
	case "UTF16", "UTF-16":
		return "UTF-16"
	case "UTF-16BE":
		return "UTF-16BE"
	case "UTF-16LE":
		return "UTF-16LE"
	case "UTF32", "UTF-32":
		return "UTF-32"
	case "UTF-32BE":
		return "UTF-32BE"
	case "UTF-32LE":
		return "UTF-32LE"
	case "UTF-7", "UTF7":
		return "UTF-7"
	case "UTF-7-IMAP", "UTF7-IMAP":
		return "UTF-7-IMAP"
	case "EUCJP", "EUCJP-WIN", "EUC-JP-WIN", "EUC-JP":
		return "EUC-JP"
	case "EUC-JP-2004", "EUCJP-2004":
		return "EUC-JP-2004"
	case "EUCJP-MS", "EUC-JP-MS":
		return "eucJP-win"
	case "EUCKR", "EUC-KR":
		return "EUC-KR"
	case "SJIS", "SHIFT-JIS", "SJIS-WIN", "SHIFT_JIS":
		return "SJIS"
	case "SJIS-2004":
		return "SJIS-2004"
	case "SJIS-MAC", "MACJAPANESE":
		return "SJIS-mac"
	case "BIG-5", "BIG5":
		return "Big5"
	case "LATIN1":
		return "ISO-8859-1"
	case "LATIN2":
		return "ISO-8859-2"
	case "LATIN3":
		return "ISO-8859-3"
	case "LATIN4":
		return "ISO-8859-4"
	case "LATIN5":
		return "ISO-8859-9"
	case "LATIN6":
		return "ISO-8859-10"
	case "CYRILLIC":
		return "ISO-8859-5"
	case "ARABIC":
		return "ISO-8859-6"
	case "GREEK":
		return "ISO-8859-7"
	case "HEBREW":
		return "ISO-8859-8"
	case "LATIN-9", "LATIN9":
		return "ISO-8859-15"

	// Windows code pages: PHP returns "Windows-XXXX" as canonical name
	case "CP1250", "CP-1250":
		return "Windows-1250"
	case "CP1251", "CP-1251":
		return "Windows-1251"
	case "CP1252", "CP-1252":
		return "Windows-1252"
	case "CP1253", "CP-1253":
		return "Windows-1253"
	case "CP1254", "CP-1254":
		return "Windows-1254"
	case "CP1255", "CP-1255":
		return "Windows-1255"
	case "CP1256", "CP-1256":
		return "Windows-1256"
	case "CP1257", "CP-1257":
		return "Windows-1257"
	case "CP1258", "CP-1258":
		return "Windows-1258"
	case "WINDOWS-1250":
		return "Windows-1250"
	case "WINDOWS-1251":
		return "Windows-1251"
	case "WINDOWS-1252":
		return "Windows-1252"
	case "WINDOWS-1253":
		return "Windows-1253"
	case "WINDOWS-1254":
		return "Windows-1254"
	case "WINDOWS-1255":
		return "Windows-1255"
	case "WINDOWS-1256":
		return "Windows-1256"
	case "WINDOWS-1257":
		return "Windows-1257"
	case "WINDOWS-1258":
		return "Windows-1258"

	case "ASCII", "US-ASCII", "ANSI_X3.4-1968", "ANSI_X3.4-1986",
		"ISO_646.IRV:1991", "ISO646-US":
		return "ASCII"
	case "7BIT":
		return "7bit"
	case "8BIT", "BYTE":
		return "8bit"
	case "HTML-ENTITIES":
		return "HTML-ENTITIES"
	case "QPRINT":
		return "QPRINT"
	case "BASE64":
		return "BASE64"
	case "UUENCODE":
		return "UUENCODE"
	case "JIS", "ISO-2022-JP":
		return "ISO-2022-JP"
	case "ISO-2022-KR":
		return "ISO-2022-KR"
	case "CP932":
		return "CP932"
	case "CP51932":
		return "CP51932"
	case "CP50220":
		return "CP50220"
	case "CP50221":
		return "CP50221"
	case "CP50222":
		return "CP50222"
	case "CP5022X":
		return "CP50220"
	case "CP936":
		return "CP936"
	case "CP950":
		return "CP950"
	case "CP850":
		return "CP850"
	case "CP866":
		return "CP866"
	case "GB18030-2022":
		return "GB18030"
	case "ISO-2022-JP-2004":
		return "ISO-2022-JP-2004"
	case "ISO-2022-JP-MS":
		return "ISO-2022-JP-MS"
	case "ISO-2022-JP-KDDI", "ISO-2022-JP-MOBILE#KDDI":
		return "ISO-2022-JP-KDDI"
	case "ARMSCII-8", "ARMSCII8":
		return "ArmSCII-8"
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
	case "7BIT":
		// 7bit: only bytes 0-127 valid
		for i := 0; i < len(s); i++ {
			if s[i] > 127 {
				return false
			}
		}
		return true
	case "8BIT", "BYTE":
		// 8bit: all byte values valid
		return true
	case "ISO-8859-1", "LATIN1":
		// ISO-8859-1 accepts all byte values 0-255
		return true
	case "HTML-ENTITIES":
		// HTML-ENTITIES: always valid (deprecated)
		return true
	case "QPRINT", "BASE64", "UUENCODE":
		// These are always considered valid for check_encoding (deprecated)
		return true
	case "UTF-7", "UTF7", "UTF-7-IMAP", "UTF7-IMAP":
		// For now, accept valid-looking UTF-7
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
	case "ASCII", "ISO-8859-1", "LATIN1", "7BIT", "8BIT", "BYTE",
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
