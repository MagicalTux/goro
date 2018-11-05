package tokenizer

import "fmt"

//go:generate stringer -type=ItemType
type ItemType int

const (
	itemError ItemType = iota
	itemEOF
	ItemSingleChar             // : ;, ., >, !, { } etc...
	T_ABSTRACT                 // "abstract"
	T_AND_EQUAL                // "&="
	T_ARRAY                    // array(...
	T_ARRAY_CAST               // "(array)"
	T_AS                       // "as"
	T_BAD_CHARACTER            // anything below ASCII 32 except \t (0x09), \n (0x0a) and \r (0x0d)
	T_BOOLEAN_AND              // "&&"
	T_BOOLEAN_OR               // "||"
	T_BOOL_CAST                // "(bool)" or "(boolean)"
	T_BREAK                    // "break"
	T_CALLABLE                 // "callable"
	T_CASE                     // "case"
	T_CATCH                    // "catch"
	T_CLASS                    // "class"
	T_CLASS_C                  // "__CLASS__"
	T_CLONE                    // "clone"
	T_CLOSE_TAG                // ?> or %>
	T_COALESCE                 // "??"
	T_COMMENT                  // // # or /* */
	T_CONCAT_EQUAL             // .=
	T_CONST                    // const
	T_CONSTANT_ENCAPSED_STRING // string in single or double quotes
	T_CONTINUE                 // continue
	T_CURLY_OPEN               // {$ (in double quote strings) see http://php.net/manual/en/language.types.string.php#language.types.string.parsing.complex
	T_DEC                      // "--"
	T_DECLARE                  // "declare"
	T_DEFAULT                  // "default"
	T_DIR                      // "__DIR__"
	T_DIV_EQUAL                // "/="
	T_DOC_COMMENT              // /** */ comments
	T_DO                       // "do"
	T_DOLLAR_OPEN_CURLY_BRACES // ${ see http://php.net/manual/en/language.types.string.php#language.types.string.parsing.complex
	T_DOUBLE_ARROW             // =>
	T_DOUBLE_CAST              // (real), (double), (float)
	T_PAAMAYIM_NEKUDOTAYIM     // "::"
	T_ECHO                     // "echo"
	T_ELLIPSIS                 // ...
	T_ELSE                     // else
	T_ELSEIF
	T_EMPTY                   // empty()
	T_ENCAPSED_AND_WHITESPACE // ?
	T_ENDDECLARE              // enddeclare
	T_ENDFOR                  // endfor
	T_ENDFOREACH              // endforeach
	T_ENDIF                   // endif
	T_ENDSWITCH               // endswitch
	T_ENDWHILE                // endwhile
	T_END_HEREDOC
	T_EVAL // eval()
	T_EXIT // "exit" or "die"
	T_EXTENDS
	T_FILE // "__FILE__"
	T_FINAL
	T_FINALLY // "finally", for exceptions
	T_FOR
	T_FOREACH
	T_FUNCTION
	T_FUNC_C // "__FUNCTION__"
	T_GLOBAL // global
	T_GOTO
	T_HALT_COMPILER // __halt_compiler()
	T_IF
	T_IMPLEMENTS
	T_INC
	T_INCLUDE
	T_INCLUDE_ONCE
	T_INLINE_HTML // default type
	T_INSTANCEOF
	T_INSTEADOF
	T_INT_CAST // (int) or (integer)
	T_INTERFACE
	T_ISSET
	T_IS_EQUAL            // ==
	T_IS_GREATER_OR_EQUAL // >=
	T_IS_IDENTICAL        // ===
	T_IS_NOT_EQUAL        // != <>
	T_IS_NOT_IDENTICAL    // !==
	T_IS_SMALLER_OR_EQUAL // <=
	T_SPACESHIP           // <=>
	T_LINE                // "__LINE__"
	T_LIST
	T_LOGICAL_AND  // "and"
	T_LOGICAL_OR   // "or"
	T_LOGICAL_XOR  // "xor"
	T_METHOD_C     // "__METHOD__"
	T_MINUS_EQUAL  // -=
	T_MOD_EQUAL    // %=
	T_MUL_EQUAL    // *=
	T_NAMESPACE    // "namespace"
	T_NS_C         // __NAMESPACE__
	T_NS_SEPARATOR // \
	T_NEW
	T_NUM_STRING
	T_OBJECT_CAST
	T_OBJECT_OPERATOR
	T_OPEN_TAG
	T_OPEN_TAG_WITH_ECHO // "<?="
	T_OR_EQUAL           // |=
	T_PLUS_EQUAL         // +=
	T_POW                // **
	T_POW_EQUAL
	T_PRINT
	T_PRIVATE
	T_PUBLIC
	T_PROTECTED
	T_REQUIRE
	T_REQUIRE_ONCE
	T_RETURN
	T_SL // <<
	T_SL_EQUAL
	T_SR // >>
	T_SR_EQUAL
	T_START_HEREDOC
	T_STATIC
	T_STRING      // parent, self, etc. identifiers, e.g. keywords like parent and self, function names, class names and more are matched. See also T_CONSTANT_ENCAPSED_STRING
	T_STRING_CAST // (string)
	T_STRING_VARNAME
	T_SWITCH
	T_THROW
	T_TRAIT
	T_TRAIT_C
	T_TRY
	T_UNSET
	T_UNSET_CAST
	T_USE
	T_VAR
	T_VARIABLE // a variable, eg $foo
	T_WHILE
	T_WHITESPACE
	T_XOR_EQUAL
	T_YIELD
	T_YIELD_FROM
	T_DNUMBER
	T_LNUMBER
)

type Item struct {
	Type       ItemType
	Data       string
	Line, Char int
}

func (i *Item) Errorf(format string, arg ...interface{}) error {
	e := fmt.Sprintf(format, arg...)
	return fmt.Errorf("%s in ? on line %d", e, i.Line)
}

func (i *Item) Unexpected() error {
	return i.Errorf("Unexpected %s", i.Type)
}
