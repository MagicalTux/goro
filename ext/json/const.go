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

const (
	ObjectAsArray = 1 << iota
	BigintAsString

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

//> const JSON_HEX_TAG: core.ZInt(HexTag)
//> const JSON_HEX_AMP: core.ZInt(HexAmp)
//> const JSON_HEX_APOS: core.ZInt(HexApos)
//> const JSON_HEX_QUOT: core.ZInt(HexQuot)
//> const JSON_FORCE_OBJECT: core.ZInt(ForceObject)
//> const JSON_NUMERIC_CHECK: core.ZInt(NumericCheck)
//> const JSON_UNESCAPED_SLASHES: core.ZInt(UnescapedSlashes)
//> const JSON_PRETTY_PRINT: core.ZInt(PrettyPrint)
//> const JSON_UNESCAPED_UNICODE: core.ZInt(UnescapedUnicode)
//> const JSON_PARTIAL_OUTPUT_ON_ERROR: core.ZInt(PartialOutputOnError)
//> const JSON_PRESERVE_ZERO_FRACTION: core.ZInt(PreserveZeroFraction)
//> const JSON_UNESCAPED_LINE_TERMINATORS: core.ZInt(UnescapedEOL)
//> const JSON_OBJECT_AS_ARRAY: core.ZInt(ObjectAsArray)
//> const JSON_BIGINT_AS_STRING: core.ZInt(BigintAsString)
//> const JSON_INVALID_UTF8_IGNORE: core.ZInt(InvalidUtf8Ignore)
//> const JSON_INVALID_UTF8_SUBSTITUTE: core.ZInt(InvalidUtf8Substitute)
//> const JSON_ERROR_NONE: core.ZInt(ErrNone)
//> const JSON_ERROR_DEPTH: core.ZInt(ErrDepth)
//> const JSON_ERROR_STATE_MISMATCH: core.ZInt(ErrStateMismatch)
//> const JSON_ERROR_CTRL_CHAR: core.ZInt(ErrCtrlChar)
//> const JSON_ERROR_SYNTAX: core.ZInt(ErrSyntax)
//> const JSON_ERROR_UTF8: core.ZInt(ErrUtf8)
//> const JSON_ERROR_RECURSION: core.ZInt(ErrRecursion)
//> const JSON_ERROR_INF_OR_NAN: core.ZInt(ErrInfOrNan)
//> const JSON_ERROR_UNSUPPORTED_TYPE: core.ZInt(ErrUnsupportedType)
//> const JSON_ERROR_INVALID_PROPERTY_NAME: core.ZInt(ErrInvalidPropName)
//> const JSON_ERROR_UTF16: core.ZInt(ErrUtf16)
