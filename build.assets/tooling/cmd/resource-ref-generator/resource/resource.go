// Teleport
// Copyright (C) 2025  Gravitational, Inc.
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

package resource

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gravitational/teleport/build.assets/tooling/lib/refgen"
)

// NewSourceData extracts type declarations from the Go files rooted at
// rootPath. Uses prefix, e.g., github.com/gravitational/teleport, to construct
// package paths.
func NewSourceData(prefix string, rootPath string) (refgen.SourceData, error) {
	// All declarations within the source tree. We use this to extract
	// information about dynamic resource fields, which we can look up by
	// package and declaration name.
	typeDecls := make(map[refgen.PackageInfo]refgen.DeclarationInfo)

	// Load each file in the source directory individually. Not using
	// packages.Load here since the resulting []*Package does not expose
	// individual file names, which we need so contributors who want to edit
	// the resulting docs page know which files to modify.
	err := filepath.Walk(rootPath, func(currentPath string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("loading Go source: %w", err)
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(info.Name()) != ".go" {
			return nil
		}

		// Skip protoopaque files. For API_HYBRID proto files, two versions are
		// generated: a regular .pb.go with exported fields and a
		// _protoopaque.pb.go with hidden fields. We only want to document the
		// regular version with exported fields.
		if strings.HasSuffix(info.Name(), "_protoopaque.pb.go") {
			return nil
		}

		// Find the Go package path corresponding to the current file.
		rel, err := filepath.Rel(rootPath, currentPath)
		if err != nil {
			return fmt.Errorf("unable to find a relative path between %v and %v: %w", rootPath, currentPath, err)
		}
		pkg := path.Join(
			prefix,
			filepath.Base(rootPath),
			filepath.Dir(rel),
		)

		// Open the file so we can pass it to ParseFile. Otherwise,
		// ParseFile always reads from the OS FS, not from fs.
		f, err := os.Open(currentPath)
		if err != nil {
			return err
		}
		defer f.Close()
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, currentPath, f, parser.ParseComments)
		if err != nil {
			return err
		}

		// Use a relative path from the source directory for cleaner
		// paths
		relDeclPath, err := filepath.Rel(rootPath, currentPath)
		if err != nil {
			return err
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
	})
	if err != nil {
		return refgen.SourceData{}, fmt.Errorf("loading Go source files: %w", err)
	}
	return refgen.SourceData{
		TypeDecls: typeDecls,
	}, nil
}

// ReferenceDataFromDeclaration gets data for the reference by examining decl.
// Looks up decl's fields in allDecls and methods in allMethods. Uses prefix,
// e.g., "github.com/gravitational/teleport", to construct package paths
func ReferenceDataFromDeclaration(
	prefix string,
	decl refgen.DeclarationInfo,
	allDecls map[refgen.PackageInfo]refgen.DeclarationInfo,
	camelCaseExceptions []string,
) (map[refgen.PackageInfo]refgen.ReferenceEntry, error) {
	rs, err := refgen.TypeForDecl(decl, allDecls, "json")
	if err != nil {
		return nil, err
	}

	fieldsToProcess, err := refgen.AllFieldsForDecl(decl, rs.Fields, allDecls, "json")
	if err != nil {
		return nil, err
	}

	description := rs.Doc
	var example string

	example, err = refgen.MakeYAMLExample(fieldsToProcess, "json")
	if err != nil {
		return nil, err
	}

	// Initialize the return value and insert the root reference entry
	// provided by decl.
	refs := make(map[refgen.PackageInfo]refgen.ReferenceEntry)
	description = strings.Trim(strings.ReplaceAll(description, "\n", " "), " ")
	entry := refgen.ReferenceEntry{
		SectionName: refgen.SplitCamelCase(rs.Name, camelCaseExceptions),
		Description: refgen.PrintableDescription(description, rs.Name, ""),
		SourcePath:  decl.FilePath,
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
			r, err := ReferenceDataFromDeclaration(prefix, gd, allDecls, camelCaseExceptions)
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
