package xml

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

func init() {
	initSimpleXML()

	// Register the "xml" extension (event-based XML parser)
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "xml",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{},
		Functions: map[string]*phpctx.ExtFunction{
			"xml_parser_create":                       {Func: fncXMLParserCreate, Args: []*phpctx.ExtFunctionArg{}},
			"xml_parser_free":                         {Func: fncXMLParserFree, Args: []*phpctx.ExtFunctionArg{}},
			"xml_parse":                               {Func: fncXMLParse, Args: []*phpctx.ExtFunctionArg{}},
			"xml_set_element_handler":                 {Func: fncXMLSetElementHandler, Args: []*phpctx.ExtFunctionArg{}},
			"xml_set_character_data_handler":          {Func: fncXMLSetCharacterDataHandler, Args: []*phpctx.ExtFunctionArg{}},
			"xml_parser_set_option":                   {Func: fncXMLParserSetOption, Args: []*phpctx.ExtFunctionArg{}},
			"xml_parser_get_option":                   {Func: fncXMLParserGetOption, Args: []*phpctx.ExtFunctionArg{}},
			"xml_error_string":                        {Func: fncXMLErrorString, Args: []*phpctx.ExtFunctionArg{}},
			"xml_get_error_code":                      {Func: fncXMLGetErrorCode, Args: []*phpctx.ExtFunctionArg{}},
			"xml_get_current_line_number":             {Func: fncXMLGetCurrentLineNumber, Args: []*phpctx.ExtFunctionArg{}},
			"xml_set_object":                          {Func: fncXMLSetObject, Args: []*phpctx.ExtFunctionArg{}},
			"xml_set_default_handler":                 {Func: fncXMLSetDefaultHandler, Args: []*phpctx.ExtFunctionArg{}},
			"xml_set_processing_instruction_handler":  {Func: fncXMLSetProcessingInstructionHandler, Args: []*phpctx.ExtFunctionArg{}},
			"xml_set_notation_decl_handler":           {Func: fncXMLSetNotationDeclHandler, Args: []*phpctx.ExtFunctionArg{}},
			"xml_set_unparsed_entity_decl_handler":    {Func: fncXMLSetUnparsedEntityDeclHandler, Args: []*phpctx.ExtFunctionArg{}},
			"xml_parse_into_struct":                   {Func: fncXMLParseIntoStruct, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"XML_ERROR_NONE":          xmlErrorNone,
			"XML_ERROR_SYNTAX":        xmlErrorSyntax,
			"XML_ERROR_NO_ELEMENTS":   xmlErrorNoElements,
			"XML_ERROR_INVALID_TOKEN": xmlErrorInvalidToken,
			"XML_ERROR_TAG_MISMATCH":  xmlErrorTagMismatch,
			"XML_OPTION_CASE_FOLDING":    xmlOptionCaseFolding,
			"XML_OPTION_TARGET_ENCODING": xmlOptionTargetEncoding,
			"XML_OPTION_SKIP_TAGSTART":   xmlOptionSkipTagstart,
			"XML_OPTION_SKIP_WHITE":      xmlOptionSkipWhite,
		},
	})

	// Register the "SimpleXML" extension
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "SimpleXML",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			SimpleXMLElementClass,
		},
		Functions: map[string]*phpctx.ExtFunction{
			"simplexml_load_string": {Func: fncSimpleXMLLoadString, Args: []*phpctx.ExtFunctionArg{}},
			"simplexml_load_file":   {Func: fncSimpleXMLLoadFile, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
