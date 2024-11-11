package main_test

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"log"
	"math"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/MagicalTux/goro/core/phpv"
	"golang.org/x/tools/go/ast/astutil"
)

type FuncDecl struct {
	GoIdent  string
	PhpIdent string
	Package  string
}

type ExtData struct {
	Imports   map[string]string
	Functions map[string]FuncDecl
	Constants map[string]struct{}
	Classes   []string
}

const M_PI = math.Pi
const M_PI2 = phpv.ZFloat(math.Pi)

func TestParse(t *testing.T) {

	fset := token.NewFileSet() // positions are relative to fset

	src := `package foo

	import "math"

	const M_PI = math.Pi
	const M_PI2 = phpv.ZFloat(math.Pi)
	const FOO = 1

	// bar comment
	// another one
	func bar() {
		// bar comment
		fmt.Println(time.Now())
	}`

	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("imports: %+v\n", astutil.Imports(fset, f))

	for _, commentGroup := range f.Comments {
		for _, comment := range commentGroup.List {
			println(">>", comment.Text)
		}
	}

	if f.Doc != nil {
		for _, comment := range f.Doc.List {
			println(">>", comment.Text)
		}
	}

	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl:
			fmt.Println("Func", decl.Name)
			for _, comment := range decl.Doc.List {
				println("comment:", comment.Text)
			}

		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.ImportSpec:
					// fmt.Println("Import", spec.Path.Value)
				case *ast.TypeSpec:
					fmt.Println("Type", spec.Name.String())
				case *ast.ValueSpec:
					// for _, id := range spec.Names {
					// 	if id.Obj.Kind == ast.Con {
					// 		fmt.Printf("Const %s: %+v\n", id.Name, id.Obj)
					// 	}
					// }
					println("-----")
					printValueSpec(spec)
				default:
					fmt.Printf("Unknown token type: %s\n", decl.Tok)
				}
			}
		default:
			fmt.Printf("Unknown declaration: %v @\n", decl.Pos())
		}
	}
}

func printValueSpec(v *ast.ValueSpec) {
	for i := range v.Values {
		fmt.Printf("VARIABLE %s\n", v.Names[i])
		printExpr(0, v.Values[i])
		fmt.Println()
	}
}

func printExpr(indent int, e ast.Expr) {
	switch val := e.(type) {
	case *ast.BasicLit:
		fmt.Printf("%v[Basic] %v\n", strings.Repeat("\t", indent), val.Value)
	case *ast.CompositeLit:
		fmt.Printf("%v[Composit]\n", strings.Repeat("\t", indent))
		for _, e := range val.Elts {
			printExpr(indent+1, e)
		}
	case *ast.KeyValueExpr: // structure fields or basic map
		fmt.Printf("%v[KeyValue]\n", strings.Repeat("\t", indent))
		printExpr(indent+1, val.Key)
		printExpr(indent+1, val.Value)
	case *ast.SelectorExpr: // structure fields or basic map
		fmt.Printf("%v[SelectorExpr]\n", strings.Repeat("\t", indent))
		printExpr(indent+1, val.X)
		printExpr(indent+1, val.Sel)

	case *ast.Ident: // structure data fields name (need something before to connect field name with value)
		fmt.Printf("%v[Ident] %v\n", strings.Repeat("\t", indent), val.Name)
	case *ast.CallExpr:
		fmt.Printf("%v[CallExpt]\n", strings.Repeat("\t", indent))
		printExpr(indent+1, val.Fun)
		for _, arg := range val.Args {
			printExpr(indent+1, arg)
		}
	default:
		fmt.Printf("%v[?] ???\n", strings.Repeat("\t", indent))
	}
}

func TestLookup(t *testing.T) {
	src := `
	package foo

	import "math"

	const M_PI = math.Pi
	// const M_PI2 = phpv.ZFloat(math.Pi)
	const FOO = 1

	func foo() {
		var y int = 2
		math := 1
		_ = math
		_ = y
	}
	`
	fset := token.NewFileSet() // positions are relative to fset
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}

	conf := types.Config{Importer: importer.Default()}
	info := &types.Info{
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
		Types: make(map[ast.Expr]types.TypeAndValue),
	}

	pkg, err := conf.Check("foo", fset, []*ast.File{f}, info)
	if err != nil {
		log.Fatal(err) // type error
	}
	obj := pkg.Scope().Lookup("M_PI")
	fmt.Printf("%v\n", obj)

	ast.Inspect(f, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok {
			inner := pkg.Scope().Innermost(id.Pos())
			_, obj := inner.LookupParent(id.Name, id.Pos())
			print(">>", id.Name, " ")
			if obj != nil {
				print(" type:", obj.Type().String())
			}
			if id.Obj != nil {
				print(" kind:", id.Obj.Kind.String())
			}
			println()
		}
		return true
	})
	// println("--------")
	// for id, obj := range info.Defs {
	// 	fmt.Printf("%s: %q defines %v\n",
	// 		fset.Position(id.Pos()), id.Name, obj)
	// }
	// println("--------")
	// for id, obj := range info.Types {
	// 	fmt.Printf("%s: %q defines %+v %v\n",
	// 		fset.Position(id.Pos()), id, obj.Value.ExactString(), obj.Type)
	// }

}

func TestProcessExt(t *testing.T) {
	processExt("ext/standard", "ext.go")
}

func processExt(dirname, destFilename string) error {
	ext := &ExtData{
		Imports:   make(map[string]string),
		Functions: make(map[string]FuncDecl),
		Constants: make(map[string]struct{}),
	}
	err := filepath.WalkDir(dirname, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		println(">", path, d.Name())
		processExtFile(path, ext)
		return nil
	})

	if err != nil {
		return err
	}

	println("------------------------------\nconstants:")
	for k := range ext.Constants {
		println("\t", k)
	}
	println("------------------------------\nfunctions:")
	var funcNames []string
	for k := range ext.Functions {
		funcNames = append(funcNames, k)
	}
	sort.Strings(funcNames)
	for _, k := range funcNames {
		decl := ext.Functions[k]
		fmt.Printf("\t%s  %s  %s\n", k, decl.GoIdent, decl.Package)
	}

	output := `
package standard

import (%s)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "standard",
		Version: core.VERSION,
		Classes: []phpv.ZClass{},
		Functions: map[string]*phpctx.ExtFunction{%s},
		Constants: map[phpv.ZString]phpv.Val{%s},
	})
}
	`

	imports := []string{
		"github.com/MagicalTux/goro/core",
		"github.com/MagicalTux/goro/core/phpctx",
		"github.com/MagicalTux/goro/core/phpv",
	}

	var buf bytes.Buffer
	var importStr string
	var functionStr string
	var constantStr string

	for _, decl := range ext.Functions {
		_ = decl
		// TODO: add subpackage imports here
	}

	if len(imports) > 0 {
		buf.WriteRune('\n')
	}
	for _, imp := range imports {
		buf.WriteString("\t\"")
		buf.WriteString(imp)
		buf.WriteString("\"\n")
	}
	if len(imports) > 0 {
		buf.WriteRune('\n')
	}
	importStr = buf.String()
	buf.Reset()

	fmt.Printf(output, importStr, functionStr, constantStr)

	return nil
}

func processExtFile(filename string, ext *ExtData) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}

	// importSet := map[string]struct{}{}
	// constants := map[string]struct{}{}
	// importNames := getImports(f)
	ids := getPackageConstants(f)
	funcs := getPhpFunctions(f)
	for _, id := range ids {
		for _, n := range id.Names {
			ext.Constants[n.Name] = struct{}{}
		}
		// var imports []string
		// ast.Inspect(id, func(node ast.Node) bool {
		// 	if id, ok := node.(*ast.Ident); ok {
		// 		if p, ok := importNames[id.Name]; ok {
		// 			imports = append(imports, p)
		// 			importSet[p] = struct{}{}
		// 		}
		// 	}
		// 	return true
		// })
	}
	for _, decl := range funcs {
		decl.Package = path.Dir(filename)
		ext.Functions[decl.PhpIdent] = decl
	}

	println(filename, "funcs:")
	for k, v := range ext.Functions {
		println("\t", k, v.GoIdent, v.Package)
	}

	println(filename, "constants:")
	for k := range ext.Constants {
		println("\t", k)
	}

	// println("imports:")
	// for k := range importSet {
	// 	println("\t", k)
	// }
}

// func TestGetImports(t *testing.T) {
// 	src := `
// 	package bar
//
// 	import math "math"
// 	import "github.com/MagicalTux/goro/core/tokenizer"
//
// 	const M_PI = math.Pi
// 	// const M_PI2 = phpv.ZFloat(math.Pi)
// 	const (
// 		FOO = 1
// 		X = tokenizer.T_EOF
// 	)
//
// 	func foo() {
// 		var y int = 2
// 		math := 1
// 		_ = math
// 		_ = y
// 		_ = importer.Default()
// 	}
// 	`
// 	fset := token.NewFileSet() // positions are relative to fset
// 	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
//
// 	importSet := map[string]struct{}{}
// 	constants := map[string]struct{}{}
// 	importNames := getImports(f)
// 	ids := getPackageConstants(f)
// 	for _, id := range ids {
// 		for _, n := range id.Names {
// 			constants[n.Name] = struct{}{}
// 		}
// 		var imports []string
// 		ast.Inspect(id, func(node ast.Node) bool {
// 			if id, ok := node.(*ast.Ident); ok {
// 				if p, ok := importNames[id.Name]; ok {
// 					imports = append(imports, p)
// 					importSet[p] = struct{}{}
// 				}
// 			}
// 			return true
// 		})
// 	}
// 	println("constants:")
// 	for k := range constants {
// 		println("\t", k)
// 	}
//
// 	println("imports:")
// 	for k := range importSet {
// 		println("\t", k)
// 	}
//
// 	println("classes:")
// 	// TODO
// 	println("functions:")
// 	// TODO
// }

func TestFunctionSignatureParse(t *testing.T) {
	parseFunctionSignature("// > func bool|x foo (x,y,z)")
	parseFunctionSignature("// > func bool bar()")
	parseFunctionSignature("// > func bool    baz(  )  ")
}

func parseFunctionSignature(s string) string {
	re := regexp.MustCompile(`\/\/\s*>\s*func\s[\w|]+\s*(\w*)\s*\(.*\)`)
	matches := re.FindStringSubmatch(s)
	if len(matches) >= 1 {
		return matches[1]
	}
	return ""
}

func getPhpFunctions(f *ast.File) []FuncDecl {
	var result []FuncDecl
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl:
			if decl.Doc == nil {
				continue
			}

			phpIdent := ""

			for _, comment := range decl.Doc.List {
				phpIdent = parseFunctionSignature(comment.Text)
				if phpIdent != "" {
					break
				}
			}
			if phpIdent != "" {
				result = append(result, FuncDecl{
					GoIdent:  decl.Name.Name,
					PhpIdent: phpIdent,
				})
			}

		}
	}

	return result
}

func getPackageConstants(f *ast.File) []*ast.ValueSpec {
	var values []*ast.ValueSpec
	for _, d := range f.Decls {
		switch decl := d.(type) {

		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec:
					for _, id := range spec.Names {
						if id.Obj.Kind == ast.Con {
							values = append(values, spec)
							break
						}
						// fmt.Printf("Const %s: %+v\n", id.Name, id.Obj)
					}
				}
			}
		}
	}
	return values
}

func getImports(f *ast.File) map[string]string {
	importNames := make(map[string]string)
	for _, im := range f.Imports {
		var name string
		if im.Name != nil {
			name = im.Name.Name
		} else {
			name = path.Base(im.Path.Value[1 : len(im.Path.Value)-1])
		}
		importNames[name] = im.Path.Value
	}
	return importNames

}
