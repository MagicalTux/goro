package core

//> func int strlen ( string $string )
func fncStrlen(ctx Context, args []*ZVal) (*ZVal, error) {
	var s ZString
	_, err := Expand(ctx, args, &s)
	if err != nil {
		return nil, err
	}

	return ZInt(len(s)).ZVal(), nil
}
