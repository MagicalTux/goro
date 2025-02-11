package core

import "iter"

func Deref[T any](ptr *T, defValue T) T {
	if ptr == nil {
		return defValue
	}
	return *ptr
}

// safe-index, returns defaultVal or default(T) if out of bounds
func Idx[T any](xs []T, i int, defaultVal ...T) T {
	if i >= 0 && i < len(xs) {
		return xs[i]
	}

	if len(defaultVal) > 0 {
		return defaultVal[0]
	}

	var x T
	return x
}

// safe-index on strings, returns 0 or defaultVal if out of bounds
func StrIdx(str string, i int, defaultVal ...byte) byte {
	if i >= 0 && i < len(str) {
		return str[i]
	}

	if len(defaultVal) > 0 {
		return defaultVal[0]
	}

	return 0
}

func IfElse[T any](cond bool, consequence, alternative T) T {
	if cond {
		return consequence
	}
	return alternative
}

func IterateBackwards[T any](xs []T) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i := len(xs) - 1; i >= 0; i-- {
			if !yield(i, xs[i]) {
				break
			}
		}
	}
}

func Every[T any](xs []T, predicate func(T) bool) bool {
	for _, x := range xs {
		if !predicate(x) {
			return false
		}
	}
	return true
}
