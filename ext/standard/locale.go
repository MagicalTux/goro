package standard

import (
	"os"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/locale"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string setlocale ( int $category , string $locale [, string $... ] )
func fncSetLocale(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// TODO: add a global per-process mutex to this functions
	var category phpv.ZInt
	var localeArg *phpv.ZVal
	_, err := core.Expand(ctx, args, &category, &localeArg)
	if err != nil {
		return nil, err
	}

	var locales []phpv.ZString
	switch localeArg.GetType() {
	case phpv.ZtArray:
		for _, elem := range localeArg.AsArray(ctx).Iterate(ctx) {
			locales = append(locales, elem.AsString(ctx))
		}
	case phpv.ZtString:
		locale := localeArg.AsString(ctx)
		locales = append(locales, locale)
	default:
		return nil, ctx.Errorf("expected string or array")
	}

	for i := 2; i < len(args); i++ {
		var v *phpv.ZVal
		err := core.ExpandAt(ctx, args, i, &v)
		if err != nil {
			return nil, err
		}

		switch v.GetType() {
		case phpv.ZtString:
			locales = append(locales, v.AsString(ctx))
		case phpv.ZtArray:
			for _, elem := range v.AsArray(ctx).Iterate(ctx) {
				locales = append(locales, elem.AsString(ctx))
			}
		default:
			return nil, ctx.Errorf("expected string or array")
		}
	}

	for _, lc := range locales {
		if lc == "" {
			var envName string
			switch category {
			case locale.LC_CTYPE:
				envName = "LC_CTYPE"
			case locale.LC_NUMERIC:
				envName = "LC_NUMERIC"
			case locale.LC_TIME:
				envName = "LC_TIME"
			case locale.LC_COLLATE:
				envName = "LC_COLLATE"
			case locale.LC_MONETARY:
				envName = "LC_MONETARY"
			case locale.LC_MESSAGES:
				envName = "LC_MESSAGES"
			case locale.LC_ALL:
				envName = "LC_ALL"
			}
			lc = phpv.ZString(os.Getenv(envName))
		}
		res, ok := locale.SetLocale(category, lc)
		if ok {
			return res.ZVal(), nil
		}
	}

	return phpv.ZFalse.ZVal(), nil
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
