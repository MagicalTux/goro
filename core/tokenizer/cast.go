package tokenizer

import "strings"

func lexPhpPossibleCast(l *Lexer) lexState {
	// possible (string) etc

	l.next() // "("
	sp := l.acceptSpaces()

	typ := l.acceptPhpLabel()

	sp2 := l.acceptSpaces()
	if l.accept(")") {

		switch strings.ToLower(typ) {
		case "int", "integer":
			l.emit(T_INT_CAST)
			return l.base
		case "bool", "boolean":
			l.emit(T_BOOL_CAST)
			return l.base
		case "float", "double", "real":
			l.emit(T_DOUBLE_CAST)
			return l.base
		case "string":
			l.emit(T_STRING_CAST)
			return l.base
		case "array":
			l.emit(T_ARRAY_CAST)
			return l.base
		case "object":
			l.emit(T_OBJECT_CAST)
			return l.base
		case "unset":
			l.emit(T_UNSET_CAST)
			return l.base
		}
	}

	l.ignore() // flush buffer, rebuild cast operator as regular thing
	l.write("(")
	l.emit(ItemSingleChar)
	if sp != "" {
		l.write(sp)
		l.emit(T_WHITESPACE)
	}
	l.write(typ)
	l.emit(labelType(typ))
	if sp2 != "" {
		l.write(sp2)
		l.emit(T_WHITESPACE)
	}
	return l.base
}
