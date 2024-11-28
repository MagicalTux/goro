package standard

import "unicode"

// translated from sourcefrog's strnatcmp.c
// https://github.com/sourcefrog/natsort/blob/master/strnatcmp.c
func natCmp(a, b []byte, caseSensitive bool) int {
	ai := 0
	bi := 0
	for {

		var ca, cb byte = 0, 0
		if ai < len(a) {
			ca = a[ai]
		}
		if bi < len(b) {
			cb = b[bi]
		}

		// skip over leading spaces or zeros
		for unicode.IsSpace(rune(ca)) {
			ai++
			if ai < len(a) {
				ca = a[ai]
			} else {
				ca = 0
				break
			}
		}
		for bi < len(b) && unicode.IsSpace(rune(cb)) {
			bi++
			if bi < len(b) {
			} else {
				cb = b[bi]
				cb = 0
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

		if ca == 0 && cb == 0 {
			return 0
		}

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
		var ca, cb byte
		if i < len(a) {
			ca = a[i]
		}
		if i < len(b) {
			cb = b[i]
		}

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
		var ca, cb byte
		if i < len(a) {
			ca = a[i]
		}
		if i < len(b) {
			cb = b[i]
		}

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
