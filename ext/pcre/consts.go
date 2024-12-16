package pcre

import "github.com/MagicalTux/goro/core/phpv"

// > const
const (
	PREG_PATTERN_ORDER     phpv.ZInt = 1
	PREG_SET_ORDER         phpv.ZInt = 2
	PREG_OFFSET_CAPTURE    phpv.ZInt = (1 << 8)
	PREG_UNMATCHED_AS_NULL phpv.ZInt = (1 << 9)

	PREG_SPLIT_NO_EMPTY       phpv.ZInt = (1 << 0)
	PREG_SPLIT_DELIM_CAPTURE  phpv.ZInt = (1 << 1)
	PREG_SPLIT_OFFSET_CAPTURE phpv.ZInt = (1 << 2)

	PREG_REPLACE_EVAL phpv.ZInt = (1 << 0)

	PREG_GREP_INVERT phpv.ZInt = (1 << 0)
)
