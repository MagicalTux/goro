package standard

import "iter"

func deref[T any](ptr *T, defValue T) T {
	if ptr == nil {
		return defValue
	}
	return *ptr
}

// safe-index, returns default(T) if out of bounds
func idx[T any](xs []T, i int) T {
	var x T
	if i >=0 && i < len(xs) {
		x = xs[i]
	}
	return x
}

func ifElse[T any](cond bool, consequence, alternative T) T {
	if cond {
		return consequence
	}
	return alternative
}

func iterateBackwards[T any](xs []T) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i := len(xs) - 1; i >= 0; i-- {
			if !yield(i, xs[i]) {
				break
			}
		}
	}
}
