package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type IdentiferMap struct {
	GoIdent  string
	PhpIdent string
	Package  string
}

type ExtData struct {
	Dirname   string
	Imports   map[string]string
	Functions map[string]IdentiferMap
	Constants map[string]IdentiferMap
	Classes   map[string]IdentiferMap
}

func main() {
	type ExtEntry struct {
		dirname  string
		destfile string
	}

	entries := []ExtEntry{
		{dirname: "core", destfile: "core-ext.go"},
	}
	files, err := os.ReadDir("ext")
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		entries = append(entries, ExtEntry{
			dirname:  path.Join("ext", f.Name()),
			destfile: "ext.go",
		})
	}

	for _, entry := range entries {
		ext, err := processExt(entry.dirname, entry.destfile)
		if err != nil {
			panic(err)
		}
		destfile := filepath.Join(entry.dirname, entry.destfile)
		file, err := os.OpenFile(destfile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		println("generated", destfile)
		writeExt(ext, file)
		file.Close()
	}

}

func writeExt(ext *ExtData, w io.Writer) {
	var constants []string
	var funcNames []string
	var classes []string
	for k := range ext.Constants {
		constants = append(constants, k)
	}
	for k := range ext.Functions {
		funcNames = append(funcNames, k)
	}
	for k := range ext.Classes {
		classes = append(classes, k)
	}

	sort.Strings(funcNames)
	sort.Strings(constants)
	sort.Strings(classes)

	template := `
package %s

import (%s)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:     "standard",
		Version:   %s,
		Classes:   []phpv.ZClass{%s},
		Functions: map[string]*phpctx.ExtFunction{%s},
		Constants: map[phpv.ZString]phpv.Val{%s},
	})
}
	`

	version := "VERSION"
	importSet := map[string]struct{}{
		// "github.com/MagicalTux/goro/core":        {},
		"github.com/MagicalTux/goro/core/phpctx": {},
		"github.com/MagicalTux/goro/core/phpv":   {},
	}

	if ext.Dirname != "core" {
		importSet["github.com/MagicalTux/goro/core"] = struct{}{}
		version = "core.VERSION"
	}

	var buf bytes.Buffer
	var importStr string
	var functionStr string
	var constantStr string
	var classStr string

	if len(ext.Functions) > 0 {
		buf.WriteRune('\n')
	}
	for _, phpIdent := range funcNames {
		decl := ext.Functions[phpIdent]
		if decl.Package != ext.Dirname {
			pkg := "github.com/MagicalTux/goro/" + decl.Package
			importSet[pkg] = struct{}{}
		}

		goIdent := decl.GoIdent
		if decl.Package != ext.Dirname {
			goIdent = path.Base(decl.Package) + "." + goIdent
		}

		format := "\t\t\t" + `"%s": {Func: %s, Args: []*phpctx.ExtFunctionArg{}},` + "\n"
		buf.WriteString(fmt.Sprintf(format, phpIdent, goIdent))
	}
	if len(ext.Functions) > 0 {
		buf.WriteString("\t\t")
	}
	functionStr = buf.String()
	buf.Reset()

	if len(ext.Classes) > 0 {
		buf.WriteRune('\n')
	}
	for _, phpIdent := range classes {
		decl := ext.Classes[phpIdent]
		if decl.Package != ext.Dirname {
			pkg := "github.com/MagicalTux/goro/" + decl.Package
			importSet[pkg] = struct{}{}
		}

		goIdent := decl.GoIdent
		if decl.Package != ext.Dirname {
			goIdent = path.Base(decl.Package) + "." + goIdent
		}

		format := "\t\t\t" + `%s,` + "\n"
		buf.WriteString(fmt.Sprintf(format, goIdent))
	}
	if len(ext.Classes) > 0 {
		buf.WriteString("\t\t")
	}
	classStr = buf.String()
	buf.Reset()

	if len(ext.Constants) > 0 {
		buf.WriteRune('\n')
	}
	for _, constant := range constants {
		decl := ext.Constants[constant]
		if decl.Package != ext.Dirname {
			pkg := "github.com/MagicalTux/goro/" + decl.Package
			importSet[pkg] = struct{}{}
		}

		goIdent := decl.GoIdent
		if decl.Package != ext.Dirname {
			goIdent = path.Base(decl.Package) + "." + goIdent
		}

		format := "\t\t\t" + `"%s": %s,` + "\n"
		buf.WriteString(fmt.Sprintf(format, constant, goIdent))
	}
	if len(ext.Constants) > 0 {
		buf.WriteString("\t\t")
	}
	constantStr = buf.String()
	buf.Reset()

	imports := []string{}
	for k := range importSet {
		imports = append(imports, k)
	}

	sort.Strings(imports)

	if len(importSet) > 0 {
		buf.WriteRune('\n')
	}
	for _, imp := range imports {
		buf.WriteString("\t\"")
		buf.WriteString(imp)
		buf.WriteString("\"\n")
	}
	if len(importSet) > 0 {
		buf.WriteRune('\n')
	}
	importStr = buf.String()
	buf.Reset()

	output := fmt.Sprintf(template, path.Base(ext.Dirname), importStr, version, classStr, functionStr, constantStr)
	output = strings.TrimSpace(output)
	w.Write([]byte(output))
}

func processExt(dirname, destFilename string) (*ExtData, error) {
	ext := &ExtData{
		Dirname:   dirname,
		Imports:   make(map[string]string),
		Functions: make(map[string]IdentiferMap),
		Constants: make(map[string]IdentiferMap),
		Classes:   make(map[string]IdentiferMap),
	}
	err := filepath.WalkDir(dirname, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == filepath.Join(dirname, destFilename) {
			return nil
		}
		if filepath.Ext(path) == ".go" {
			processExtFile(path, ext)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return ext, nil
}

func processExtFile(filename string, ext *ExtData) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}
	pkg := path.Dir(filename)

	consts := getPhpConstants(f, pkg)
	funcs := getPhpFunctions(f, pkg)
	classes := getPhpClasses(f, pkg)

	for k, id := range consts {
		ext.Constants[k] = id
	}
	for k, id := range funcs {
		ext.Functions[k] = id
	}
	for k, id := range classes {
		ext.Classes[k] = id
	}
}

func parseFunctionSignature(s string) string {
	re := regexp.MustCompile(`\/\/\s*>\s*func\s*[\w|]+\s*(\w*)\s*\(.*\)`)
	matches := re.FindStringSubmatch(s)
	if len(matches) >= 1 {
		return matches[1]
	}
	return ""
}
func parseClassSignature(s string) (string, bool) {
	re := regexp.MustCompile(`\/\/\s*>\s*class\s*(\w*)\s*`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return "", false
	}

	var name string
	if len(matches) >= 1 {
		name = matches[1]
	}
	return name, true
}

func parseConstantSignature(s string) *string {
	re := regexp.MustCompile(`\/\/\s*>\s*const\s*(\w*)\s*`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return nil
	}

	var name string
	if len(matches) >= 1 {
		name = matches[1]
	}
	return &name
}

func getConstantSignature(commentGroup *ast.CommentGroup) *string {
	if commentGroup == nil {
		return nil
	}
	for _, comment := range commentGroup.List {
		text := parseConstantSignature(comment.Text)
		if text != nil {
			return text
		}
	}
	return nil
}

func getPhpFunctions(f *ast.File, pkg string) map[string]IdentiferMap {
	result := map[string]IdentiferMap{}
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
				result[phpIdent] = IdentiferMap{
					GoIdent:  decl.Name.Name,
					PhpIdent: phpIdent,
					Package:  pkg,
				}
			}

		}
	}

	return result
}

func getPhpClasses(f *ast.File, pkg string) map[string]IdentiferMap {
	result := map[string]IdentiferMap{}
	for _, decl := range f.Decls {
		decl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for i, spec := range decl.Specs {
			spec, ok := spec.(*ast.ValueSpec)
			if !ok || len(spec.Names) == 0 {
				continue
			}

			var phpIdent string
			ok = false

			if spec.Doc != nil {
				phpIdent, ok = parseClassSignature(spec.Doc.List[0].Text)
			}
			if !ok && i == 0 && decl.Doc != nil {
				phpIdent, ok = parseClassSignature(decl.Doc.List[0].Text)
			}

			if ok && phpIdent == "" {
				phpIdent = spec.Names[0].Name
			}

			if phpIdent != "" {
				result[phpIdent] = IdentiferMap{
					PhpIdent: phpIdent,
					GoIdent:  spec.Names[0].Name,
					Package:  pkg,
				}
			}
		}
	}
	return result
}

func getPhpConstants(f *ast.File, pkg string) map[string]IdentiferMap {
	result := map[string]IdentiferMap{}
	for _, decl := range f.Decls {

		decl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		declDoc := getConstantSignature(decl.Doc)

		for _, spec := range decl.Specs {
			spec, ok := spec.(*ast.ValueSpec)
			if !ok || len(spec.Names) == 0 {
				continue
			}

			for i, id := range spec.Names {
				specDoc := getConstantSignature(spec.Doc)

				if declDoc == nil && specDoc == nil {
					continue
				}

				if specDoc == nil {
					result[id.Name] = IdentiferMap{
						GoIdent:  id.Name,
						PhpIdent: id.Name,
						Package:  pkg,
					}
					continue
				}

				phpIdent := id.Name
				if *specDoc != "" && i == 0 {
					phpIdent = *specDoc
				}

				result[phpIdent] = IdentiferMap{
					GoIdent:  id.Name,
					PhpIdent: phpIdent,
					Package:  pkg,
				}
			}
		}
	}
	return result
}
