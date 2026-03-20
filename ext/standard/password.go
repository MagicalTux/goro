package standard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"golang.org/x/crypto/bcrypt"
)

const (
	PASSWORD_BCRYPT_DEFAULT_COST = 12
)

// password_hash() - creates a password hash
func fncPasswordHash(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("password_hash() expects at least 2 arguments, %d given", len(args)))
	}

	// Validate arg 1 is string
	if args[0].GetType() == phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "password_hash(): Argument #1 ($password) must be of type string, array given")
	}

	password, err := args[0].As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}
	passwordStr := string(password.Value().(phpv.ZString))

	// Validate arg 2 type - must be string|int|null
	if args[1].GetType() == phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "password_hash(): Argument #2 ($algo) must be of type string|int|null, array given")
	}

	// Validate arg 3 if present - must be array
	if len(args) >= 3 {
		if args[2].GetType() == phpv.ZtObject {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "password_hash(): Argument #3 ($options) must be of type array, stdClass given")
		}
		if args[2].GetType() != phpv.ZtArray && args[2].GetType() != phpv.ZtNull {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("password_hash(): Argument #3 ($options) must be of type array, %s given", args[2].GetType().String()))
		}
	}

	algoVal := args[1]

	// Determine algorithm
	algo := ""
	if algoVal.GetType() == phpv.ZtNull {
		algo = "2y" // PASSWORD_DEFAULT
	} else if algoVal.GetType() == phpv.ZtInt {
		intVal := int(algoVal.Value().(phpv.ZInt))
		if intVal == 1 {
			algo = "2y"
		} else {
			algo = strconv.Itoa(intVal)
		}
	} else {
		s, _ := algoVal.As(ctx, phpv.ZtString)
		algo = string(s.Value().(phpv.ZString))
	}

	if algo != "2y" {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "password_hash(): Argument #2 ($algo) must be a valid password hashing algorithm")
	}

	// Check for null bytes in password
	if strings.Contains(passwordStr, "\x00") {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "Bcrypt password must not contain null character")
	}

	// Parse options
	cost := PASSWORD_BCRYPT_DEFAULT_COST
	if len(args) >= 3 && args[2] != nil && args[2].GetType() == phpv.ZtArray {
		arr := args[2].Value().(*phpv.ZArray)

		// Check for deprecated salt option
		saltVal, _ := arr.OffsetGet(ctx, phpv.ZStr("salt"))
		if saltVal != nil && saltVal.GetType() != phpv.ZtNull {
			ctx.Warn("The \"salt\" option has been ignored, since providing a custom salt is no longer supported")
		}

		costVal, _ := arr.OffsetGet(ctx, phpv.ZStr("cost"))
		if costVal != nil && costVal.GetType() != phpv.ZtNull {
			costZval, cerr := costVal.As(ctx, phpv.ZtInt)
			if cerr == nil {
				cost = int(costZval.Value().(phpv.ZInt))
			}
		}
	}

	if cost < 4 || cost > 31 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("Invalid bcrypt cost parameter specified: %d", cost))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(passwordStr), cost)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Go's bcrypt uses $2a$ prefix, but PHP uses $2y$
	hashStr := string(hash)
	if strings.HasPrefix(hashStr, "$2a$") {
		hashStr = "$2y$" + hashStr[4:]
	}

	return phpv.ZStr(hashStr), nil
}

// password_verify() - verifies a password against a hash
func fncPasswordVerify(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("password_verify() expects exactly 2 arguments, %d given", len(args)))
	}

	var password phpv.ZString
	var hash phpv.ZString
	_, err := core.Expand(ctx, args, &password, &hash)
	if err != nil {
		return nil, err
	}

	hashStr := string(hash)

	// Short/invalid hash - just return false
	if len(hashStr) < 3 {
		return phpv.ZBool(false).ZVal(), nil
	}

	// Bcrypt verification
	if strings.HasPrefix(hashStr, "$2y$") || strings.HasPrefix(hashStr, "$2a$") || strings.HasPrefix(hashStr, "$2b$") {
		err := bcrypt.CompareHashAndPassword([]byte(hashStr), []byte(password))
		if err == nil {
			return phpv.ZBool(true).ZVal(), nil
		}
		return phpv.ZBool(false).ZVal(), nil
	}

	// DES crypt verification (13-char hash, no $ prefix)
	if !strings.HasPrefix(hashStr, "$") && len(hashStr) == 13 {
		result := cryptDES(string(password), hashStr)
		if result == hashStr {
			return phpv.ZBool(true).ZVal(), nil
		}
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(false).ZVal(), nil
}

// password_get_info() - returns information about a hash
func fncPasswordGetInfo(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var hash phpv.ZString
	_, err := core.Expand(ctx, args, &hash)
	if err != nil {
		return nil, err
	}

	hashStr := string(hash)
	result := phpv.NewZArray()

	if strings.HasPrefix(hashStr, "$2y$") && len(hashStr) == 60 {
		result.OffsetSet(ctx, phpv.ZStr("algo"), phpv.ZStr("2y"))
		result.OffsetSet(ctx, phpv.ZStr("algoName"), phpv.ZStr("bcrypt"))

		options := phpv.NewZArray()
		// Extract cost
		if len(hashStr) > 7 {
			costStr := hashStr[4:6]
			cost, cerr := strconv.Atoi(costStr)
			if cerr == nil {
				options.OffsetSet(ctx, phpv.ZStr("cost"), phpv.ZInt(cost).ZVal())
			}
		}
		result.OffsetSet(ctx, phpv.ZStr("options"), options.ZVal())
	} else {
		result.OffsetSet(ctx, phpv.ZStr("algo"), phpv.ZNULL.ZVal())
		result.OffsetSet(ctx, phpv.ZStr("algoName"), phpv.ZStr("unknown"))
		options := phpv.NewZArray()
		result.OffsetSet(ctx, phpv.ZStr("options"), options.ZVal())
	}

	return result.ZVal(), nil
}

// password_needs_rehash() - checks if a hash needs rehashing
func fncPasswordNeedsRehash(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, fmt.Sprintf("password_needs_rehash() expects at least 2 arguments, %d given", len(args)))
	}

	// Validate arg 1 is string
	if args[0].GetType() == phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "password_needs_rehash(): Argument #1 ($hash) must be of type string, array given")
	}

	hashVal, err := args[0].As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}
	hashStr := string(hashVal.Value().(phpv.ZString))

	// Validate arg 2 type
	if args[1].GetType() == phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "password_needs_rehash(): Argument #2 ($algo) must be of type string|int|null, array given")
	}

	// Validate arg 3 if present
	if len(args) >= 3 {
		if args[2].GetType() != phpv.ZtArray && args[2].GetType() != phpv.ZtNull {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("password_needs_rehash(): Argument #3 ($options) must be of type array, %s given", args[2].GetType().String()))
		}
	}

	// Determine requested algorithm
	algoVal := args[1]
	algo := ""
	if algoVal.GetType() == phpv.ZtNull {
		algo = "2y"
	} else if algoVal.GetType() == phpv.ZtInt {
		intVal := int(algoVal.Value().(phpv.ZInt))
		if intVal == 1 {
			algo = "2y"
		} else {
			algo = strconv.Itoa(intVal)
		}
	} else {
		s, _ := algoVal.As(ctx, phpv.ZtString)
		algo = string(s.Value().(phpv.ZString))
	}

	if algo == "2y" || algo == "1" {
		// Check if hash matches bcrypt format
		if !strings.HasPrefix(hashStr, "$2y$") || len(hashStr) != 60 {
			return phpv.ZBool(true).ZVal(), nil
		}

		// Extract current cost from hash
		currentCost := 0
		if len(hashStr) > 6 {
			costStr := hashStr[4:6]
			currentCost, _ = strconv.Atoi(costStr)
		}

		// Determine requested cost
		requestedCost := PASSWORD_BCRYPT_DEFAULT_COST
		if len(args) >= 3 && args[2] != nil && args[2].GetType() == phpv.ZtArray {
			arr := args[2].Value().(*phpv.ZArray)
			costVal, _ := arr.OffsetGet(ctx, phpv.ZStr("cost"))
			if costVal != nil && costVal.GetType() != phpv.ZtNull {
				costZval, cerr := costVal.As(ctx, phpv.ZtInt)
				if cerr == nil {
					requestedCost = int(costZval.Value().(phpv.ZInt))
				}
			}
		}

		if currentCost != requestedCost {
			return phpv.ZBool(true).ZVal(), nil
		}
		return phpv.ZBool(false).ZVal(), nil
	}

	// PASSWORD_DEFAULT with 0 or null
	if algo == "0" || algo == "" {
		if !strings.HasPrefix(hashStr, "$2y$") || len(hashStr) != 60 {
			return phpv.ZBool(true).ZVal(), nil
		}
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZBool(true).ZVal(), nil
}

// password_algos() - returns available password hashing algorithms
func fncPasswordAlgos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	result := phpv.NewZArray()
	result.OffsetSet(ctx, nil, phpv.ZStr("2y"))
	return result.ZVal(), nil
}
