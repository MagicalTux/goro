package tokenizer

import (
	"strings"
)

func lexPhpPossibleCast(l *Lexer) lexState {
	// possible (string) etc

	l.next() // "("
	l.acceptSpaces()

	typ := l.acceptPhpLabel()

	l.acceptSpaces()
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

	l.reset() // return to initial state
	l.next()  // "("
	l.emit(ItemSingleChar)

	return l.base
}
