package standard

func deref[T any](ptr *T, defValue T) T {
	if ptr == nil {
		return defValue
	}
	return *ptr
}

func ifElse[T any](cond bool, consequence, alternative T) T {
	if cond {
		return consequence
	}
	return alternative
}
