package core

import (
	"math"

	"github.com/MagicalTux/goro/core/phpv"
)

// > const
var NULL = phpv.ZNull{}

// > const
const (
	TRUE                 = phpv.ZBool(true)
	FALSE                = phpv.ZBool(false)
	ZEND_THREAD_SAFE     = phpv.ZBool(true) // consider things thread safe
	PHP_ZTS              = phpv.ZInt(1)
	DEFAULT_INCLUDE_PATH = phpv.ZString(".:")
	DIRECTORY_SEPARATOR  = phpv.ZString("/")
	PHP_EOL              = phpv.ZString("\n")
	PHP_INT_MAX          = phpv.ZInt(math.MaxInt64)
	PHP_INT_MIN          = phpv.ZInt(math.MinInt64)
	PHP_INT_SIZE         = phpv.ZInt(8)

	// standard values
	PHP_FD_SETSIZE    = phpv.ZInt(1024)
	PHP_MAXPATHLEN    = phpv.ZInt(4096)
	PHP_FLOAT_DIG     = phpv.ZInt(15)
	PHP_FLOAT_EPSILON = phpv.ZFloat(2.220446049250313e-16)
	PHP_FLOAT_MAX     = phpv.ZFloat(math.MaxFloat64)
	PHP_FLOAT_MIN     = phpv.ZFloat(math.SmallestNonzeroFloat64)

	E_ERROR             = phpv.ZInt(phpv.E_ERROR)
	E_WARNING           = phpv.ZInt(phpv.E_WARNING)
	E_PARSE             = phpv.ZInt(phpv.E_PARSE)
	E_NOTICE            = phpv.ZInt(phpv.E_NOTICE)
	E_CORE_ERROR        = phpv.ZInt(phpv.E_CORE_ERROR)
	E_CORE_WARNING      = phpv.ZInt(phpv.E_CORE_WARNING)
	E_COMPILE_ERROR     = phpv.ZInt(phpv.E_COMPILE_ERROR)
	E_COMPILE_WARNING   = phpv.ZInt(phpv.E_COMPILE_WARNING)
	E_USER_ERROR        = phpv.ZInt(phpv.E_USER_ERROR)
	E_USER_WARNING      = phpv.ZInt(phpv.E_USER_WARNING)
	E_USER_NOTICE       = phpv.ZInt(phpv.E_USER_NOTICE)
	E_STRICT            = phpv.ZInt(phpv.E_STRICT)
	E_RECOVERABLE_ERROR = phpv.ZInt(phpv.E_RECOVERABLE_ERROR)
	E_DEPRECATED        = phpv.ZInt(phpv.E_DEPRECATED)
	E_USER_DEPRECATED   = phpv.ZInt(phpv.E_USER_DEPRECATED)
	E_ALL               = phpv.ZInt(phpv.E_ALL)
)
