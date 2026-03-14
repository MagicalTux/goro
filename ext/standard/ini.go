package standard

import (
	"bufio"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// INI scanner mode constants
const (
	INI_SCANNER_NORMAL phpv.ZInt = 0
	INI_SCANNER_RAW    phpv.ZInt = 1
	INI_SCANNER_TYPED  phpv.ZInt = 2
)

// > func array|false parse_ini_file ( string $filename [, bool $process_sections = false [, int $scanner_mode = INI_SCANNER_NORMAL ]] )
func fncParseIniFile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var processSections core.Optional[phpv.ZBool]
	var scannerMode core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &filename, &processSections, &scannerMode)
	if err != nil {
		return nil, err
	}

	sections := bool(processSections.GetOrDefault(phpv.ZBool(false)))
	mode := scannerMode.GetOrDefault(INI_SCANNER_NORMAL)

	// Use ctx.Global().OpenFile() for file access
	path := string(filename)
	if len(path) == 0 || path[0] != '/' {
		path = string(ctx.Global().Getwd()) + "/" + path
	}

	rc, err := ctx.Global().OpenFile(ctx, path)
	if err != nil {
		ctx.Warn("parse_ini_file(%s): Failed to open stream: %s", string(filename), err.Error())
		return phpv.ZFalse.ZVal(), nil
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		ctx.Warn("parse_ini_file(%s): Failed to read file: %s", string(filename), err.Error())
		return phpv.ZFalse.ZVal(), nil
	}

	return parseIniString(ctx, string(data), sections, mode)
}

// > func array|false parse_ini_string ( string $ini_string [, bool $process_sections = false [, int $scanner_mode = INI_SCANNER_NORMAL ]] )
func fncParseIniString(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var iniString phpv.ZString
	var processSections core.Optional[phpv.ZBool]
	var scannerMode core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &iniString, &processSections, &scannerMode)
	if err != nil {
		return nil, err
	}

	sections := bool(processSections.GetOrDefault(phpv.ZBool(false)))
	mode := scannerMode.GetOrDefault(INI_SCANNER_NORMAL)

	return parseIniString(ctx, string(iniString), sections, mode)
}

// parseIniString parses an INI-format string and returns a PHP array.
func parseIniString(ctx phpv.Context, data string, processSections bool, mode phpv.ZInt) (*phpv.ZVal, error) {
	result := phpv.NewZArray()
	var currentSection *phpv.ZArray

	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || line[0] == ';' || line[0] == '#' {
			continue
		}

		// Check for section header
		if line[0] == '[' {
			end := strings.IndexByte(line, ']')
			if end == -1 {
				continue
			}
			sectionName := strings.TrimSpace(line[1:end])
			if processSections {
				currentSection = phpv.NewZArray()
				result.OffsetSet(ctx, phpv.ZString(sectionName).ZVal(), currentSection.ZVal())
			}
			continue
		}

		// Parse key = value
		eqIdx := strings.IndexByte(line, '=')
		if eqIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:eqIdx])
		value := strings.TrimSpace(line[eqIdx+1:])

		// Remove surrounding quotes from value
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// Process the value based on scanner mode
		phpVal := iniProcessValue(ctx, value, mode)

		// Check if key has array syntax: key[]
		isArray := false
		arrayKey := ""
		if strings.HasSuffix(key, "[]") {
			isArray = true
			key = key[:len(key)-2]
		} else if idx := strings.IndexByte(key, '['); idx != -1 {
			end := strings.IndexByte(key[idx:], ']')
			if end != -1 {
				arrayKey = key[idx+1 : idx+end]
				key = key[:idx]
				isArray = true
			}
		}

		target := result
		if processSections && currentSection != nil {
			target = currentSection
		}

		if isArray {
			// Get or create the array for this key
			existing, _ := target.OffsetGet(ctx, phpv.ZString(key).ZVal())
			var arr *phpv.ZArray
			if existing != nil && existing.GetType() == phpv.ZtArray {
				arr = existing.Value().(*phpv.ZArray)
			} else {
				arr = phpv.NewZArray()
				target.OffsetSet(ctx, phpv.ZString(key).ZVal(), arr.ZVal())
			}
			if arrayKey == "" {
				arr.OffsetSet(ctx, nil, phpVal)
			} else {
				arr.OffsetSet(ctx, phpv.ZString(arrayKey).ZVal(), phpVal)
			}
		} else {
			target.OffsetSet(ctx, phpv.ZString(key).ZVal(), phpVal)
		}
	}

	return result.ZVal(), nil
}

// iniProcessValue converts an INI value string to the appropriate PHP value.
func iniProcessValue(ctx phpv.Context, value string, mode phpv.ZInt) *phpv.ZVal {
	if mode == INI_SCANNER_RAW {
		return phpv.ZString(value).ZVal()
	}

	// Handle special values
	lower := strings.ToLower(value)
	switch lower {
	case "true", "on", "yes":
		if mode == INI_SCANNER_TYPED {
			return phpv.ZTrue.ZVal()
		}
		return phpv.ZString("1").ZVal()
	case "false", "off", "no", "none":
		if mode == INI_SCANNER_TYPED {
			return phpv.ZFalse.ZVal()
		}
		return phpv.ZString("").ZVal()
	case "null":
		if mode == INI_SCANNER_TYPED {
			return phpv.ZNULL.ZVal()
		}
		return phpv.ZString("").ZVal()
	case "":
		return phpv.ZString("").ZVal()
	}

	return phpv.ZString(value).ZVal()
}
