package random

import (
	"os"
	"time"
)

type Lcg struct {
	s1 int32
	s2 int32
}

func NewLcg() *Lcg {
	t := time.Now()
	sec, usec := t.Unix(), t.UnixMicro()%0x100000
	return &Lcg{
		s1: int32(sec ^ (usec << 11)),
		s2: int32(os.Getpid()),
	}
}

func (ls *Lcg) Next() float64 {
	var q, a, b, c, m int32

	a, b, c, m = 53668, 40014, 12211, 2147483563
	q = ls.s1 / a
	ls.s1 = b*(ls.s1-a*q) - c*q
	if ls.s1 < 0 {
		ls.s1 += m
	}

	a, b, c, m = 53668, 40014, 12211, 2147483563
	q = ls.s2 / a
	ls.s2 = b*(ls.s2-a*q) - c*q
	if ls.s2 < 0 {
		ls.s2 += m
	}

	z := ls.s1 - ls.s2
	if z < 1 {
		z += 2147483562
	}
	return float64(z) * 4.656613e-10
}
