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

package refgen

import (
	"fmt"
	"go/ast"
)

// PackageInfo is used to look up a Go declaration in a map of declaration names
// to resource data.
type PackageInfo struct {
	// DeclName is the name of a Go declaration.
	DeclName string
	// PackagePath is the full path of a Go package containing the
	// declaration, e.g.,
	// "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	PackagePath string
}

// ReferenceEntry represents a section in the configuration reference docs.
type ReferenceEntry struct {
	SectionName string
	Description string
	SourcePath  string
	YAMLName    string
	Fields      []Field
	YAMLExample string
}

// DeclarationInfo includes data about a declaration so the generator can
// convert it into a ReferenceEntry.
type DeclarationInfo struct {
	FilePath    string
	Decl        ast.Decl
	PackageName string
	// Maps the file-scoped name of each import (if given) to the
	// corresponding full package path.
	NamedImports map[string]string
}

// Field represents a row in a table that provides information about a field in
// the resource reference.
type Field struct {
	Name        string
	Description string
	Type        string
}

type SourceData struct {
	// TypeDecls maps package and declaration names to data that the generator
	// uses to format documentation for dynamic resource fields.
	TypeDecls map[PackageInfo]DeclarationInfo
}

// rawField contains simplified information about a struct field type. This
// prevents passing around AST nodes and makes testing easier.
type RawField struct {
	// Package that declares the field type
	PackageName string
	// A declaration's GoDoc, including newline characters but not comment
	// characters.
	Doc string
	// The type of the field.
	Kind yamlKindNode
	// Original name of the field.
	Name string
	// Name as it appears in YAML, based on the struct tag and
	// marshaling rules in the encoding/json package.
	FieldName string
	// The entire struct tag expression for the field.
	Tags string
}

// RawType contains simplified information about a type, which may or may not be
// a struct. This prevents passing around AST nodes and makes testing easier.
type RawType struct {
	// A declaration's GoDoc, including newline characters but not comment
	// characters.
	Doc string
	// The name of the type declaration.
	Name string
	// Struct fields within the type. Empty if not a struct.
	Fields []RawField
}

type NotAGenDeclError struct{}

func (e NotAGenDeclError) Error() string {
	return "the declaration is not a GenDecl"
}

// GenerationError aggregates multiple errors that occur during the generation of a
// reference. Used in the Generate function of a reference generator to report all
// errors that occur during generation.
type GenerationError struct {
	Messages []error
}

func (g GenerationError) Error() string {
	// Begin with a newline to format the first list item below the outer
	// error.
	final := "\n"
	for _, e := range g.Messages {
		final += fmt.Sprintf("- %v\n", e)
	}
	return final
}
