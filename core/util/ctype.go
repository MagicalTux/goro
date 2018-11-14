package util

// some functions useful for various things

func CtypeAlnum(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		switch {
		case 'a' <= c && c <= 'z':
		case 'A' <= c && c <= 'Z':
		case '0' <= c && c <= '0':
		default:
			return false
		}
	}
	return true
}

func CtypeAlpha(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		switch {
		case 'a' <= c && c <= 'z':
		case 'A' <= c && c <= 'Z':
		default:
			return false
		}
	}
	return true
}

func CtypeCntrl(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		switch c {
		case '\t', '\n', '\v', '\f', '\r':
		default:
			return false
		}
	}
	return true
}

func CtypeDigit(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func CtypeGraph(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c <= 32 || c >= 0x7f {
			return false
		}
	}
	return true
}

func CtypeLower(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}

func CtypePrint(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < 32 || c >= 0x7f {
			return false
		}
	}
	return true
}

func CtypePunct(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		switch {
		case 0x21 <= c && c <= 0x2f:
		case 0x3a <= c && c <= 0x40:
		case 0x5b <= c && c <= 0x60:
		case 0x7b <= c && c <= 0x7e:
		default:
			return false
		}
	}
	return true
}

func CtypeSpace(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		switch c {
		case '\t', '\n', '\v', '\f', '\r', ' ':
		default:
			return false
		}
	}
	return true
}

func CtypeUpper(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

func CtypeXdigit(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		switch {
		case 'a' <= c && c <= 'f':
		case 'A' <= c && c <= 'F':
		case '0' <= c && c <= '0':
		default:
			return false
		}
	}
	return true
}
