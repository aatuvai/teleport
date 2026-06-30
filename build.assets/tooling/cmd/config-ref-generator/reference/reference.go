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

package reference

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	template "github.com/DataDog/datadog-agent/pkg/template/text"

	"github.com/gravitational/teleport/build.assets/tooling/lib/refgen"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/config-ref-generator/section"
)

// pageContent represents a page for a section of the configuration reference. Fields must be exported so we can use them in templates.
type pageContent struct {
	// Introduction is optional text to override the default introduction.
	Introduction string
	Section      sectionEntry
	// Fields are the top-level fields of the configuration section for this page.
	Fields map[refgen.PackageInfo]refgen.ReferenceEntry
}

// sectionEntry represents a top-level section of the configuration reference.
type sectionEntry struct {
	SectionExample string
	refgen.ReferenceEntry
}

// SectionConfig describes a section type to include in the reference.
type SectionConfig struct {
	// The name of the struct type as declared in the Go source, e.g.,
	// Jamf.
	TypeName string `yaml:"type"`
	// Introduction paragraph(s) to add to the template in place of the
	// default, which is the GoDoc for the section type.
	Introduction string `yaml:"introduction"`
}

// GeneratorConfig is the user-facing configuration for the configuration
// reference generator.
type GeneratorConfig struct {
	Sections []SectionConfig `yaml:"sections"`
	// Source is the Go source file path to parse for type declarations.
	Source string `yaml:"source"`
	// Directory where the generator writes reference pages.
	DestinationDirectory string `yaml:"destination"`
	// CamelCaseExceptions is a list of strings that should be exempt from
	// conversion to title case in section titles. For example, "SSH" or "OIDC".
	CamelCaseExceptions []string `yaml:"camel_case_exceptions"`
	// Directory where example YAML files are located.
	ExamplesDirectory string `yaml:"examples_directory"`
	// A list of mappings from known abbreviations to their replacements in section titles.
	// For example "App" to "Application".
	TitleWordReplacements []map[string]string `yaml:"title_word_replacements"`
}

// Generate uses the provided user-facing configuration to write the configuration
// reference to fs. Uses prefix, e.g., github.com/gravitational/teleport, to
// construct package paths.
func Generate(prefix string, conf GeneratorConfig, tmpl *template.Template) error {

	sourceData, err := section.NewSourceData(prefix, conf.Source)
	if err != nil {
		return fmt.Errorf("loading Go source files: %w", err)
	}

	if err := os.MkdirAll(conf.DestinationDirectory, 0755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	var errs refgen.GenerationError
	for _, r := range conf.Sections {
		relPath := section.ToModuleRelativePath(conf.Source)
		k := refgen.PackageInfo{
			DeclName: r.TypeName,
			PackagePath: path.Join(
				prefix,
				path.Dir(relPath),
			),
		}

		decl, ok := sourceData.TypeDecls[k]
		if !ok {
			errs.Messages = append(errs.Messages, fmt.Errorf("creating a configuration reference entry for declaration %v in %v: cannot find a declaration of this configuration section type", k.DeclName, k.PackagePath))
			continue
		}

		pc := pageContent{}
		pc.Introduction = r.Introduction

		// decl is a configuration type, so get data for the type and its dependencies.
		entries, err := section.ReferenceDataFromDeclaration(prefix, decl, sourceData.TypeDecls, conf.CamelCaseExceptions, conf.TitleWordReplacements)
		if errors.As(err, &refgen.NotAGenDeclError{}) {
			continue
		}
		if err != nil {
			errs.Messages = append(errs.Messages, fmt.Errorf("creating a reference entry for declaration %v in %v: %w", k.DeclName, k.PackagePath, err))
		}

		pc.Section.ReferenceEntry = entries[k]
		delete(entries, k)
		pc.Fields = entries

		sectionExampleLocation := filepath.Join(conf.ExamplesDirectory, pc.Section.ReferenceEntry.YAMLName+".yaml")
		if exampleBytes, err := os.ReadFile(sectionExampleLocation); err == nil {
			pc.Section.SectionExample = string(exampleBytes)
		}

		filename := strings.ReplaceAll(strings.ToLower(pc.Section.SectionName), " ", "-")
		docpath := filepath.Join(conf.DestinationDirectory, filename+".mdx")
		doc, err := os.Create(docpath)
		if err != nil {
			errs.Messages = append(errs.Messages, fmt.Errorf("cannot create page at %v: %w", docpath, err))
			continue
		}
		defer doc.Close()

		if err := tmpl.Execute(doc, pc); err != nil {
			errs.Messages = append(errs.Messages, fmt.Errorf("cannot populate the configuration reference template: %w", err))
		}
	}
	if len(errs.Messages) > 0 {
		return errs
	}

	return nil
}
