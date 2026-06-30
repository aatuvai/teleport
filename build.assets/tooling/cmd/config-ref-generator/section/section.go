// Teleport
// Copyright (C) 2026  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package section

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	refgen "github.com/gravitational/teleport/build.assets/tooling/lib/refgen"
)

// ToModuleRelativePath converts a relative path relative to the module. E.g.
// "../../../../lib/config/fileconfig.go" is converted to "lib/config/fileconfig.go".
func ToModuleRelativePath(p string) string {
	for strings.HasPrefix(p, "../") {
		p = strings.TrimPrefix(p, "../")
	}
	return p
}

// NewSourceData returns a SourceData containing information about all declarations
// within the source file and its imports. Returns an error if the source file cannot
// be opened or parsed. Uses prefix, e.g., github.com/gravitational/teleport, to
// construct package paths.
func NewSourceData(prefix string, source string) (refgen.SourceData, error) {
	// All declarations within the source tree. We use this to extract
	// information about configuration section fields, which we can look up by
	// package and declaration name.
	typeDecls := make(map[refgen.PackageInfo]refgen.DeclarationInfo)

	// The source file should be a Go file since we need to parse it and its imports for declarations
	if filepath.Ext(source) != ".go" {
		return refgen.SourceData{}, fmt.Errorf("source path %v is not a Go file", source)
	}

	// load the source file's declarations into typeDecls, which we will reference for all
	// subsequent processing, since the source file should contain the section types we want
	// to include in the reference.
	if err := registerDeclsFromFile(prefix, source, typeDecls); err != nil {
		return refgen.SourceData{}, err
	}

	moduleRelativePath := ToModuleRelativePath(source)
	sourceDir := path.Dir(moduleRelativePath)
	moduleRoot := filepath.Dir(source)
	for _, seg := range strings.Split(sourceDir, "/") {
		if seg != "" && seg != "." {
			moduleRoot = filepath.Dir(moduleRoot)
		}
	}

	f, err := os.Open(source)
	if err != nil {
		return refgen.SourceData{}, fmt.Errorf("opening source file %v: %w", source, err)
	}
	defer f.Close()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, source, f, parser.ParseComments)
	if err != nil {
		return refgen.SourceData{}, fmt.Errorf("parsing source file %v: %w", source, err)
	}

	// Look at each imported package in the source file and load its declarations.
	// Only look at imports with the provided prefix e.g. github.com/gravitational/teleport
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if !strings.HasPrefix(importPath, prefix+"/") {
			continue
		}
		relPkg := strings.TrimPrefix(importPath, prefix+"/")
		pkgDir := filepath.Join(moduleRoot, filepath.FromSlash(relPkg))

		entries, err := os.ReadDir(pkgDir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return refgen.SourceData{}, fmt.Errorf("reading %v: %w", pkgDir, err)
		}

		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			if err := registerDeclsFromFile(prefix, filepath.Join(pkgDir, name), typeDecls); err != nil {
				return refgen.SourceData{}, err
			}
		}
	}

	return refgen.SourceData{TypeDecls: typeDecls}, nil
}

// registerDeclsFromFile parses the provided Go source file and registers all type declarations within it to typeDecls
func registerDeclsFromFile(prefix string, source string, typeDecls map[refgen.PackageInfo]refgen.DeclarationInfo) error {
	relDeclPath := ToModuleRelativePath(source)

	pkg := path.Join(
		prefix,
		filepath.Dir(relDeclPath),
	)

	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("opening source file %v: %w", source, err)
	}
	defer f.Close()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, source, f, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing source file %v: %w", source, err)
	}

	// Collect information from each file:
	// - Imported packages and their aliases
	// - Possible function declarations (for identifying relevant
	//   methods later)
	// - Type declarations
	pn := refgen.NamedImports(file)
	for _, decl := range file.Decls {
		l, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if len(l.Specs) != 1 {
			continue
		}
		spec, ok := l.Specs[0].(*ast.TypeSpec)
		if !ok {
			continue
		}

		typeDecls[refgen.PackageInfo{
			DeclName:    spec.Name.Name,
			PackagePath: pkg,
		}] = refgen.DeclarationInfo{
			Decl:         l,
			FilePath:     relDeclPath,
			PackageName:  pkg,
			NamedImports: pn,
		}
	}
	return nil
}

// printableTitle converts a yaml name of a configuration section such as "app_service" to "App Service".
// It uses the title_word_replacements config to convert known abbreviations such as "app" to "Application".
// // ident is the name of a Go identifier. Returns ident if yamlName is empty.
func printableTitle(ident, yamlName string, camelCaseExceptions []string, titleWordReplacements []map[string]string) string {
	if yamlName == "" {
		return refgen.SplitCamelCase(ident, camelCaseExceptions)
	}
	words := strings.Split(yamlName, "_")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
			for _, replacement := range titleWordReplacements {
				for word, repl := range replacement {
					if strings.EqualFold(w, word) {
						words[i] = repl
					}
				}
			}
		}

	}
	return strings.Join(words, " ")
}

// getSectionYAMLName returns the YAML tag name declared for a configuration section of the FileConfig structure.
// The bool return value is true only when the type was found as a direct field of FileConfig.
// When not found, typeName is returned as a fallback.
func getSectionYAMLName(allDecls map[refgen.PackageInfo]refgen.DeclarationInfo, typeName string) (string, bool) {
	var fileConfigDecl *refgen.DeclarationInfo
	for info, d := range allDecls {
		if info.DeclName == "FileConfig" {
			fileConfigDecl = &d
			break
		}
	}
	if fileConfigDecl == nil {
		return typeName, false
	}
	fcType, err := refgen.TypeForDecl(*fileConfigDecl, allDecls, "yaml")

	if err != nil {
		return typeName, false
	}
	for _, f := range fcType.Fields {
		custom, ok := f.Kind.(refgen.YAMLCustomType)
		if ok && custom.Name == typeName {
			return f.FieldName, true
		}
	}
	return typeName, false
}

// ReferenceDataFromDeclaration gets data for the reference by examining decl.
// Looks up decl's fields in allDecls and methods in allMethods. Uses prefix,
// e.g., "github.com/gravitational/teleport", to construct package paths
func ReferenceDataFromDeclaration(
	prefix string,
	decl refgen.DeclarationInfo,
	allDecls map[refgen.PackageInfo]refgen.DeclarationInfo,
	camelCaseExceptions []string,
	titleWordReplacements []map[string]string,
) (map[refgen.PackageInfo]refgen.ReferenceEntry, error) {
	rs, err := refgen.TypeForDecl(decl, allDecls, "yaml")
	if err != nil {
		return nil, err
	}

	fieldsToProcess, err := refgen.AllFieldsForDecl(decl, rs.Fields, allDecls, "yaml")
	if err != nil {
		return nil, err
	}

	description := rs.Doc
	var example string

	example, err = refgen.MakeYAMLExample(fieldsToProcess, "yaml")
	if err != nil {
		return nil, err
	}

	// Initialize the return value and insert the root reference entry
	// provided by decl.
	refs := make(map[refgen.PackageInfo]refgen.ReferenceEntry)
	description = strings.Trim(strings.ReplaceAll(description, "\n", " "), " ")
	yamlName, found := getSectionYAMLName(allDecls, rs.Name)
	// Only use the yaml section name for generating the title and description
	// when the type was actually found as a direct field of FileConfig. For
	// child types, use the Go type name to derive the title and docstring
	// comment for the description.
	var sectionYAMLName string
	if found {
		sectionYAMLName = yamlName
	}
	entry := refgen.ReferenceEntry{
		SectionName: printableTitle(rs.Name, sectionYAMLName, camelCaseExceptions, titleWordReplacements),
		Description: refgen.PrintableDescription(description, rs.Name, sectionYAMLName),
		SourcePath:  decl.FilePath,
		YAMLName:    yamlName,
		YAMLExample: example,
		Fields:      []refgen.Field{},
	}
	key := refgen.PackageInfo{
		DeclName:    rs.Name,
		PackagePath: decl.PackageName,
	}

	fld, err := refgen.MakeFieldTableInfo(fieldsToProcess, camelCaseExceptions)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(fld, refgen.SortFieldsByName)
	entry.Fields = fld
	refs[key] = entry

	// For any fields within decl that have a custom type, look up the
	// declaration for that type and create a separate reference entry for
	// it.
	for _, f := range fieldsToProcess {
		// Don't make separate reference entries for embedded structs
		// since they are part of the containing struct for the purposes
		// of unmarshaling YAML.
		//
		if f.Name == "" {
			continue
		}

		c := f.Kind.CustomFieldData()

		for _, d := range c {
			// Find the package name to use to look up the declaration from
			// its identifier.
			i, ok := decl.NamedImports[d.PackagePath]
			// The file that made the declaration provided a name for the
			// package associated with the identifier, so find the full
			// package path and use that to look up the declaration.
			if ok {
				d.PackagePath = i
			}

			// Get information about the field type's declaration.
			// If we can't find it, it means the field type was
			// probably declared in the standard library or
			// third-party package. In this case, leave it to the
			// GoDoc to describe the field type.
			gd, ok := allDecls[d]
			if !ok {
				continue
			}
			r, err := ReferenceDataFromDeclaration(prefix, gd, allDecls, camelCaseExceptions, titleWordReplacements)
			if errors.As(err, &refgen.NotAGenDeclError{}) {
				continue
			}
			if err != nil {
				return nil, err
			}

			for k, v := range r {
				slices.SortFunc(v.Fields, refgen.SortFieldsByName)
				refs[k] = v
			}
		}
	}
	return refs, nil
}
