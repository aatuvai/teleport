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
	"go/ast"
	"path"
	"strings"
)

// getYAMLTypeForExpr takes an AST type expression and recursively
// traverses it to populate a yamlKindNode. Each iteration converts a
// single *ast.Expr into a single yamlKindNode, returning the new node.
func getYAMLTypeForExpr(exp ast.Expr, pkg string, allDecls map[PackageInfo]DeclarationInfo, namedImports map[string]string) (yamlKindNode, error) {
	switch t := exp.(type) {
	case *ast.StarExpr:
		// Ignore the star, since YAML fields are unmarshaled as the
		// values they point to.
		return getYAMLTypeForExpr(t.X, pkg, allDecls, namedImports)
	case *ast.Ident:
		switch t.Name {
		case "string":
			return YAMLString{}, nil
		case "uint", "uint8", "uint16", "uint32", "uint64", "int", "int8", "int16", "int32", "int64", "float32", "float64":
			return YAMLNumber{}, nil
		case "bool":
			return YAMLBool{}, nil
		default:
			info := PackageInfo{
				DeclName:    t.Name,
				PackagePath: pkg,
			}
			if _, ok := allDecls[info]; !ok {
				return nonYAMLKind{}, nil
			}

			return YAMLCustomType{
				Name:            t.Name,
				DeclarationInfo: info,
			}, nil
		}
	case *ast.MapType:
		k, err := getYAMLTypeForExpr(t.Key, pkg, allDecls, namedImports)
		if err != nil {
			return nil, err
		}

		v, err := getYAMLTypeForExpr(t.Value, pkg, allDecls, namedImports)
		if err != nil {
			return nil, err
		}
		return YAMLMapping{
			KeyKind:   k,
			ValueKind: v,
		}, nil
	case *ast.ArrayType:
		// Bite slices marshal to base64 strings
		if isByteSlice(t) {
			return yamlBase64{}, nil
		}
		e, err := getYAMLTypeForExpr(t.Elt, pkg, allDecls, namedImports)
		if err != nil {
			return nil, err
		}
		return YAMLSequence{
			ElementKind: e,
		}, nil
	case *ast.SelectorExpr:
		var pkg string
		x, ok := t.X.(*ast.Ident)
		if ok {
			pkg = x.Name
			if i, ok := namedImports[x.Name]; ok {
				pkg = i
			}
		}
		info := PackageInfo{
			DeclName:    t.Sel.Name,
			PackagePath: pkg,
		}
		if _, ok := allDecls[info]; !ok {
			return nonYAMLKind{}, nil
		}

		return YAMLCustomType{
			Name:            t.Sel.Name,
			DeclarationInfo: info,
		}, nil
	default:
		return nonYAMLKind{}, nil
	}
}

// getYAMLType returns YAML type information for a struct field so we can print
// information about it in the resource reference.
func getYAMLType(field *ast.Field, pkg string, allDecls map[PackageInfo]DeclarationInfo, namedImports map[string]string) (yamlKindNode, error) {
	return getYAMLTypeForExpr(field.Type, pkg, allDecls, namedImports)
}

// isByteSlice returns whether t is a []byte.
func isByteSlice(t *ast.ArrayType) bool {
	i, ok := t.Elt.(*ast.Ident)
	if !ok {
		return false
	}
	return i.Name == "byte"
}

// NamedImports creates a mapping from the provided name of each package import
// to the original package path. If the package does not have an explicit name,
// map the full path to the final path segment instead.
func NamedImports(file *ast.File) map[string]string {
	m := make(map[string]string)
	for _, i := range file.Imports {
		pkgPath := strings.Trim(i.Path.Value, "\"")
		if i.Name == nil {
			m[path.Base(pkgPath)] = pkgPath
		} else {
			m[i.Name.Name] = pkgPath
		}
	}
	return m
}
