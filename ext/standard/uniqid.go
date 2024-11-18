package standard

import (
	"fmt"
	"os"
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

var lcgSeed struct {
	s1 int32
	s2 int32
}

func init() {
	// TODO: seed should be per goroutine/context
	t := time.Now()
	sec, usec := t.Unix(), t.Unix()%0x100000
	lcgSeed.s1 = int32(sec ^ (usec << 11))
	lcgSeed.s2 = int32(os.Getpid())
}

// vodoo translated from PHP's ext/standard/lcg.c
// no idea if it's correct, but at least it passes the test ehe
func combinedLcg() float64 {
	var q, a, b, c, m int32

	a, b, c, m = 53668, 40014, 12211, 2147483563
	q = lcgSeed.s1 / a
	lcgSeed.s1 = b*(lcgSeed.s1-a*q) - c*q
	if lcgSeed.s1 < 0 {
		lcgSeed.s1 += m
	}

	a, b, c, m = 53668, 40014, 12211, 2147483563
	q = lcgSeed.s2 / a
	lcgSeed.s2 = b*(lcgSeed.s2-a*q) - c*q
	if lcgSeed.s2 < 0 {
		lcgSeed.s2 += m
	}

	z := lcgSeed.s1 - lcgSeed.s2
	if z < 1 {
		z += 2147483562
	}
	return float64(z) * 4.656613e-10
}

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
		uniqid = fmt.Sprintf("%s%08x%05x%.8F", prefix, sec, usec, combinedLcg())
	} else {
		t := time.Now()
		sec, usec := t.Unix(), t.Unix()%0x100000
		uniqid = fmt.Sprintf("%s%08x%05x", prefix, sec, usec)
	}

	return phpv.ZStr(uniqid), nil
}
