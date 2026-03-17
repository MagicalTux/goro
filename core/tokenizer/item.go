package tokenizer

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpv"
)

//go:generate stringer -type=ItemType
type ItemType int

const (
	itemError ItemType = iota
	T_EOF
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
	T_COALESCE_EQUAL           // "??="
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
	T_ENUM // "enum"
	T_EVAL // eval()
	T_EXIT // "exit" or "die"
	T_EXTENDS
	T_FILE // "__FILE__"
	T_FINAL
	T_FINALLY // "finally", for exceptions
	T_FN      // "fn" (arrow functions)
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
	T_MATCH        // "match"
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
	T_NULLSAFE_OBJECT_OPERATOR // ?->
	T_OBJECT_OPERATOR          // ->
	T_OPEN_TAG
	T_OPEN_TAG_WITH_ECHO // "<?="
	T_OR_EQUAL           // |=
	T_PIPE               // |>
	T_PLUS_EQUAL         // +=
	T_POW                // **
	T_POW_EQUAL
	T_PRINT
	T_PRIVATE
	T_PUBLIC
	T_PROTECTED
	T_READONLY // "readonly"
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
	T_ATTRIBUTE  // #[
	T_VOID_CAST  // "(void)"
	itemMax
)

type Item struct {
	Type       ItemType
	Data       string
	Filename   string
	Line, Char int
}

func (i *Item) Errorf(format string, arg ...interface{}) error {
	e := fmt.Sprintf(format, arg...)
	return fmt.Errorf("%s in %s on line %d", e, i.Filename, i.Line)
}

func (i *Item) String() string {
	return i.Type.Name()
}

func (i ItemType) Name() string {
	if i > itemMax {
		return string([]rune{'\'', i.Rune(), '\''})
	}
	return i.String()
}

// OpString returns the operator symbol for use in error messages.
// For compound assignment operators (+=, -= etc.), returns the base operator (+, - etc.)
// since PHP error messages like "Unsupported operand types" use the base operator.
func (i ItemType) OpString() string {
	// Map compound assignment operators to their base operator
	switch i {
	case T_PLUS_EQUAL:
		return "+"
	case T_MINUS_EQUAL:
		return "-"
	case T_MUL_EQUAL:
		return "*"
	case T_DIV_EQUAL:
		return "/"
	case T_MOD_EQUAL:
		return "%"
	case T_POW_EQUAL:
		return "**"
	case T_SL_EQUAL:
		return "<<"
	case T_SR_EQUAL:
		return ">>"
	case T_AND_EQUAL:
		return "&"
	case T_OR_EQUAL:
		return "|"
	case T_XOR_EQUAL:
		return "^"
	case T_CONCAT_EQUAL:
		return "."
	}
	if i > itemMax {
		return string(i.Rune())
	}
	for s, t := range lexPhpOps {
		if t == i {
			return s
		}
	}
	// Single-char operators
	switch {
	case i == Rune('+'):
		return "+"
	case i == Rune('-'):
		return "-"
	case i == Rune('*'):
		return "*"
	case i == Rune('/'):
		return "/"
	case i == Rune('%'):
		return "%"
	}
	return i.String()
}

func (i *Item) Rune() rune {
	return i.Type.Rune()
}

func (i ItemType) Rune() rune {
	if i < itemMax {
		return rune(0)
	}
	return rune(i - itemMax)
}

func (i *Item) IsSingle(r rune) bool {
	if i.Type < itemMax {
		return false
	}
	return i.Type == ItemType(r)+itemMax
}

// IsLabel returns true if the item represents a PHP label/identifier,
// including keywords. Used for named arguments where keywords are valid names.
func (i *Item) IsLabel() bool {
	if i.Type == T_STRING {
		return true
	}
	// Keywords are also valid as named argument names in PHP 8.0
	if i.Type >= itemMax {
		return false // single-character token
	}
	// Check if this token type was produced from a keyword label
	return i.Data != "" && labelType(i.Data) != T_STRING
}

func (i *Item) IsExpressionEnd() bool {
	// T_CLOSE_TAG is acceptable to end an expression
	return i.IsSingle(';') || i.Type == T_CLOSE_TAG
}

// IsSemiReserved returns true if the token is a semi-reserved keyword that can
// be used as a class constant name, method name, or property name in PHP.
// See https://www.php.net/manual/en/reserved.other-reserved-words.php
func (i *Item) IsSemiReserved() bool {
	switch i.Type {
	case T_STRING:
		return true
	case T_ABSTRACT, T_ARRAY, T_AS, T_BREAK, T_CALLABLE, T_CASE, T_CATCH,
		T_CLASS, T_CLONE, T_CONST, T_CONTINUE, T_DECLARE, T_DEFAULT,
		T_DO, T_ECHO, T_ELSE, T_ELSEIF, T_EMPTY, T_ENDDECLARE,
		T_ENDFOR, T_ENDFOREACH, T_ENDIF, T_ENDSWITCH, T_ENDWHILE,
		T_EVAL, T_EXIT, T_EXTENDS, T_FINAL, T_FINALLY, T_FN,
		T_FOR, T_FOREACH, T_FUNCTION, T_GLOBAL, T_GOTO, T_IF,
		T_IMPLEMENTS, T_INCLUDE, T_INCLUDE_ONCE, T_INSTANCEOF,
		T_INSTEADOF, T_INTERFACE, T_ISSET, T_LIST, T_LOGICAL_AND,
		T_LOGICAL_OR, T_LOGICAL_XOR, T_MATCH, T_NAMESPACE, T_NEW,
		T_PRINT, T_PRIVATE, T_PROTECTED, T_PUBLIC, T_READONLY,
		T_REQUIRE, T_REQUIRE_ONCE, T_RETURN, T_STATIC, T_SWITCH,
		T_THROW, T_TRAIT, T_TRY, T_UNSET, T_USE, T_VAR, T_WHILE,
		T_YIELD, T_YIELD_FROM, T_ENUM,
		T_CLASS_C, T_TRAIT_C, T_FUNC_C, T_METHOD_C, T_LINE, T_FILE,
		T_DIR, T_NS_C:
		return true
	}
	return false
}

func (i *Item) Unexpected() error {
	err := fmt.Errorf("syntax error, unexpected %s", i.HumanName())
	return &phpv.PhpError{
		Err:          err,
		Code:         phpv.E_PARSE,
		Loc:          i.Loc(),
		GoStackTrace: phpv.GetGoDebugTrace(),
	}
}

// UnexpectedExpecting returns a parse error with the "expecting" hint.
func (i *Item) UnexpectedExpecting(expecting string) error {
	err := fmt.Errorf("syntax error, unexpected %s, expecting %s", i.HumanName(), expecting)
	return &phpv.PhpError{
		Err:          err,
		Code:         phpv.E_PARSE,
		Loc:          i.Loc(),
		GoStackTrace: phpv.GetGoDebugTrace(),
	}
}

// HumanName returns the PHP 8-style human-readable token name for error messages.
// For keywords, returns `token "keyword"` (e.g. `token "exit"`).
// For symbols, returns `"c"` (e.g. `"("`).
// For special tokens, returns their description (e.g. `end of file`).
func (i *Item) HumanName() string {
	if i.Type > itemMax {
		return fmt.Sprintf("token \"%c\"", i.Type.Rune())
	}
	switch i.Type {
	case T_EOF:
		return "end of file"
	case T_LNUMBER:
		return fmt.Sprintf("integer \"%s\"", i.Data)
	case T_DNUMBER:
		return fmt.Sprintf("floating-point number \"%s\"", i.Data)
	case T_STRING:
		return fmt.Sprintf("identifier \"%s\"", i.Data)
	case T_VARIABLE:
		return fmt.Sprintf("variable \"%s\"", i.Data)
	case T_CONSTANT_ENCAPSED_STRING:
		return fmt.Sprintf("string content \"%s\"", i.Data)
	case T_INLINE_HTML:
		return "inline HTML"
	}
	// For keywords and other tokens, use the label map to get the keyword name
	name := tokenHumanName(i.Type)
	if name != "" {
		return fmt.Sprintf("token \"%s\"", name)
	}
	return i.Type.Name()
}

// tokenHumanName returns the human-readable keyword for a token type,
// matching PHP 8's error message format.
func tokenHumanName(t ItemType) string {
	switch t {
	case T_ABSTRACT:
		return "abstract"
	case T_ARRAY:
		return "array"
	case T_AS:
		return "as"
	case T_BREAK:
		return "break"
	case T_CALLABLE:
		return "callable"
	case T_CASE:
		return "case"
	case T_CATCH:
		return "catch"
	case T_CLASS:
		return "class"
	case T_CLONE:
		return "clone"
	case T_CONST:
		return "const"
	case T_CONTINUE:
		return "continue"
	case T_DECLARE:
		return "declare"
	case T_DEFAULT:
		return "default"
	case T_DO:
		return "do"
	case T_ECHO:
		return "echo"
	case T_ELSE:
		return "else"
	case T_ELSEIF:
		return "elseif"
	case T_EMPTY:
		return "empty"
	case T_ENDDECLARE:
		return "enddeclare"
	case T_ENDFOR:
		return "endfor"
	case T_ENDFOREACH:
		return "endforeach"
	case T_ENDIF:
		return "endif"
	case T_ENDSWITCH:
		return "endswitch"
	case T_ENDWHILE:
		return "endwhile"
	case T_ENUM:
		return "enum"
	case T_EVAL:
		return "eval"
	case T_EXIT:
		return "exit"
	case T_EXTENDS:
		return "extends"
	case T_FINAL:
		return "final"
	case T_FINALLY:
		return "finally"
	case T_FN:
		return "fn"
	case T_FOR:
		return "for"
	case T_FOREACH:
		return "foreach"
	case T_FUNCTION:
		return "function"
	case T_GLOBAL:
		return "global"
	case T_GOTO:
		return "goto"
	case T_IF:
		return "if"
	case T_IMPLEMENTS:
		return "implements"
	case T_INCLUDE:
		return "include"
	case T_INCLUDE_ONCE:
		return "include_once"
	case T_INSTANCEOF:
		return "instanceof"
	case T_INSTEADOF:
		return "insteadof"
	case T_INTERFACE:
		return "interface"
	case T_ISSET:
		return "isset"
	case T_LIST:
		return "list"
	case T_MATCH:
		return "match"
	case T_NAMESPACE:
		return "namespace"
	case T_NEW:
		return "new"
	case T_PRINT:
		return "print"
	case T_PRIVATE:
		return "private"
	case T_PROTECTED:
		return "protected"
	case T_PUBLIC:
		return "public"
	case T_READONLY:
		return "readonly"
	case T_REQUIRE:
		return "require"
	case T_REQUIRE_ONCE:
		return "require_once"
	case T_RETURN:
		return "return"
	case T_STATIC:
		return "static"
	case T_SWITCH:
		return "switch"
	case T_THROW:
		return "throw"
	case T_TRAIT:
		return "trait"
	case T_TRY:
		return "try"
	case T_UNSET:
		return "unset"
	case T_USE:
		return "use"
	case T_VAR:
		return "var"
	case T_WHILE:
		return "while"
	case T_YIELD:
		return "yield"
	case T_YIELD_FROM:
		return "yield from"
	case T_INT_CAST:
		return "(int)"
	case T_DOUBLE_CAST:
		return "(double)"
	case T_STRING_CAST:
		return "(string)"
	case T_ARRAY_CAST:
		return "(array)"
	case T_OBJECT_CAST:
		return "(object)"
	case T_BOOL_CAST:
		return "(bool)"
	case T_UNSET_CAST:
		return "(unset)"
	case T_VOID_CAST:
		return "(void)"
	case T_DOUBLE_ARROW:
		return "=>"
	case T_PAAMAYIM_NEKUDOTAYIM:
		return "::"
	case T_ELLIPSIS:
		return "..."
	case T_NS_SEPARATOR:
		return "\\"
	case T_ATTRIBUTE:
		return "#["
	case T_PIPE:
		return "|>"
	}
	return ""
}

func (i *Item) Loc() *phpv.Loc {
	return &phpv.Loc{Filename: i.Filename, Line: i.Line, Char: i.Char}
}

func Rune(r rune) ItemType {
	return ItemType(r) + itemMax
}

func Type(t ItemType) rune {
	c := rune(t - itemMax)
	return c
}
