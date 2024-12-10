package standard

import (
	"fmt"
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string uniqid ( string $string, string $charlist )
func fncUniqId(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var prefixArg *phpv.ZString
	var moreEntropyArg *phpv.ZBool
	_, err := core.Expand(ctx, args, &prefixArg, &moreEntropyArg)
	if err != nil {
		return nil, err
	}
	prefix := ""
	if prefixArg != nil {
		prefix = string(*prefixArg)
	}

	var uniqid string
	if moreEntropyArg != nil && *moreEntropyArg {
		t := time.Now()
		sec, usec := t.Unix(), t.Unix()%0x100000
		r := ctx.Global().Random()
		uniqid = fmt.Sprintf("%s%08x%05x%.8F", prefix, sec, usec, r.Lcg.Next())
	} else {
		t := time.Now()
		sec, usec := t.Unix(), t.UnixMicro()%0x100000
		uniqid = fmt.Sprintf("%s%08x%05x", prefix, sec, usec)
	}

	return phpv.ZStr(uniqid), nil
}
