package tokenizer

import "strings"

func lexPhpPossibleCast(l *Lexer) lexState {
	// possible (string) etc

	l.next() // "("
	l.acceptSpaces()

	typ := l.acceptPhpLabel()

	l.acceptSpaces()
	if l.accept(")") {

		ltyp := strings.ToLower(typ)
		switch ltyp {
		case "int", "integer":
			l.emitWithData(T_INT_CAST, ltyp)
			return l.base
		case "bool", "boolean":
			l.emitWithData(T_BOOL_CAST, ltyp)
			return l.base
		case "float", "double":
			l.emitWithData(T_DOUBLE_CAST, ltyp)
			return l.base
		case "real":
			l.emitWithData(T_DOUBLE_CAST, "real")
			return l.base
		case "string", "binary":
			l.emitWithData(T_STRING_CAST, ltyp)
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
		case "void":
			l.emit(T_VOID_CAST)
			return l.base
		}
	}

	l.reset() // return to initial state
	l.next()  // "("
	l.emit(Rune('('))

	return l.base
}
