package xml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// xmlNode is the internal Go struct for the XML tree
type xmlNode struct {
	Name     xml.Name
	Attrs    []xml.Attr
	Children []*xmlNode
	Content  string
	Parent   *xmlNode
}

// SimpleXMLElementClass is the PHP class object
var SimpleXMLElementClass *phpobj.ZClass

// countableInterface is the PHP Countable interface (name-based so count() recognizes it)
var countableInterface = &phpobj.ZClass{
	Type: phpv.ZClassTypeInterface,
	Name: "Countable",
	Methods: map[phpv.ZString]*phpv.ZClassMethod{
		"count": {Name: "count", Modifiers: phpv.ZAttrPublic, Empty: true},
	},
}

// simpleXMLData holds the internal state of a SimpleXMLElement instance.
// The iterator behaviour depends on the mode:
//   - Normal mode (siblingFilter == ""): iterates over all children of node
//   - Filtered mode (siblingFilter != ""): iterates over siblings of node
//     that share the same name as siblingFilter, starting from node itself.
//     In this case, node is the FIRST matching sibling, and node.Parent is
//     used to find the full sibling list.
type simpleXMLData struct {
	node          *xmlNode
	siblingFilter string // when non-empty, iterate siblings with this name from parent
	iterPos       int    // current position in the sibling/child list during iteration
	// cached filtered list (computed lazily during iteration)
	filteredList []*xmlNode
}

func getSimpleXMLData(o *phpobj.ZObject) *simpleXMLData {
	if d, ok := o.GetOpaque(SimpleXMLElementClass).(*simpleXMLData); ok {
		return d
	}
	return nil
}

// getIterList returns the list of nodes to iterate over.
// For normal mode: children of node.
// For filtered mode: siblings of node with matching name.
func (d *simpleXMLData) getIterList() []*xmlNode {
	if d.siblingFilter == "" {
		return d.node.Children
	}
	if d.filteredList != nil {
		return d.filteredList
	}
	// Build filtered list from parent's children
	if d.node.Parent == nil {
		d.filteredList = []*xmlNode{d.node}
		return d.filteredList
	}
	for _, child := range d.node.Parent.Children {
		if child.Name.Local == d.siblingFilter {
			d.filteredList = append(d.filteredList, child)
		}
	}
	return d.filteredList
}

// parseXMLString parses XML string and returns the root node
func parseXMLString(data string) (*xmlNode, error) {
	decoder := xml.NewDecoder(strings.NewReader(data))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity

	var stack []*xmlNode
	var root *xmlNode

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("XML parse error: %s", err.Error())
		}

		switch t := tok.(type) {
		case xml.StartElement:
			node := &xmlNode{
				Name:  t.Name,
				Attrs: t.Attr,
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				node.Parent = parent
				parent.Children = append(parent.Children, node)
			} else {
				root = node
			}
			stack = append(stack, node)

		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}

		case xml.CharData:
			if len(stack) > 0 {
				current := stack[len(stack)-1]
				current.Content += string(t)
			}
		}
	}

	return root, nil
}

// nodeToXMLString serializes an xmlNode back to XML
func nodeToXMLString(node *xmlNode) string {
	var buf bytes.Buffer
	buf.WriteString("<?xml version=\"1.0\"?>\n")
	writeNodeXML(&buf, node)
	buf.WriteString("\n")
	return buf.String()
}

func writeNodeXML(buf *bytes.Buffer, node *xmlNode) {
	// Build element name
	elemName := node.Name.Local
	if node.Name.Space != "" {
		elemName = node.Name.Space + ":" + node.Name.Local
	}

	buf.WriteString("<" + elemName)

	// Write attributes
	for _, attr := range node.Attrs {
		attrName := attr.Name.Local
		if attr.Name.Space != "" {
			attrName = attr.Name.Space + ":" + attr.Name.Local
		}
		buf.WriteString(" " + attrName + "=\"" + xmlEscape(attr.Value) + "\"")
	}

	if len(node.Children) == 0 {
		if node.Content == "" {
			buf.WriteString("/>")
		} else {
			buf.WriteString(">" + xmlEscape(node.Content) + "</" + elemName + ">")
		}
	} else {
		buf.WriteString(">")
		if node.Content != "" && strings.TrimSpace(node.Content) != "" {
			buf.WriteString(xmlEscape(node.Content))
		}
		for _, child := range node.Children {
			writeNodeXML(buf, child)
		}
		buf.WriteString("</" + elemName + ">")
	}
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// makeSimpleXMLObject creates a SimpleXMLElement PHP object from an xmlNode
func makeSimpleXMLObject(ctx phpv.Context, node *xmlNode) (*phpobj.ZObject, error) {
	return makeSimpleXMLObjectFiltered(ctx, node, "")
}

// makeSimpleXMLObjectFiltered creates a filtered SimpleXMLElement (for sibling iteration)
func makeSimpleXMLObjectFiltered(ctx phpv.Context, node *xmlNode, filter string) (*phpobj.ZObject, error) {
	obj, err := phpobj.CreateZObject(ctx, SimpleXMLElementClass)
	if err != nil {
		return nil, err
	}
	obj.SetOpaque(SimpleXMLElementClass, &simpleXMLData{
		node:          node,
		siblingFilter: filter,
	})
	return obj, nil
}

func initSimpleXML() {
	SimpleXMLElementClass = &phpobj.ZClass{
		Name: "SimpleXMLElement",
		Implementations: []*phpobj.ZClass{
			phpobj.Iterator,
			phpobj.ArrayAccess,
			phpobj.Stringable,
			countableInterface,
		},
		Methods: map[phpv.ZString]*phpv.ZClassMethod{
			"__construct": {
				Name:   "__construct",
				Method: phpobj.NativeMethod(sxeConstruct),
			},
			"__tostring": {
				Name:   "__toString",
				Method: phpobj.NativeMethod(sxeToString),
			},
			"getname": {
				Name:   "getName",
				Method: phpobj.NativeMethod(sxeGetName),
			},
			"asxml": {
				Name:   "asXML",
				Method: phpobj.NativeMethod(sxeAsXML),
			},
			"children": {
				Name:   "children",
				Method: phpobj.NativeMethod(sxeChildren),
			},
			"attributes": {
				Name:   "attributes",
				Method: phpobj.NativeMethod(sxeAttributes),
			},
			"count": {
				Name:   "count",
				Method: phpobj.NativeMethod(sxeCount),
			},
			"addchild": {
				Name:   "addChild",
				Method: phpobj.NativeMethod(sxeAddChild),
			},
			"addattribute": {
				Name:   "addAttribute",
				Method: phpobj.NativeMethod(sxeAddAttribute),
			},
			// Iterator methods
			"rewind": {
				Name:   "rewind",
				Method: phpobj.NativeMethod(sxeRewind),
			},
			"current": {
				Name:   "current",
				Method: phpobj.NativeMethod(sxeCurrent),
			},
			"key": {
				Name:   "key",
				Method: phpobj.NativeMethod(sxeKey),
			},
			"next": {
				Name:   "next",
				Method: phpobj.NativeMethod(sxeNext),
			},
			"valid": {
				Name:   "valid",
				Method: phpobj.NativeMethod(sxeValid),
			},
			// ArrayAccess methods for $xml['attr'] attribute access
			"offsetexists": {
				Name:   "offsetExists",
				Method: phpobj.NativeMethod(sxeOffsetExists),
			},
			"offsetget": {
				Name:   "offsetGet",
				Method: phpobj.NativeMethod(sxeOffsetGet),
			},
			"offsetset": {
				Name:   "offsetSet",
				Method: phpobj.NativeMethod(sxeOffsetSet),
			},
			"offsetunset": {
				Name:   "offsetUnset",
				Method: phpobj.NativeMethod(sxeOffsetUnset),
			},
		},
		H: &phpv.ZClassHandlers{
			// Intercept $xml->childName property access to return child nodes.
			// When there are multiple children with the same name, we return an
			// object that will iterate over all of them.
			HandlePropGet: func(ctx phpv.Context, o phpv.ZObject, key phpv.ZString) (*phpv.ZVal, error) {
				zo, ok := o.(*phpobj.ZObject)
				if !ok {
					return nil, nil
				}
				d := getSimpleXMLData(zo)
				if d == nil {
					return nil, nil
				}
				// Find first matching child
				var firstMatch *xmlNode
				var count int
				for _, child := range d.node.Children {
					if phpv.ZString(child.Name.Local) == key {
						if firstMatch == nil {
							firstMatch = child
						}
						count++
					}
				}
				if firstMatch == nil {
					// Return empty SimpleXMLElement with the requested name
					emptyNode := &xmlNode{Name: xml.Name{Local: string(key)}}
					obj, err := makeSimpleXMLObject(ctx, emptyNode)
					if err != nil {
						return nil, err
					}
					return obj.ZVal(), nil
				}
				if count == 1 {
					// Single child: return it directly (iteration will just return itself)
					obj, err := makeSimpleXMLObjectFiltered(ctx, firstMatch, string(key))
					if err != nil {
						return nil, err
					}
					return obj.ZVal(), nil
				}
				// Multiple children: return filtered object for sibling iteration
				obj, err := makeSimpleXMLObjectFiltered(ctx, firstMatch, string(key))
				if err != nil {
					return nil, err
				}
				return obj.ZVal(), nil
			},
			HandlePropSet: func(ctx phpv.Context, o phpv.ZObject, key phpv.ZString, value *phpv.ZVal) (bool, error) {
				zo, ok := o.(*phpobj.ZObject)
				if !ok {
					return false, nil
				}
				d := getSimpleXMLData(zo)
				if d == nil {
					return false, nil
				}
				// Find or create child with that name
				for _, child := range d.node.Children {
					if phpv.ZString(child.Name.Local) == key {
						child.Content = string(value.AsString(ctx))
						return true, nil
					}
				}
				// Create new child
				child := &xmlNode{
					Name:    xml.Name{Local: string(key)},
					Content: string(value.AsString(ctx)),
					Parent:  d.node,
				}
				d.node.Children = append(d.node.Children, child)
				return true, nil
			},
		},
	}
}

func sxeConstruct(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var options core.Optional[phpv.ZInt]
	var dataIsURL core.Optional[phpv.ZBool]
	_, err := core.Expand(ctx, args, &data, &options, &dataIsURL)
	if err != nil {
		return nil, err
	}

	var xmlData string
	if dataIsURL.GetOrDefault(false) {
		// Load from URL/file
		content, err := os.ReadFile(string(data))
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
				fmt.Sprintf("SimpleXMLElement::__construct(): %s", err.Error()))
		}
		xmlData = string(content)
	} else {
		xmlData = string(data)
	}

	root, err := parseXMLString(xmlData)
	if err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			fmt.Sprintf("SimpleXMLElement::__construct(): %s", err.Error()))
	}
	if root == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException,
			"SimpleXMLElement::__construct(): Failed to parse XML")
	}

	o.SetOpaque(SimpleXMLElementClass, &simpleXMLData{node: root})
	return nil, nil
}

func sxeToString(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(strings.TrimSpace(d.node.Content)), nil
}

func sxeGetName(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil {
		return phpv.ZStr(""), nil
	}
	return phpv.ZStr(d.node.Name.Local), nil
}

func sxeAsXML(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	xmlStr := nodeToXMLString(d.node)

	if len(args) > 0 && args[0] != nil && !args[0].IsNull() {
		filename := string(args[0].AsString(ctx))
		if err := os.WriteFile(filename, []byte(xmlStr), 0644); err != nil {
			return phpv.ZFalse.ZVal(), nil
		}
		return phpv.ZTrue.ZVal(), nil
	}

	return phpv.ZStr(xmlStr), nil
}

func sxeChildren(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	// Return a normal-mode SimpleXMLElement wrapping the same node.
	// Iterating over it yields all children.
	obj, err := makeSimpleXMLObject(ctx, d.node)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func sxeAttributes(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	// Create a virtual node whose "children" are the attributes
	// represented as text nodes. In PHP, attributes() returns an
	// object where foreach gives attribute name => value pairs.
	// We implement this by creating an @attributes virtual node.
	attrNode := &xmlNode{
		Name:   xml.Name{Local: "@attributes"},
		Attrs:  d.node.Attrs,
		Parent: d.node,
	}
	// Add virtual "children" for each attribute (for iteration)
	for _, attr := range d.node.Attrs {
		child := &xmlNode{
			Name:    xml.Name{Local: attr.Name.Local},
			Content: attr.Value,
			Parent:  attrNode,
		}
		attrNode.Children = append(attrNode.Children, child)
	}
	obj, err := makeSimpleXMLObject(ctx, attrNode)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func sxeCount(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	list := d.getIterList()
	return phpv.ZInt(len(list)).ZVal(), nil
}

func sxeAddChild(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if len(args) < 1 {
		return phpv.ZNULL.ZVal(), nil
	}
	name := string(args[0].AsString(ctx))
	content := ""
	if len(args) > 1 && args[1] != nil && !args[1].IsNull() {
		content = string(args[1].AsString(ctx))
	}

	child := &xmlNode{
		Name:    xml.Name{Local: name},
		Content: content,
		Parent:  d.node,
	}
	d.node.Children = append(d.node.Children, child)

	obj, err := makeSimpleXMLObject(ctx, child)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func sxeAddAttribute(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil {
		return phpv.ZNULL.ZVal(), nil
	}
	if len(args) < 2 {
		return phpv.ZNULL.ZVal(), nil
	}
	name := string(args[0].AsString(ctx))
	value := string(args[1].AsString(ctx))

	// Check if attribute already exists
	for i, attr := range d.node.Attrs {
		if attr.Name.Local == name {
			d.node.Attrs[i].Value = value
			return phpv.ZNULL.ZVal(), nil
		}
	}

	d.node.Attrs = append(d.node.Attrs, xml.Attr{
		Name:  xml.Name{Local: name},
		Value: value,
	})
	return phpv.ZNULL.ZVal(), nil
}

// Iterator methods
// When siblingFilter is set, we iterate over siblings with that name.
// When not set, we iterate over direct children.

func sxeRewind(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil {
		return nil, nil
	}
	d.iterPos = 0
	return nil, nil
}

func sxeCurrent(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	list := d.getIterList()
	if d.iterPos < 0 || d.iterPos >= len(list) {
		return phpv.ZFalse.ZVal(), nil
	}
	node := list[d.iterPos]
	obj, err := makeSimpleXMLObject(ctx, node)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func sxeKey(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil {
		return phpv.ZInt(0).ZVal(), nil
	}
	// For child iteration, return element name as key (PHP SimpleXML behaviour)
	list := d.getIterList()
	if d.iterPos >= 0 && d.iterPos < len(list) {
		return phpv.ZStr(list[d.iterPos].Name.Local), nil
	}
	return phpv.ZInt(d.iterPos).ZVal(), nil
}

func sxeNext(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil {
		return nil, nil
	}
	d.iterPos++
	return nil, nil
}

func sxeValid(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	list := d.getIterList()
	return phpv.ZBool(d.iterPos >= 0 && d.iterPos < len(list)).ZVal(), nil
}

// ArrayAccess methods - for $xml['attr'] attribute access
func sxeOffsetExists(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil || len(args) < 1 {
		return phpv.ZFalse.ZVal(), nil
	}
	key := string(args[0].AsString(ctx))
	for _, attr := range d.node.Attrs {
		if attr.Name.Local == key {
			return phpv.ZTrue.ZVal(), nil
		}
	}
	return phpv.ZFalse.ZVal(), nil
}

func sxeOffsetGet(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil || len(args) < 1 {
		return phpv.ZNULL.ZVal(), nil
	}
	key := string(args[0].AsString(ctx))
	for _, attr := range d.node.Attrs {
		if attr.Name.Local == key {
			// Return a SimpleXMLElement wrapping the attribute value
			attrNode := &xmlNode{
				Name:    xml.Name{Local: attr.Name.Local},
				Content: attr.Value,
				Parent:  d.node,
			}
			obj, err := makeSimpleXMLObject(ctx, attrNode)
			if err != nil {
				return phpv.ZNULL.ZVal(), nil
			}
			return obj.ZVal(), nil
		}
	}
	return phpv.ZNULL.ZVal(), nil
}

func sxeOffsetSet(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil || len(args) < 2 {
		return nil, nil
	}
	key := string(args[0].AsString(ctx))
	value := string(args[1].AsString(ctx))
	for i, attr := range d.node.Attrs {
		if attr.Name.Local == key {
			d.node.Attrs[i].Value = value
			return nil, nil
		}
	}
	d.node.Attrs = append(d.node.Attrs, xml.Attr{
		Name:  xml.Name{Local: key},
		Value: value,
	})
	return nil, nil
}

func sxeOffsetUnset(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
	d := getSimpleXMLData(o)
	if d == nil || len(args) < 1 {
		return nil, nil
	}
	key := string(args[0].AsString(ctx))
	for i, attr := range d.node.Attrs {
		if attr.Name.Local == key {
			d.node.Attrs = append(d.node.Attrs[:i], d.node.Attrs[i+1:]...)
			return nil, nil
		}
	}
	return nil, nil
}

// simplexml_load_string - parse XML from string
func fncSimpleXMLLoadString(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"simplexml_load_string() expects at least 1 argument, 0 given")
	}

	data := string(args[0].AsString(ctx))

	root, err := parseXMLString(data)
	if err != nil || root == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	obj, err := makeSimpleXMLObject(ctx, root)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return obj.ZVal(), nil
}

// simplexml_load_file - parse XML from file
func fncSimpleXMLLoadFile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			"simplexml_load_file() expects at least 1 argument, 0 given")
	}

	filename := string(args[0].AsString(ctx))
	content, err := os.ReadFile(filename)
	if err != nil {
		ctx.Warn("simplexml_load_file(%s): failed to open stream: %s", filename, err.Error())
		return phpv.ZFalse.ZVal(), nil
	}

	root, parseErr := parseXMLString(string(content))
	if parseErr != nil || root == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	obj, err := makeSimpleXMLObject(ctx, root)
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return obj.ZVal(), nil
}
