package standard

import (
	"unicode"

	"github.com/MagicalTux/goro/core"
)

// translated from sourcefrog's strnatcmp.c
// https://github.com/sourcefrog/natsort/blob/master/strnatcmp.c
func natCmp(a, b []byte, caseSensitive bool) int {
	ai := 0
	bi := 0
	for {
		// Check for end of strings
		aEnd := ai >= len(a)
		bEnd := bi >= len(b)
		if aEnd && bEnd {
			return 0
		}
		if aEnd {
			return -1
		}
		if bEnd {
			return 1
		}

		ca := a[ai]
		cb := b[bi]

		// skip over leading spaces
		for unicode.IsSpace(rune(ca)) {
			if ai+1 < len(a) {
				ai++
				ca = a[ai]
			} else {
				break
			}
		}
		for bi < len(b) && unicode.IsSpace(rune(cb)) {
			if bi+1 < len(b) {
				bi++
				cb = b[bi]
			} else {
				break
			}
		}

		// process run of digits
		if unicode.IsDigit(rune(ca)) && unicode.IsDigit(rune(cb)) {
			fractional := ca == '0' || cb == '0'

			if fractional {
				result := natCmpLeft(a[ai:], b[bi:])
				if result != 0 {
					return result
				}
			} else {
				result := natCmpRight(a[ai:], b[bi:])
				if result != 0 {
					return result
				}
			}
		}

		// Re-check bounds after digit processing may have advanced
		if ai >= len(a) && bi >= len(b) {
			return 0
		}
		if ai >= len(a) {
			return -1
		}
		if bi >= len(b) {
			return 1
		}
		ca = a[ai]
		cb = b[bi]

		if !caseSensitive {
			ca = bytesUpperCase(ca)
			cb = bytesUpperCase(cb)
		}

		if ca < cb {
			return -1
		}
		if ca > cb {
			return +1
		}

		ai++
		bi++
	}
}

func natCmpRight(a, b []byte) int {
	bias := 0

	// The longest run of digits wins.  That aside, the greatest
	// value wins, but we can'*t know that it will until we've scanned
	// both numbers to know that they have the same magnitude, so we
	// remember it in BIAS.
	for i := range max(len(a), len(b)) {
		ca := core.Idx(a, i)
		cb := core.Idx(b, i)

		aDigit := unicode.IsDigit(rune(ca))
		bDigit := unicode.IsDigit(rune(cb))

		if !aDigit && !bDigit {
			return bias
		}
		if !aDigit {
			return -1
		}
		if !bDigit {
			return +1
		}

		if ca < cb {
			if bias == 0 {
				bias = -1
			}
		} else if ca > cb {
			if bias == 0 {
				bias = +1
			}
		} else if ca == 0 && cb == 0 {
			return bias
		}
	}

	return 0
}

func natCmpLeft(a, b []byte) int {
	// Compare two left-aligned numbers: the first to have a
	// different value wins.
	for i := range max(len(a), len(b)) {
		ca := core.Idx(a, i)
		cb := core.Idx(b, i)

		aDigit := unicode.IsDigit(rune(ca))
		bDigit := unicode.IsDigit(rune(cb))

		if !aDigit && !bDigit {
			return 0
		}
		if !aDigit {
			return -1
		}
		if !bDigit {
			return +1
		}

		if ca < cb {
			return -1
		}
		if ca > cb {
			return +1
		}
	}

	return 0
}
