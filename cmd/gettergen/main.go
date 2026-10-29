// Command gettergen generates getters for private struct fields.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func main() {
	typeNames := flag.String("types", "", "optional comma-separated struct type filter (defaults to all structs)")
	pointers := flag.Bool("pointers", false, "generate pointer receivers for all types by default")
	receivers := flag.String("receivers", "", "optional receiver overrides: Type=name or Type=*name")
	output := flag.String("output", "getters_gen.go", "generated file path")
	flag.Parse()

	receiverSpecs, err := parseReceivers(*receivers)
	if err != nil {
		fatalf("%v", err)
	}
	var types []string
	if strings.TrimSpace(*typeNames) != "" {
		types = strings.Split(*typeNames, ",")
	}
	generated, err := generate(types, receiverSpecs, *pointers, *output)
	if err != nil {
		fatalf("%v", err)
	}
	if err := os.WriteFile(generated.path, generated.content, 0o644); err != nil {
		fatalf("write %s: %v", generated.path, err)
	}
}

type generatedFile struct {
	path    string
	content []byte
}

type structDeclaration struct {
	fields  *ast.StructType
	imports map[string]string
}

type receiverSpec struct {
	name       string
	pointer    bool
	pointerSet bool
}

func parseReceivers(spec string) (map[string]receiverSpec, error) {
	receivers := make(map[string]receiverSpec)
	if strings.TrimSpace(spec) == "" {
		return receivers, nil
	}
	for _, item := range strings.Split(spec, ",") {
		typeName, receiver, found := strings.Cut(strings.TrimSpace(item), "=")
		if !found || typeName == "" || receiver == "" {
			return nil, fmt.Errorf("invalid receiver spec %q; expected Type=name or Type=*name", item)
		}
		pointer := strings.HasPrefix(receiver, "*")
		if pointer {
			receiver = strings.TrimPrefix(receiver, "*")
		}
		if !token.IsIdentifier(receiver) || token.Lookup(receiver).IsKeyword() {
			return nil, fmt.Errorf("invalid receiver name %q for %s", receiver, typeName)
		}
		receivers[typeName] = receiverSpec{name: receiver, pointer: pointer, pointerSet: true}
	}
	return receivers, nil
}

func generate(typeNames []string, receivers map[string]receiverSpec, pointers bool, output string) (generatedFile, error) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		return generatedFile{}, fmt.Errorf("list Go files: %w", err)
	}

	fset := token.NewFileSet()
	var packageName string
	targets := make(map[string]structDeclaration, len(typeNames))
	requestedTypes := len(typeNames) > 0
	for _, typeName := range typeNames {
		typeName = strings.TrimSpace(typeName)
		if typeName == "" {
			return generatedFile{}, fmt.Errorf("type name cannot be empty")
		}
		targets[typeName] = structDeclaration{}
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_gen.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return generatedFile{}, fmt.Errorf("parse %s: %w", path, err)
		}
		if packageName == "" {
			packageName = file.Name.Name
		}
		imports := make(map[string]string, len(file.Imports))
		for _, importSpec := range file.Imports {
			importPath, err := strconv.Unquote(importSpec.Path.Value)
			if err != nil {
				return generatedFile{}, fmt.Errorf("parse import path in %s: %w", path, err)
			}
			localName := filepath.Base(importPath)
			if importSpec.Name != nil {
				localName = importSpec.Name.Name
			}
			imports[localName] = importSpec.Path.Value
		}
		for _, declaration := range file.Decls {
			genDecl, ok := declaration.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				_, requested := targets[typeSpec.Name.Name]
				if requestedTypes && !requested {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					if requestedTypes {
						return generatedFile{}, fmt.Errorf("%s is not a struct", typeSpec.Name.Name)
					}
					continue
				}
				targets[typeSpec.Name.Name] = structDeclaration{fields: structType, imports: imports}
			}
		}
	}
	if len(targets) == 0 {
		return generatedFile{}, fmt.Errorf("no structs found in current package")
	}
	for typeName, target := range targets {
		if requestedTypes && target.fields == nil {
			return generatedFile{}, fmt.Errorf("struct %s not found in current package", typeName)
		}
	}

	typeNames = typeNames[:0]
	for typeName := range targets {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)

	var methods bytes.Buffer
	generatedImports := make(map[string]string)
	for _, typeName := range typeNames {
		target := targets[typeName]
		receiver := receiverSpec{name: receiverName(typeName), pointer: pointers}
		if override, ok := receivers[typeName]; ok {
			receiver.name = override.name
			if override.pointerSet {
				receiver.pointer = override.pointer
			}
		}
		receiverType := typeName
		if receiver.pointer {
			receiverType = "*" + receiverType
		}
		for _, field := range target.fields.Fields.List {
			if len(field.Names) == 0 {
				continue
			}
			var fieldType bytes.Buffer
			if err := format.Node(&fieldType, fset, field.Type); err != nil {
				return generatedFile{}, fmt.Errorf("format field type: %w", err)
			}
			ast.Inspect(field.Type, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				packageIdent, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				if importPath, ok := target.imports[packageIdent.Name]; ok {
					generatedImports[packageIdent.Name] = importPath
				}
				return true
			})
			for _, name := range field.Names {
				if ast.IsExported(name.Name) {
					continue
				}
				getter, include, err := getterName(name.Name, field.Type, field.Tag)
				if err != nil {
					return generatedFile{}, fmt.Errorf("%s.%s: %w", typeName, name.Name, err)
				}
				if !include {
					continue
				}
				fmt.Fprintf(&methods, "func (%s %s) %s() %s { return %s.%s }\n", receiver.name, receiverType, getter, fieldType.String(), receiver.name, name.Name)
			}
		}
	}
	var source bytes.Buffer
	fmt.Fprintf(&source, "// Code generated by gettergen; DO NOT EDIT.\n\npackage %s\n\n", packageName)
	if len(generatedImports) > 0 {
		source.WriteString("import (\n")
		importNames := make([]string, 0, len(generatedImports))
		for importName := range generatedImports {
			importNames = append(importNames, importName)
		}
		sort.Strings(importNames)
		for _, importName := range importNames {
			fmt.Fprintf(&source, "\t%s %s\n", importName, generatedImports[importName])
		}
		source.WriteString(")\n\n")
	}
	source.Write(methods.Bytes())

	formatted, err := format.Source(source.Bytes())
	if err != nil {
		return generatedFile{}, fmt.Errorf("format generated source: %w", err)
	}
	return generatedFile{path: output, content: formatted}, nil
}

func receiverName(value string) string {
	r, _ := utf8.DecodeRuneInString(value)
	return string(unicode.ToLower(r))
}

func getterName(fieldName string, fieldType ast.Expr, tag *ast.BasicLit) (string, bool, error) {
	if tag != nil {
		raw, err := strconv.Unquote(tag.Value)
		if err != nil {
			return "", false, fmt.Errorf("invalid struct tag: %w", err)
		}
		if override, ok := reflect.StructTag(raw).Lookup("getter"); ok {
			if override == "-" {
				return "", false, nil
			}
			if !token.IsIdentifier(override) || token.Lookup(override).IsKeyword() || !ast.IsExported(override) {
				return "", false, fmt.Errorf("invalid getter name %q", override)
			}
			return override, true, nil
		}
	}
	getter := exportedFieldName(fieldName)
	if isBoolType(fieldType) && !hasIsPrefix(fieldName) {
		getter = "Is" + getter
	}
	return getter, true, nil
}

func isBoolType(fieldType ast.Expr) bool {
	ident, ok := fieldType.(*ast.Ident)
	return ok && ident.Name == "bool"
}

func hasIsPrefix(fieldName string) bool {
	words := splitWords(fieldName)
	return len(words) > 1 && strings.EqualFold(words[0], "is")
}

var initialisms = map[string]string{
	"id": "ID", "uid": "UID", "api": "API", "ascii": "ASCII", "cpu": "CPU",
	"css": "CSS", "dns": "DNS", "eof": "EOF", "guid": "GUID", "html": "HTML",
	"http": "HTTP", "https": "HTTPS", "ip": "IP", "json": "JSON", "qps": "QPS",
	"ram": "RAM", "rpc": "RPC", "sla": "SLA", "smtp": "SMTP", "sql": "SQL",
	"ssh": "SSH", "tcp": "TCP", "tls": "TLS", "ttl": "TTL", "udp": "UDP",
	"ui": "UI", "uri": "URI", "url": "URL", "utf8": "UTF8", "uuid": "UUID",
	"vm": "VM", "xml": "XML", "xsrf": "XSRF", "xss": "XSS",
}

func exportedFieldName(value string) string {
	words := splitWords(value)
	var result strings.Builder
	for _, word := range words {
		if acronym, ok := initialisms[strings.ToLower(word)]; ok {
			result.WriteString(acronym)
		} else {
			first, size := utf8.DecodeRuneInString(word)
			result.WriteRune(unicode.ToUpper(first))
			result.WriteString(word[size:])
		}
	}
	return result.String()
}

func splitWords(value string) []string {
	runes := []rune(value)
	words := make([]string, 0, 4)
	start := 0
	for i, current := range runes {
		if current == '_' {
			if start < i {
				words = append(words, string(runes[start:i]))
			}
			start = i + 1
			continue
		}
		if i == start {
			continue
		}
		previous := runes[i-1]
		nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
		if unicode.IsUpper(current) && (unicode.IsLower(previous) || unicode.IsDigit(previous) || (unicode.IsUpper(previous) && nextIsLower)) {
			words = append(words, string(runes[start:i]))
			start = i
		}
	}
	if start < len(runes) {
		words = append(words, string(runes[start:]))
	}
	return words
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gettergen: "+format+"\n", args...)
	os.Exit(1)
}
