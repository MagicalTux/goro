package locale

// #include <locale.h>
// #include <langinfo.h>
import (
	"C"
)

import (
	"github.com/MagicalTux/goro/core/phpv"
)

// > const
const (
	LC_CTYPE phpv.ZInt = iota
	LC_NUMERIC
	LC_TIME
	LC_COLLATE
	LC_MONETARY
	LC_MESSAGES
	LC_ALL
)

// > const
const (
	D_T_FMT    phpv.ZInt = 131112
	D_FMT      phpv.ZInt = 131113
	T_FMT      phpv.ZInt = 131114
	T_FMT_AMPM phpv.ZInt = 131115
	AM_STR     phpv.ZInt = 131110
	PM_STR     phpv.ZInt = 131111
	DAY_1      phpv.ZInt = 131079
	DAY_2      phpv.ZInt = 131080
	DAY_3      phpv.ZInt = 131081
	DAY_4      phpv.ZInt = 131082
	DAY_5      phpv.ZInt = 131083
	DAY_6      phpv.ZInt = 131084
	DAY_7      phpv.ZInt = 131085
	ABDAY_1    phpv.ZInt = 131072
	ABDAY_2    phpv.ZInt = 131073
	ABDAY_3    phpv.ZInt = 131074
	ABDAY_4    phpv.ZInt = 131075
	ABDAY_5    phpv.ZInt = 131076
	ABDAY_6    phpv.ZInt = 131077
	ABDAY_7    phpv.ZInt = 131078
	MON_1      phpv.ZInt = 131098
	MON_2      phpv.ZInt = 131099
	MON_3      phpv.ZInt = 131100
	MON_4      phpv.ZInt = 131101
	MON_5      phpv.ZInt = 131102
	MON_6      phpv.ZInt = 131103
	MON_7      phpv.ZInt = 131104
	MON_8      phpv.ZInt = 131105
	MON_9      phpv.ZInt = 131106
	MON_10     phpv.ZInt = 131107
	MON_11     phpv.ZInt = 131108
	MON_12     phpv.ZInt = 131109
	ABMON_1    phpv.ZInt = 131086
	ABMON_2    phpv.ZInt = 131087
	ABMON_3    phpv.ZInt = 131088
	ABMON_4    phpv.ZInt = 131089
	ABMON_5    phpv.ZInt = 131090
	ABMON_6    phpv.ZInt = 131091
	ABMON_7    phpv.ZInt = 131092
	ABMON_8    phpv.ZInt = 131093
	ABMON_9    phpv.ZInt = 131094
	ABMON_10   phpv.ZInt = 131095
	ABMON_11   phpv.ZInt = 131096
	ABMON_12   phpv.ZInt = 131097
	RADIXCHAR  phpv.ZInt = 65536
	THOUSEP    phpv.ZInt = 65537
	YESSTR     phpv.ZInt = 327682
	YESEXPR    phpv.ZInt = 327680
	NOSTR      phpv.ZInt = 327683
	NOEXPR     phpv.ZInt = 327681
	CRNCYSTR   phpv.ZInt = 262159
	CODESET    phpv.ZInt = 14
)

type LconvNumericFormat struct {
	CurrencySymbol string
	FragDigits     int8
	PCsPrecedes    int8
	NCsPrecedes    int8
	PSepBySpace    int8
	NSepBySpace    int8
	PSignPosN      int8
	NSignPosN      int8
}

type Lconv struct {
	DecimalPoint string
	ThousandsSep string
	Grouping     string

	MonDecimalPoint string
	MonThousandsSep string
	MonGrouping     string
	PositiveSign    string
	NegativeSign    string

	LocalFormat LconvNumericFormat
	IntlFormat  LconvNumericFormat
}

func SetLocale(lc phpv.ZInt, locale phpv.ZString) (phpv.ZString, bool) {
	var param *C.char
	if locale != "0" {
		param = C.CString(string(locale))
	}
	ret := C.setlocale(C.int(lc), param)
	if ret == nil || *ret == 0 {
		return "", false
	}
	return phpv.ZString(C.GoString(ret)), true
}

func Localeconv() *Lconv {
	ret := C.localeconv()

	return &Lconv{
		DecimalPoint:    C.GoString(ret.decimal_point),
		ThousandsSep:    C.GoString(ret.thousands_sep),
		Grouping:        C.GoString(ret.grouping),
		MonDecimalPoint: C.GoString(ret.mon_decimal_point),
		MonThousandsSep: C.GoString(ret.mon_thousands_sep),
		MonGrouping:     C.GoString(ret.mon_grouping),
		PositiveSign:    C.GoString(ret.positive_sign),
		NegativeSign:    C.GoString(ret.negative_sign),
		LocalFormat: LconvNumericFormat{
			CurrencySymbol: C.GoString(ret.currency_symbol),
			FragDigits:     int8(ret.frac_digits),
			PCsPrecedes:    int8(ret.p_cs_precedes),
			NCsPrecedes:    int8(ret.n_cs_precedes),
			PSepBySpace:    int8(ret.p_sep_by_space),
			NSepBySpace:    int8(ret.n_sep_by_space),
			PSignPosN:      int8(ret.p_sign_posn),
			NSignPosN:      int8(ret.n_sign_posn),
		},
		IntlFormat: LconvNumericFormat{
			CurrencySymbol: C.GoString(ret.int_curr_symbol),
			FragDigits:     int8(ret.int_frac_digits),
			PCsPrecedes:    int8(ret.int_p_cs_precedes),
			NCsPrecedes:    int8(ret.int_n_cs_precedes),
			PSepBySpace:    int8(ret.int_p_sep_by_space),
			NSepBySpace:    int8(ret.int_n_sep_by_space),
			PSignPosN:      int8(ret.int_p_sign_posn),
			NSignPosN:      int8(ret.int_n_sign_posn),
		},
	}
}

func LangInfo(item phpv.ZInt) phpv.ZString {
	elem := C.GoString(C.nl_langinfo(C.int(item)))
	return phpv.ZString(elem)
}
