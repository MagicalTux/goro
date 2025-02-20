package locale

// #include <locale.h>
import (
	"C"
)

import "github.com/MagicalTux/goro/core/phpv"

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
