package standard

import "github.com/MagicalTux/gophp/core"

//> func bool get_magic_quotes_gpc ( void )
func getMagicQuotesGpc(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return core.ZBool(false).ZVal(), nil
}

//> func bool get_magic_quotes_runtime ( void )
func getMagicQuotesRuntime(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return core.ZBool(false).ZVal(), nil
}
