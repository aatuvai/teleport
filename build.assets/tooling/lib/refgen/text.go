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
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

var camelCaseWordBoundary = regexp.MustCompile(`([a-z0-9])([A-Z][a-z0-9])`)

// curlyBracePairPattern matches a pair of curly braces, with a capture group
// for the content enclosed by the braces.
var curlyBracePairPattern = regexp.MustCompile(`\{([^}]*)\}`)

// splitCamelCase edits the original name of a declaration to make it more
// suitable as a section within the resource reference.
func SplitCamelCase(original string, camelCaseExceptions []string) string {
	exceptionMap := make(map[string]struct{})
	for _, e := range camelCaseExceptions {
		exceptionMap[e] = struct{}{}
	}

	// Ensure that each exception occupies its own word. This way, we can
	// feed each exception-only word to the result and split the remaining
	// camel case boundaries.
	exceptions := regexp.MustCompile(
		fmt.Sprintf("(%v)", strings.Join(camelCaseExceptions, "|")),
	)
	split := exceptions.ReplaceAllString(original, " $1 ")
	words := bufio.NewScanner(strings.NewReader(split))

	// Iterate through the words we have so far, preserving exceptions and
	// splitting the remaining camel-cased words.
	var result bytes.Buffer
	words.Split(bufio.ScanWords)
	for words.Scan() {
		word := words.Text()
		if _, ok := exceptionMap[word]; ok {
			result.WriteString(word + " ")
			continue
		}
		result.WriteString(camelCaseWordBoundary.ReplaceAllString(word, "$1 $2") + " ")
	}

	return strings.Trim(result.String(), " ")
}

// PrintableDescription modifies a field or type description to make it suitable
// for reading on a docs page.
//
// ident is the name of a Go identifier. PrintableDescription removes the name
// from the description so we can include it within the configuration reference,
// fixing capitalization issues resulting from removing the name. Since the
// identifier's name within the source won't mean anything to a docs reader,
// removing it makes the description easier to read.
//
// Since curly brace pairs break docs site builds, PrintableDescription also
// encloses any curly brace pairs with backticks.
func PrintableDescription(description, ident, yamlName string) string {

	if yamlName != "" {
		return fmt.Sprintf("Represents the `%v` section of the configuration file.", yamlName)
	}

	result := curlyBracePairPattern.ReplaceAllString(description, "`{$1}`")
	// Replace any double-backticks resulting from the previous operation.
	// This is a hack to avoid the need for more complex logic. Double
	// backticks won't render as expected in the docs anyway, so it's fine
	// to replace ones that don't result from the earlier replacement.
	result = strings.ReplaceAll(result, "``", "`")

	if len(ident) <= len(result) {
		switch {
		case strings.HasPrefix(result, ident+" are "):
			result = strings.TrimPrefix(result, ident+" are ")
		case strings.HasPrefix(result, ident+" is "):
			result = strings.TrimPrefix(result, ident+" is ")
		case strings.HasPrefix(result, ident+" "):
			result = strings.TrimPrefix(result, ident+" ")
		case strings.HasPrefix(result, ident):
			result = strings.TrimPrefix(result, ident)
		}
	}

	// Make sure the result begins with a capital letter
	if len(result) > 0 {
		result = strings.ToUpper(result[:1]) + result[1:]
	}

	// Not possible to trim the name from description
	return result
}
