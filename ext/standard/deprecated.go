package standard

import (
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool get_magic_quotes_gpc ( void )
func getMagicQuotesGpc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZBool(false).ZVal(), nil
}

// > func bool get_magic_quotes_runtime ( void )
func getMagicQuotesRuntime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZBool(false).ZVal(), nil
}
