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
	"strings"
)

// kindTableFormatOptions configures the way the generator formats YAML kinds
// for the field table in a reference page.
type kindTableFormatOptions struct {
	// camelCaseExceptions is a list of strings to exempt when splitting
	// camel-case words.
	camelCaseExceptions []string
}

// yamlKindNode represents a node in a potentially recursive YAML type, such as
// an integer, a map of integers to strings, a sequence of maps of strings to
// strings, etc. Used for printing example YAML documents and tables of fields.
// This is not intended to be a comprehensive YAML AST.
type yamlKindNode interface {
	// Generate a string representation to include in a table of fields.
	formatForTable(kindTableFormatOptions) string
	// Generate an example YAML value for the type with the provided number
	// of indendations.
	formatForExampleYAML(indents int) string
	// Get the custom children of this yamlKindNode. Must call
	// CustomFieldData on its own children before returning.
	CustomFieldData() []PackageInfo
}

// nonYAMLKind represents a field type that we cannot convert to YAML. Consumers
// should return an error if there is no way to avoid creating a reference entry
// for this kind.
type nonYAMLKind struct{}

func (n nonYAMLKind) formatForTable(opts kindTableFormatOptions) string {
	return ""
}

func (n nonYAMLKind) formatForExampleYAML(indents int) string {
	return "# See description"
}

func (n nonYAMLKind) CustomFieldData() []PackageInfo {
	return []PackageInfo{}
}

// YAMLSequence is a list of elements.
type YAMLSequence struct {
	ElementKind yamlKindNode
}

func (y YAMLSequence) formatForTable(opts kindTableFormatOptions) string {
	return `[]` + y.ElementKind.formatForTable(opts)
}

func (y YAMLSequence) formatForExampleYAML(indents int) string {
	var leading string
	indents++
	for i := 0; i < indents; i++ {
		leading += "  "
	}
	el := y.ElementKind.formatForExampleYAML(indents)
	// Trim leading indentation since each element is already indented.
	el = strings.TrimLeft(el, " ")
	// Always start a sequence on a new line
	return fmt.Sprintf(`
%v- %v
%v- %v
%v- %v`,
		leading, el,
		leading, el,
		leading, el,
	)
}

func (y YAMLSequence) CustomFieldData() []PackageInfo {
	return y.ElementKind.CustomFieldData()
}

// YALMapping is a mapping of keys to values.
type YAMLMapping struct {
	KeyKind   yamlKindNode
	ValueKind yamlKindNode
}

func (y YAMLMapping) formatForExampleYAML(indents int) string {
	var leading string
	// Add an extra indent for mappings
	indents = indents + 1
	for i := 0; i < indents; i++ {
		leading += "  "
	}

	val := y.ValueKind.formatForExampleYAML(indents)
	// Remove leading indentation on the first line of the value since the
	// key/value pair is already indented. This does not affect subsequent
	// lines of the value.
	val = strings.TrimLeft(val, " ")

	kv := fmt.Sprintf("%v%v: %v", leading, y.KeyKind.formatForExampleYAML(0), val)
	return fmt.Sprintf("\n%v\n%v\n%v", kv, kv, kv)
}

func (y YAMLMapping) formatForTable(opts kindTableFormatOptions) string {
	return fmt.Sprintf("map[%v]%v", y.KeyKind.formatForTable(opts), y.ValueKind.formatForTable(opts))
}

func (y YAMLMapping) CustomFieldData() []PackageInfo {
	k := y.KeyKind.CustomFieldData()
	v := y.ValueKind.CustomFieldData()
	return append(k, v...)
}

type YAMLString struct{}

func (y YAMLString) formatForTable(opts kindTableFormatOptions) string {
	return "string"
}

func (y YAMLString) formatForExampleYAML(indents int) string {
	return `"string"`
}

func (y YAMLString) CustomFieldData() []PackageInfo {
	return []PackageInfo{}
}

type yamlBase64 struct{}

func (y yamlBase64) formatForTable(opts kindTableFormatOptions) string {
	return "base64-encoded string"
}

func (y yamlBase64) formatForExampleYAML(indents int) string {
	return "BASE64_STRING"
}

func (y yamlBase64) CustomFieldData() []PackageInfo {
	return []PackageInfo{}
}

type YAMLNumber struct{}

func (y YAMLNumber) formatForTable(opts kindTableFormatOptions) string {
	return "number"
}

func (y YAMLNumber) formatForExampleYAML(indents int) string {
	return "1"
}

func (y YAMLNumber) CustomFieldData() []PackageInfo {
	return []PackageInfo{}
}

type YAMLBool struct{}

func (y YAMLBool) formatForTable(opts kindTableFormatOptions) string {
	return "Boolean"
}

func (y YAMLBool) formatForExampleYAML(indents int) string {
	return "true"
}

func (y YAMLBool) CustomFieldData() []PackageInfo {
	return []PackageInfo{}
}

// A type declared by the program, i.e., not one of Go's predeclared types.
type YAMLCustomType struct {
	Name string
	// Used to look up more information about the declaration of the custom
	// type so we can populate additional reference entries.
	DeclarationInfo PackageInfo
}

func (y YAMLCustomType) CustomFieldData() []PackageInfo {
	return []PackageInfo{
		y.DeclarationInfo,
	}
}

func (y YAMLCustomType) formatForExampleYAML(indents int) string {
	var leading string
	for i := 0; i < indents; i++ {
		leading += "  "
	}

	return leading + "# [...]"
}

func (y YAMLCustomType) formatForTable(opts kindTableFormatOptions) string {
	name := SplitCamelCase(y.Name, opts.camelCaseExceptions)
	return fmt.Sprintf(
		"[%v](#%v)",
		name,
		strings.ReplaceAll(strings.ToLower(name), " ", "-"),
	)
}
