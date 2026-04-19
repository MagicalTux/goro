package standard

import (
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/locale"
	"github.com/KarpelesLab/goro/core/phpv"
)

// Longest locale name PHP accepts (matches ext/standard/string.c in upstream
// PHP, which rejects anything longer with a warning).
const maxLocaleNameLen = 255

// > func string setlocale ( int $category , string $locale [, string $... ] )
func fncSetLocale(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var category phpv.ZInt
	var localeArg *phpv.ZVal
	_, err := core.Expand(ctx, args, &category, &localeArg)
	if err != nil {
		return nil, err
	}

	// PHP: setlocale($cat, 0) or setlocale($cat, "0") queries the current
	// locale without changing it.
	if isLocaleQuery(localeArg) {
		res := locale.GetLocale(category)
		if res == "" {
			return phpv.ZFalse.ZVal(), nil
		}
		return res.ZVal(), nil
	}

	// Collect candidate names, in order. PHP tries each in turn and keeps
	// the first that resolves. Accepts a single string, an array of
	// strings, or positional var-args after the first one.
	var candidates []phpv.ZString
	switch localeArg.GetType() {
	case phpv.ZtArray:
		for _, elem := range localeArg.AsArray(ctx).Iterate(ctx) {
			candidates = append(candidates, elem.AsString(ctx))
		}
	case phpv.ZtNull:
		candidates = append(candidates, "")
	default:
		candidates = append(candidates, localeArg.AsString(ctx))
	}

	for i := 2; i < len(args); i++ {
		var v *phpv.ZVal
		if err := core.ExpandAt(ctx, args, i, &v); err != nil {
			return nil, err
		}
		switch v.GetType() {
		case phpv.ZtArray:
			for _, elem := range v.AsArray(ctx).Iterate(ctx) {
				candidates = append(candidates, elem.AsString(ctx))
			}
		case phpv.ZtNull:
			candidates = append(candidates, "")
		default:
			candidates = append(candidates, v.AsString(ctx))
		}
	}

	for _, lc := range candidates {
		if len(lc) > maxLocaleNameLen {
			ctx.Warn("setlocale(): Specified locale name is too long")
			continue
		}
		if res, ok := locale.SetLocale(category, lc); ok {
			return res.ZVal(), nil
		}
	}

	return phpv.ZFalse.ZVal(), nil
}

// isLocaleQuery reports whether the argument is PHP's sentinel for "query
// the current locale": integer 0 or the literal string "0".
func isLocaleQuery(v *phpv.ZVal) bool {
	if v == nil {
		return false
	}
	switch v.GetType() {
	case phpv.ZtInt:
		return v.Value().(phpv.ZInt) == 0
	case phpv.ZtString:
		return v.Value().(phpv.ZString) == "0"
	}
	return false
}

// > func array localeconv ()
func fncLocaleConv(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	conv := locale.Localeconv()
	result := phpv.NewZArray()

	result.OffsetSet(ctx, phpv.ZStr("decimal_point"), phpv.ZStr(conv.DecimalPoint))
	result.OffsetSet(ctx, phpv.ZStr("thousands_sep"), phpv.ZStr(conv.ThousandsSep))
	result.OffsetSet(ctx, phpv.ZStr("grouping"), phpv.ZStr(conv.Grouping))

	result.OffsetSet(ctx, phpv.ZStr("mon_decimal_point"), phpv.ZStr(conv.MonDecimalPoint))
	result.OffsetSet(ctx, phpv.ZStr("mon_thousands_sep"), phpv.ZStr(conv.MonThousandsSep))
	result.OffsetSet(ctx, phpv.ZStr("mon_grouping"), phpv.ZStr(conv.MonGrouping))
	result.OffsetSet(ctx, phpv.ZStr("positive_sign"), phpv.ZStr(conv.PositiveSign))
	result.OffsetSet(ctx, phpv.ZStr("negative_sign"), phpv.ZStr(conv.NegativeSign))

	local := conv.LocalFormat
	result.OffsetSet(ctx, phpv.ZStr("currency_symbol"), phpv.ZStr(local.CurrencySymbol))
	result.OffsetSet(ctx, phpv.ZStr("frac_digits"), phpv.ZInt(local.FragDigits).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("p_cs_precedes"), phpv.ZInt(local.PCsPrecedes).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("n_cs_precedes"), phpv.ZInt(local.NCsPrecedes).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("p_sep_by_space"), phpv.ZInt(local.PSepBySpace).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("n_sep_by_space"), phpv.ZInt(local.NSepBySpace).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("p_sign_posn"), phpv.ZInt(local.PSignPosN).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("n_sign_posn"), phpv.ZInt(local.NSignPosN).ZVal())

	intl := conv.IntlFormat
	result.OffsetSet(ctx, phpv.ZStr("int_curr_symbol"), phpv.ZStr(intl.CurrencySymbol))
	result.OffsetSet(ctx, phpv.ZStr("int_frac_digits"), phpv.ZInt(intl.FragDigits).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("int_p_cs_precedes"), phpv.ZInt(intl.PCsPrecedes).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("int_n_cs_precedes"), phpv.ZInt(intl.NCsPrecedes).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("int_p_sep_by_space"), phpv.ZInt(intl.PSepBySpace).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("int_n_sep_by_space"), phpv.ZInt(intl.NSepBySpace).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("int_p_sign_posn"), phpv.ZInt(intl.PSignPosN).ZVal())
	result.OffsetSet(ctx, phpv.ZStr("int_n_sign_posn"), phpv.ZInt(intl.NSignPosN).ZVal())

	return result.ZVal(), nil
}

// > func string nl_langinfo ( int $item )
func fncNlLangInfo(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var item phpv.ZInt
	_, err := core.Expand(ctx, args, &item)
	if err != nil {
		return nil, err
	}
	return locale.LangInfo(item).ZVal(), nil
}
