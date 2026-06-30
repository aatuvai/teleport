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
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"regexp"
	"strings"
)

// TypeForDecl returns a representation of the type spec of decl to use for
// further processing. Returns an error if there is either no type spec or more
// than one.
// structTagKey is the struct tag key to look for when determining field names,
// such as "json" or "yaml".
func TypeForDecl(decl DeclarationInfo, allDecls map[PackageInfo]DeclarationInfo, structTagKey string) (RawType, error) {
	gendecl, ok := decl.Decl.(*ast.GenDecl)
	if !ok {
		return RawType{}, NotAGenDeclError{}
	}

	if len(gendecl.Specs) == 0 {
		return RawType{}, errors.New("declaration has no specs")
	}

	if len(gendecl.Specs) > 1 {
		return RawType{}, errors.New("declaration contains more than one type spec")
	}

	if gendecl.Specs[0] == nil {
		return RawType{}, errors.New("no spec found")
	}

	t, ok := gendecl.Specs[0].(*ast.TypeSpec)
	if !ok {
		return RawType{}, errors.New("no type spec found")
	}

	str, ok := t.Type.(*ast.StructType)
	// The declaration is not a struct, but we may still want to include it
	// in the reference. Return a RawType with no fields.
	if !ok {
		return RawType{
			Name:   t.Name.Name,
			Doc:    gendecl.Doc.Text(),
			Fields: []RawField{},
		}, nil
	}

	// We have determined that decl is a struct type, so collect its fields.
	var rawFields []RawField
	for _, field := range str.Fields.List {
		f, err := makeRawField(field, decl.PackageName, allDecls, decl.NamedImports, structTagKey)
		if err != nil {
			return RawType{}, err
		}

		// The struct field name is lowercased, so the field is not
		// exported. Ignore it.
		if f.Name != "" && f.Name[0] >= 'a' && f.Name[0] <= 122 {
			continue
		}

		fieldName := GetTag(f.Tags, structTagKey)
		// This field is ignored, so skip it.
		// See: https://pkg.go.dev/encoding/json#Marshal
		if fieldName == "-" {
			continue
		}
		// Using the exported field declaration name as the field name
		// per JSON marshaling rules.
		if fieldName == "" {
			f.FieldName = f.Name
		}

		rawFields = append(rawFields, f)
	}

	result := RawType{
		Name: t.Name.Name,
		// Preserving newlines for downstream processing
		Doc:    gendecl.Doc.Text(),
		Fields: rawFields,
	}

	return result, nil
}

// makeRawField translates an *ast.Field into a RawField for downstream
// processing. packageName is the name of the package that includes this name in
// a struct declaration.
func makeRawField(field *ast.Field, packageName string, allDecls map[PackageInfo]DeclarationInfo, namedImports map[string]string, structTagKey string) (RawField, error) {
	doc := field.Doc.Text()
	if len(field.Names) > 1 {
		return RawField{}, fmt.Errorf("field %+v in %v contains more than one name", field, packageName)
	}

	var name string
	// Otherwise, the field is likely an embedded struct.
	if len(field.Names) == 1 {
		name = field.Names[0].Name
	}

	tn, err := getYAMLType(field, packageName, allDecls, namedImports)
	if err != nil {
		return RawField{}, err
	}

	// Indicate which package declared this field depending on whether the
	// field's type name includes the name of another package.
	pkg := packageName
	// Unwrap any pointer indirection before checking for a cross-package selector.
	if star, ok := field.Type.(*ast.StarExpr); ok {
		field.Type = star.X
	}
	s, ok := field.Type.(*ast.SelectorExpr)
	if ok {
		i, ok := s.X.(*ast.Ident)
		// Not an identifier, so don't look up imports
		if !ok {
			goto assignTag
		}

		p, imp := namedImports[i.Name]
		if !imp {
			return RawField{}, fmt.Errorf("package %v does not include an import with name %v", packageName, i.Name)
		}

		pkg = p
	}

assignTag:
	var tag string
	if field.Tag != nil {
		tag = field.Tag.Value
	}

	return RawField{
		PackageName: pkg,
		Doc:         doc,
		Kind:        tn,
		Name:        name,
		FieldName:   GetTag(tag, structTagKey),
		Tags:        tag,
	}, nil
}

// AllFieldsForDecl finds embedded structs within fld and recursively
// processes the fields of those structs as though the fields belonged to the
// containing struct. Uses decl and allDecls to look up fields within the base
// structs. Returns a modified slice of fields that include all non-embedded
// fields within fld.
// structTagKey is the struct tag key to look for when determining field names,
// such as "json" or "yaml".
func AllFieldsForDecl(decl DeclarationInfo, fld []RawField, allDecls map[PackageInfo]DeclarationInfo, structTagKey string) ([]RawField, error) {
	fieldsToProcess := []RawField{}
	for _, l := range fld {
		// Not an embedded struct field, so append it to the final
		// result.
		if l.Name != "" {
			fieldsToProcess = append(fieldsToProcess, l)
			continue
		}
		c, ok := l.Kind.(YAMLCustomType)
		// Not an embedded struct since it's not a declared type.
		if !ok {
			continue
		}

		// Find the package name to use to look up the declaration from
		// its identifier.
		var pkg string
		i, ok := decl.NamedImports[l.PackageName]
		switch {
		// The file that made the declaration provided a name for the
		// package associated with the identifier, so find the full
		// package path and use that to look up the declaration.
		case ok:
			pkg = i
		case l.PackageName != "":
			pkg = l.PackageName
		// If the field's type has no package name, assume the field's
		// package name is the same as the one for decl.
		default:
			pkg = decl.PackageName

		}
		p := PackageInfo{
			DeclName:    c.DeclarationInfo.DeclName,
			PackagePath: pkg,
		}

		// We expect to find a declaration of the embedded struct.
		d, ok := allDecls[p]
		if !ok {
			return nil, fmt.Errorf(
				"%v: field %v.%v is not declared anywhere",
				decl.FilePath,
				l.PackageName,
				c.Name,
			)
		}
		e, err := TypeForDecl(d, allDecls, structTagKey)
		if err != nil && !errors.As(err, &NotAGenDeclError{}) {
			return nil, err
		}

		// The embedded struct field may have its own embedded struct
		// fields.
		nf, err := AllFieldsForDecl(decl, e.Fields, allDecls, structTagKey)
		if err != nil {
			return nil, err
		}

		fieldsToProcess = append(fieldsToProcess, nf...)
	}
	return fieldsToProcess, nil
}

// MakeYAMLExample creates an example YAML document illustrating the fields
// within a declaration. This appears at the end of a section within the
// reference.
func MakeYAMLExample(fields []RawField, structTagKey string) (string, error) {
	var buf bytes.Buffer

	for _, field := range fields {
		example := field.Kind.formatForExampleYAML(0) + "\n"
		buf.WriteString(GetTag(field.Tags, structTagKey) + ": ")
		buf.WriteString(example)
	}

	return buf.String(), nil
}

// MakeFieldTableInfo assembles a slice of human-readable information about
// fields within a Go struct to include within the resource reference.
func MakeFieldTableInfo(fields []RawField, camelCaseExceptions []string) ([]Field, error) {
	var result []Field
	for _, field := range fields {
		var desc string
		var typ string

		desc = field.Doc
		typ = field.Kind.formatForTable(kindTableFormatOptions{
			camelCaseExceptions: camelCaseExceptions,
		})
		// Escape pipes so they do not affect table rendering.
		desc = strings.ReplaceAll(desc, "|", `\|`)
		// Remove surrounding spaces and inner line breaks.
		desc = strings.Trim(strings.ReplaceAll(desc, "\n", " "), " ")

		// Escape angle brackets so the docs engine handles them as
		// strings instead of HTML tags.
		desc = strings.ReplaceAll(desc, "<", `\<`)
		desc = strings.ReplaceAll(desc, ">", `\>`)

		result = append(result, Field{
			Description: PrintableDescription(desc, field.Name, ""),
			Name:        field.FieldName,
			Type:        typ,
		})
	}
	return result, nil
}

// GetTag returns the tag value from the provided struct tag
// expression. The "key" argument is the key within the struct tag to
// look up, such as "json" or "yaml". Returns the value associated with
// the key, or an empty string if the key is not present.
func GetTag(tags string, key string) string {
	// look for the pattern key:"value" within the struct tag.
	pattern := fmt.Sprintf(`%s:"([^"]+)"`, key)
	re := regexp.MustCompile(pattern)
	kv := re.FindStringSubmatch(tags)

	// No tag with the provided key, or a tag with the provided key but no value.
	if len(kv) != 2 {
		return ""
	}

	return strings.TrimSuffix(kv[1], ",omitempty")
}

// SortFieldsByName sorts a and b ascending by Name.
func SortFieldsByName(a, b Field) int {
	return strings.Compare(a.Name, b.Name)
}
