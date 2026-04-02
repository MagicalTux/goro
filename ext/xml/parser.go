package xml

import (
	"encoding/xml"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// resolveCallable converts a ZVal to a phpv.Callable.
// Handles PHP closures, function name strings, and native Callables.
func resolveCallable(ctx phpv.Context, v *phpv.ZVal) (phpv.Callable, bool) {
	if v == nil || v.IsNull() {
		return nil, true // null = no handler (success)
	}
	c, err := core.SpawnCallable(ctx, v)
	if err != nil {
		return nil, false
	}
	return c, true
}

// xmlParserResource implements phpv.Resource for xml_parser_create
type xmlParserResource struct {
	id         int
	encoding   string
	caseFolding bool

	startHandler    phpv.Callable
	endHandler      phpv.Callable
	charDataHandler phpv.Callable

	// Error state
	errorCode int
	errorLine int

	// For incremental parsing: accumulate data
	buf strings.Builder
}

func (p *xmlParserResource) GetResourceType() phpv.ResourceType { return phpv.ResourceUnknown }
func (p *xmlParserResource) GetResourceID() int                 { return p.id }
func (p *xmlParserResource) ZVal() *phpv.ZVal                   { return phpv.NewZVal(p) }
func (p *xmlParserResource) Value() phpv.Val                    { return p }
func (p *xmlParserResource) GetType() phpv.ZType                { return phpv.ZtResource }
func (p *xmlParserResource) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	switch t {
	case phpv.ZtResource:
		return p, nil
	case phpv.ZtString:
		return phpv.ZStr("Resource id #" + phpv.ZInt(p.id).String()), nil
	case phpv.ZtInt:
		return phpv.ZInt(p.id), nil
	case phpv.ZtBool:
		return phpv.ZBool(true), nil
	}
	return phpv.ZBool(true), nil
}
func (p *xmlParserResource) String() string { return "Resource id #" + phpv.ZInt(p.id).String() }

// XML parser error constants
const (
	xmlErrorNone        phpv.ZInt = 0
	xmlErrorSyntax      phpv.ZInt = 76
	xmlErrorNoElements  phpv.ZInt = 3
	xmlErrorInvalidToken phpv.ZInt = 4
	xmlErrorTagMismatch phpv.ZInt = 7

	xmlOptionCaseFolding     phpv.ZInt = 1
	xmlOptionTargetEncoding  phpv.ZInt = 2
	xmlOptionSkipTagstart    phpv.ZInt = 3
	xmlOptionSkipWhite       phpv.ZInt = 4
)

// xml_parser_create - create an XML parser
func fncXMLParserCreate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var encoding core.Optional[phpv.ZString]
	_, err := core.Expand(ctx, args, &encoding)
	if err != nil {
		return nil, err
	}

	enc := "UTF-8"
	if encoding.HasArg() {
		enc = string(encoding.Get())
	}

	p := &xmlParserResource{
		id:          ctx.Global().NextResourceID(),
		encoding:    enc,
		caseFolding: true,
	}
	return p.ZVal(), nil
}

// xml_parser_free - free an XML parser
func fncXMLParserFree(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return phpv.ZFalse.ZVal(), nil
	}
	// Just return true - Go's GC handles cleanup
	return phpv.ZTrue.ZVal(), nil
}

// xml_parse - parse XML data
func fncXMLParse(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return phpv.ZFalse.ZVal(), nil
	}

	res, ok := args[0].Value().(*xmlParserResource)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}

	data := string(args[1].AsString(ctx))
	isFinal := false
	if len(args) > 2 && args[2] != nil {
		isFinal = bool(args[2].AsBool(ctx))
	}

	res.buf.WriteString(data)

	if !isFinal {
		return phpv.ZTrue.ZVal(), nil
	}

	// Parse the accumulated data
	return parseXMLWithCallbacks(ctx, res, res.buf.String())
}

func parseXMLWithCallbacks(ctx phpv.Context, res *xmlParserResource, data string) (*phpv.ZVal, error) {
	decoder := xml.NewDecoder(strings.NewReader(data))
	decoder.Strict = false

	lineNum := 1
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			res.errorCode = int(xmlErrorSyntax)
			res.errorLine = lineNum
			return phpv.ZInt(0).ZVal(), nil
		}

		switch t := tok.(type) {
		case xml.StartElement:
			lineNum++
			if res.startHandler != nil {
				elemName := t.Name.Local
				if res.caseFolding {
					elemName = strings.ToUpper(elemName)
				}
				// Build attrs array
				attrsArr := phpv.NewZArray()
				for _, attr := range t.Attr {
					attrName := attr.Name.Local
					if res.caseFolding {
						attrName = strings.ToUpper(attrName)
					}
					attrsArr.OffsetSet(ctx, phpv.ZString(attrName), phpv.ZStr(attr.Value))
				}
				_, err := ctx.CallZVal(ctx, res.startHandler, []*phpv.ZVal{
					phpv.NewZVal(res),
					phpv.ZStr(elemName),
					attrsArr.ZVal(),
				})
				if err != nil {
					return nil, err
				}
			}

		case xml.EndElement:
			lineNum++
			if res.endHandler != nil {
				elemName := t.Name.Local
				if res.caseFolding {
					elemName = strings.ToUpper(elemName)
				}
				_, err := ctx.CallZVal(ctx, res.endHandler, []*phpv.ZVal{
					phpv.NewZVal(res),
					phpv.ZStr(elemName),
				})
				if err != nil {
					return nil, err
				}
			}

		case xml.CharData:
			if res.charDataHandler != nil {
				_, err := ctx.CallZVal(ctx, res.charDataHandler, []*phpv.ZVal{
					phpv.NewZVal(res),
					phpv.ZStr(string(t)),
				})
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return phpv.ZTrue.ZVal(), nil
}

// xml_set_element_handler - set start/end element handlers
func fncXMLSetElementHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 {
		return phpv.ZFalse.ZVal(), nil
	}

	res, ok := args[0].Value().(*xmlParserResource)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}

	// Start handler
	if c, ok := resolveCallable(ctx, args[1]); ok {
		res.startHandler = c
	}

	// End handler
	if c, ok := resolveCallable(ctx, args[2]); ok {
		res.endHandler = c
	}

	return phpv.ZTrue.ZVal(), nil
}

// xml_set_character_data_handler - set character data handler
func fncXMLSetCharacterDataHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return phpv.ZFalse.ZVal(), nil
	}

	res, ok := args[0].Value().(*xmlParserResource)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}

	if c, ok := resolveCallable(ctx, args[1]); ok {
		res.charDataHandler = c
	}

	return phpv.ZTrue.ZVal(), nil
}

// xml_parser_set_option - set parser option
func fncXMLParserSetOption(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 {
		return phpv.ZFalse.ZVal(), nil
	}

	res, ok := args[0].Value().(*xmlParserResource)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}

	option := args[1].AsInt(ctx)
	switch phpv.ZInt(option) {
	case xmlOptionCaseFolding:
		res.caseFolding = bool(args[2].AsBool(ctx))
	case xmlOptionTargetEncoding:
		res.encoding = string(args[2].AsString(ctx))
	}

	return phpv.ZTrue.ZVal(), nil
}

// xml_parser_get_option - get parser option
func fncXMLParserGetOption(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return phpv.ZFalse.ZVal(), nil
	}

	res, ok := args[0].Value().(*xmlParserResource)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}

	option := phpv.ZInt(args[1].AsInt(ctx))
	switch option {
	case xmlOptionCaseFolding:
		return phpv.ZBool(res.caseFolding).ZVal(), nil
	case xmlOptionTargetEncoding:
		return phpv.ZStr(res.encoding), nil
	}

	return phpv.ZFalse.ZVal(), nil
}

// xml_error_string - get error string for error code
func fncXMLErrorString(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return phpv.ZStr(""), nil
	}
	code := phpv.ZInt(args[0].AsInt(ctx))
	switch code {
	case xmlErrorNone:
		return phpv.ZStr(""), nil
	case xmlErrorSyntax:
		return phpv.ZStr("syntax error"), nil
	case xmlErrorNoElements:
		return phpv.ZStr("no element found"), nil
	case xmlErrorInvalidToken:
		return phpv.ZStr("not well-formed (invalid token)"), nil
	case xmlErrorTagMismatch:
		return phpv.ZStr("mismatched tag"), nil
	}
	return phpv.ZStr("unknown error"), nil
}

// xml_get_error_code - get error code from parser
func fncXMLGetErrorCode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return phpv.ZFalse.ZVal(), nil
	}
	res, ok := args[0].Value().(*xmlParserResource)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZInt(res.errorCode).ZVal(), nil
}

// xml_get_current_line_number - get current line number
func fncXMLGetCurrentLineNumber(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return phpv.ZFalse.ZVal(), nil
	}
	res, ok := args[0].Value().(*xmlParserResource)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}
	return phpv.ZInt(res.errorLine).ZVal(), nil
}

// xml_set_object - set object to use for handlers (deprecated since PHP 8.4)
func fncXMLSetObject(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	ctx.Deprecated("Function xml_set_object() is deprecated since 8.4, provide a proper method callable to xml_set_*_handler() functions")
	return phpv.ZTrue.ZVal(), nil
}

// xml_set_default_handler - set default handler
func fncXMLSetDefaultHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZTrue.ZVal(), nil
}

// xml_set_processing_instruction_handler - set PI handler
func fncXMLSetProcessingInstructionHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZTrue.ZVal(), nil
}

// xml_set_notation_decl_handler - set notation declaration handler
func fncXMLSetNotationDeclHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZTrue.ZVal(), nil
}

// xml_set_unparsed_entity_decl_handler - set unparsed entity decl handler
func fncXMLSetUnparsedEntityDeclHandler(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZTrue.ZVal(), nil
}

// xml_parse_into_struct - parse XML into struct
func fncXMLParseIntoStruct(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 3 {
		return phpv.ZFalse.ZVal(), nil
	}

	res, ok := args[0].Value().(*xmlParserResource)
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}

	data := string(args[1].AsString(ctx))

	// Get the values array reference
	var valuesArr *phpv.ZArray
	if args[2] != nil && args[2].GetType() == phpv.ZtArray {
		valuesArr = args[2].Value().(*phpv.ZArray)
	} else {
		valuesArr = phpv.NewZArray()
	}

	// Optional index array
	var indexArr *phpv.ZArray
	if len(args) > 3 && args[3] != nil && args[3].GetType() == phpv.ZtArray {
		indexArr = args[3].Value().(*phpv.ZArray)
	}

	decoder := xml.NewDecoder(strings.NewReader(data))
	decoder.Strict = false

	var depth int
	var idx int

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			res.errorCode = int(xmlErrorSyntax)
			return phpv.ZInt(0).ZVal(), nil
		}

		switch t := tok.(type) {
		case xml.StartElement:
			elemName := t.Name.Local
			if res.caseFolding {
				elemName = strings.ToUpper(elemName)
			}

			entry := phpv.NewZArray()
			entry.OffsetSet(ctx, phpv.ZString("tag"), phpv.ZStr(elemName))
			entry.OffsetSet(ctx, phpv.ZString("type"), phpv.ZStr("open"))
			entry.OffsetSet(ctx, phpv.ZString("level"), phpv.ZInt(depth+1).ZVal())

			if len(t.Attr) > 0 {
				attrsArr := phpv.NewZArray()
				for _, attr := range t.Attr {
					attrName := attr.Name.Local
					if res.caseFolding {
						attrName = strings.ToUpper(attrName)
					}
					attrsArr.OffsetSet(ctx, phpv.ZString(attrName), phpv.ZStr(attr.Value))
				}
				entry.OffsetSet(ctx, phpv.ZString("attributes"), attrsArr.ZVal())
			}

			valuesArr.OffsetSet(ctx, phpv.ZInt(idx), entry.ZVal())
			if indexArr != nil {
				indexArr.OffsetSet(ctx, phpv.ZString(elemName), phpv.ZInt(idx).ZVal())
			}
			idx++
			depth++

		case xml.EndElement:
			depth--
			elemName := t.Name.Local
			if res.caseFolding {
				elemName = strings.ToUpper(elemName)
			}

			entry := phpv.NewZArray()
			entry.OffsetSet(ctx, phpv.ZString("tag"), phpv.ZStr(elemName))
			entry.OffsetSet(ctx, phpv.ZString("type"), phpv.ZStr("close"))
			entry.OffsetSet(ctx, phpv.ZString("level"), phpv.ZInt(depth+1).ZVal())

			valuesArr.OffsetSet(ctx, phpv.ZInt(idx), entry.ZVal())
			idx++

		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" {
				// Add content to previous open element
				if valuesArr.Count(ctx) > 0 {
					// Just append character data as a separate entry
					entry := phpv.NewZArray()
					entry.OffsetSet(ctx, phpv.ZString("tag"), phpv.ZStr(""))
					entry.OffsetSet(ctx, phpv.ZString("value"), phpv.ZStr(string(t)))
					entry.OffsetSet(ctx, phpv.ZString("type"), phpv.ZStr("cdata"))
					entry.OffsetSet(ctx, phpv.ZString("level"), phpv.ZInt(depth).ZVal())
					valuesArr.OffsetSet(ctx, phpv.ZInt(idx), entry.ZVal())
					idx++
				}
			}
		}
	}

	return phpv.ZTrue.ZVal(), nil
}
