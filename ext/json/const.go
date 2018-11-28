package json

//go:generate stringer -type=JsonError,JsonEncOpt -output stringer.go

type JsonEncOpt int

const (
	HexTag JsonEncOpt = 1 << iota
	HexAmp
	HexApos
	HexQuot
	ForceObject
	NumericCheck
	UnescapedSlashes
	PrettyPrint
	UnescapedUnicode
	PartialOutputOnError
	PreserveZeroFraction
	UnescapedEOL
)

type JsonDecOpt int

const (
	ObjectAsArray JsonDecOpt = 1 << iota
	BigintAsString
)

const (
	InvalidUtf8Ignore     = 0x100000
	InvalidUtf8Substitute = 0x200000
)

type JsonError int

const (
	ErrNone JsonError = iota
	ErrDepth
	ErrStateMismatch
	ErrCtrlChar
	ErrSyntax
	ErrUtf8
	ErrRecursion
	ErrInfOrNan
	ErrUnsupportedType
	ErrInvalidPropName
	ErrUtf16
)

//> const JSON_HEX_TAG: phpv.ZInt(HexTag)
//> const JSON_HEX_AMP: phpv.ZInt(HexAmp)
//> const JSON_HEX_APOS: phpv.ZInt(HexApos)
//> const JSON_HEX_QUOT: phpv.ZInt(HexQuot)
//> const JSON_FORCE_OBJECT: phpv.ZInt(ForceObject)
//> const JSON_NUMERIC_CHECK: phpv.ZInt(NumericCheck)
//> const JSON_UNESCAPED_SLASHES: phpv.ZInt(UnescapedSlashes)
//> const JSON_PRETTY_PRINT: phpv.ZInt(PrettyPrint)
//> const JSON_UNESCAPED_UNICODE: phpv.ZInt(UnescapedUnicode)
//> const JSON_PARTIAL_OUTPUT_ON_ERROR: phpv.ZInt(PartialOutputOnError)
//> const JSON_PRESERVE_ZERO_FRACTION: phpv.ZInt(PreserveZeroFraction)
//> const JSON_UNESCAPED_LINE_TERMINATORS: phpv.ZInt(UnescapedEOL)
//> const JSON_OBJECT_AS_ARRAY: phpv.ZInt(ObjectAsArray)
//> const JSON_BIGINT_AS_STRING: phpv.ZInt(BigintAsString)
//> const JSON_INVALID_UTF8_IGNORE: phpv.ZInt(InvalidUtf8Ignore)
//> const JSON_INVALID_UTF8_SUBSTITUTE: phpv.ZInt(InvalidUtf8Substitute)
//> const JSON_ERROR_NONE: phpv.ZInt(ErrNone)
//> const JSON_ERROR_DEPTH: phpv.ZInt(ErrDepth)
//> const JSON_ERROR_STATE_MISMATCH: phpv.ZInt(ErrStateMismatch)
//> const JSON_ERROR_CTRL_CHAR: phpv.ZInt(ErrCtrlChar)
//> const JSON_ERROR_SYNTAX: phpv.ZInt(ErrSyntax)
//> const JSON_ERROR_UTF8: phpv.ZInt(ErrUtf8)
//> const JSON_ERROR_RECURSION: phpv.ZInt(ErrRecursion)
//> const JSON_ERROR_INF_OR_NAN: phpv.ZInt(ErrInfOrNan)
//> const JSON_ERROR_UNSUPPORTED_TYPE: phpv.ZInt(ErrUnsupportedType)
//> const JSON_ERROR_INVALID_PROPERTY_NAME: phpv.ZInt(ErrInvalidPropName)
//> const JSON_ERROR_UTF16: phpv.ZInt(ErrUtf16)
