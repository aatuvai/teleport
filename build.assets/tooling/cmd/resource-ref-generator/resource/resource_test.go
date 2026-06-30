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
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gravitational/teleport/build.assets/tooling/lib/refgen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// replaceBackticks replaces the "BACKTICK" placeholder text with backticks so
// we can include struct tags within source fixtures.
func replaceBackticks(source string) string {
	return strings.ReplaceAll(source, "BACKTICK", "`")
}

func TestReferenceDataFromDeclaration(t *testing.T) {
	camelCaseExceptions := []string{
		"IdP",
	}

	cases := []struct {
		description string
		source      string
		expected    map[refgen.PackageInfo]refgen.ReferenceEntry
		// Go source fixtures that the test uses for named type fields.
		declSources []string
		// Substring to expect in a resulting error message
		errorSubstring string
		declInfo       refgen.PackageInfo
	}{
		{
			description: "scalar fields with one field ignored",
			declInfo: refgen.PackageInfo{
				DeclName:    "Metadata",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Namespace is the resource's namespace
    Namespace string BACKTICKprotobuf:"bytes,2,opt,name=Namespace,proto3" json:"-"BACKTICK
    // Description is the resource's description.
    Description string BACKTICKprotobuf:"bytes,3,opt,name=Description,proto3" json:"description,omitempty"BACKTICK
    // Age is the resource's age in seconds.
    Age uint BACKTICKjson:"age"BACKTICK
    // Active indicates whether the resource is currently in use.
    Active bool BACKTICKjson:"active"BACKTICK
}
`,
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Metadata",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `name: "string"
description: "string"
age: 1
active: true
`,
					Fields: []refgen.Field{
						{
							Name:        "active",
							Description: "Indicates whether the resource is currently in use.",
							Type:        "Boolean",
						},
						{
							Name:        "age",
							Description: "The resource's age in seconds.",
							Type:        "number",
						},
						{
							Name:        "description",
							Description: "The resource's description.",
							Type:        "string",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "sequences of scalars",
			declInfo: refgen.PackageInfo{
				DeclName:    "Metadata",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Names is a list of names.
    Names []string BACKTICKjson:"names"BACKTICK
    // Numbers is a list of numbers.
    Numbers []int BACKTICKjson:"numbers"BACKTICK
    // Booleans is a list of Booleans.
    Booleans []bool BACKTICKjson:"booleans"BACKTICK
}
`,
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Metadata",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `names: 
  - "string"
  - "string"
  - "string"
numbers: 
  - 1
  - 1
  - 1
booleans: 
  - true
  - true
  - true
`,
					Fields: []refgen.Field{
						{
							Name:        "booleans",
							Description: "A list of Booleans.",
							Type:        "[]Boolean",
						},
						{
							Name:        "names",
							Description: "A list of names.",
							Type:        "[]string",
						},
						{
							Name:        "numbers",
							Description: "A list of numbers.",
							Type:        "[]number",
						},
					},
				},
			},
		},
		{
			description: "a map of strings to sequences",
			declInfo: refgen.PackageInfo{
				DeclName:    "Metadata",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
  // Attributes indicates additional data for the resource.
  Attributes map[string][]string BACKTICKjson:"attributes"BACKTICK
}
`,
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Metadata",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `attributes: 
  "string": 
    - "string"
    - "string"
    - "string"
  "string": 
    - "string"
    - "string"
    - "string"
  "string": 
    - "string"
    - "string"
    - "string"
`,
					Fields: []refgen.Field{
						{
							Name:        "attributes",
							Description: "Indicates additional data for the resource.",
							Type:        "map[string][]string",
						},
					},
				},
			},
		},
		{
			description: "an undeclared custom type field",
			declInfo: refgen.PackageInfo{
				DeclName:    "Server",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

import "types"

// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Spec contains information about the server.
    Spec types.ServerSpecV1 BACKTICKjson:"spec"BACKTICK
}
`,
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Server",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "src/myfile.go",
					Fields: []refgen.Field{
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "",
						},
					},
					YAMLExample: "name: \"string\"\nspec: # See description\n",
				},
			},
		},
		{
			description: "named scalar type",
			declInfo: refgen.PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "Server",
			},
			source: `package mypkg

import types "github.com/gravitational/teleport/src"

// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Spec contains information about the server.
    Spec types.ServerSpecV1 BACKTICKjson:"spec"BACKTICK
    // Label specifies labels for the server.
    Label Labels BACKTICKjson:"labels"BACKTICK
}
`,
			declSources: []string{
				`package mypkg

// Labels is a slice of strings that we'll process downstream
type Labels []string
`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Server",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "src/myfile.go",
					Fields: []refgen.Field{
						{
							Name:        "labels",
							Description: "Specifies labels for the server.",
							Type:        "[Labels](#labels)",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "",
						},
					},
					YAMLExample: "name: \"string\"\nspec: # See description\nlabels: # [...]\n",
				},
				refgen.PackageInfo{
					DeclName:    "Labels",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Labels",
					Description: "A slice of strings that we'll process downstream",
					SourcePath:  "src/myfile0.go",
					Fields:      nil,
					YAMLExample: "",
				},
			},
		},
		{
			description: "custom type fields with a custom JSON unmarshaller",
			declInfo: refgen.PackageInfo{
				DeclName:    "Server",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

import types "github.com/gravitational/teleport/src"

// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Spec contains information about the server.
    Spec types.ServerSpecV1 BACKTICKjson:"spec"BACKTICK
}
`,
			declSources: []string{
				`package mypkg

func (s *Server) UnmarshalJSON (b []byte) error {
  return nil
}
`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Server",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "src/myfile.go",
					Fields: []refgen.Field{
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "",
						},
					},
					YAMLExample: "name: \"string\"\nspec: # See description\n",
				},
			},
		},
		{
			description: "custom type with custom YAML unmarshaller",
			declInfo: refgen.PackageInfo{
				DeclName:    "Application",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

import types "github.com/gravitational/teleport/src"

// Application includes information about an application registered with Teleport.
type Application struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Spec contains information about the application.
    Spec types.AppSpecV1 BACKTICKjson:"spec"BACKTICK
}
`,
			declSources: []string{
				`package mypkg

func (a *Application) UnmarshalYAML(value *yaml.Node) error {
  return nil
}
`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Application",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Application",
					Description: "Includes information about an application registered with Teleport.",
					SourcePath:  "src/myfile.go",
					Fields: []refgen.Field{
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						{
							Name:        "spec",
							Description: "Contains information about the application.",
							Type:        "",
						},
					},
					YAMLExample: "name: \"string\"\nspec: # See description\n",
				},
			},
		},
		{
			description: "a custom type field declared in a second source file",
			declInfo: refgen.PackageInfo{
				DeclName:    "Server",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

import types "github.com/gravitational/teleport/src"

// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Spec contains information about the server.
    Spec types.ServerSpecV1 BACKTICKjson:"spec"BACKTICK
}
`,
			declSources: []string{`package types
// ServerSpecV1 includes aspects of a proxied server.
type ServerSpecV1 struct {
    // The address of the server.
    Address string BACKTICKjson:"address"BACKTICK
    // How long the resource is valid.
    TTL int BACKTICKjson:"ttl"BACKTICK
    // Whether the server is active.
    IsActive bool BACKTICKjson:"is_active"BACKTICK
}`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Server",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `name: "string"
spec: # [...]
`,
					Fields: []refgen.Field{
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "[Server Spec V1](#server-spec-v1)",
						},
					},
				},
				refgen.PackageInfo{
					DeclName:    "ServerSpecV1",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Server Spec V1",
					Description: "Includes aspects of a proxied server.",
					SourcePath:  "src/myfile0.go",
					YAMLExample: `address: "string"
ttl: 1
is_active: true
`,
					Fields: []refgen.Field{
						{
							Name:        "address",
							Description: "The address of the server.",
							Type:        "string",
						},
						{
							Name:        "is_active",
							Description: "Whether the server is active.",
							Type:        "Boolean",
						},
						{
							Name:        "ttl",
							Description: "How long the resource is valid.",
							Type:        "number",
						},
					},
				},
			},
		},
		{
			description: "composite field type with named scalar type",
			declInfo: refgen.PackageInfo{
				DeclName:    "Server",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

import types "github.com/gravitational/teleport/src"

// Server includes information about a server registered with Teleport.
type Server struct {
    // Spec contains information about the server.
    Spec types.ServerSpecV1 BACKTICKjson:"spec"BACKTICK
    // LabelMaps includes a map of strings to labels.
    LabelMaps []map[string]types.Label BACKTICKjson:"label_maps"BACKTICK
}
`,
			declSources: []string{`package types
// ServerSpecV1 includes aspects of a proxied server.
type ServerSpecV1 struct {
    // The address of the server.
    Address string BACKTICKjson:"address"BACKTICK
}`,
				`package types

// Label is a custom type that we unmarshal in a non-default way.
type Label string
`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Server",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `spec: # [...]
label_maps: 
  - 
    "string": # [...]
    "string": # [...]
    "string": # [...]
  - 
    "string": # [...]
    "string": # [...]
    "string": # [...]
  - 
    "string": # [...]
    "string": # [...]
    "string": # [...]
`,
					Fields: []refgen.Field{
						{
							Name:        "label_maps",
							Description: "Includes a map of strings to labels.",
							Type:        "[]map[string][Label](#label)",
						},
						{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "[Server Spec V1](#server-spec-v1)",
						},
					},
				},
				refgen.PackageInfo{
					DeclName:    "ServerSpecV1",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Server Spec V1",
					Description: "Includes aspects of a proxied server.",
					SourcePath:  "src/myfile0.go",
					YAMLExample: `address: "string"
`,
					Fields: []refgen.Field{
						{
							Name:        "address",
							Description: "The address of the server.",
							Type:        "string",
						},
					},
				},
				refgen.PackageInfo{
					DeclName:    "Label",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Label",
					Description: "A custom type that we unmarshal in a non-default way.",
					SourcePath:  "src/myfile1.go",
					Fields:      nil,
				},
			},
		},
		{
			description: "struct type with an interface field",
			declInfo: refgen.PackageInfo{
				DeclName:    "Server",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

// Server includes information about a server registered with Teleport.
type Server struct {
  // The name of the server.
  Name string BACKTICK:json:"name"BACKTICK
  // Impl is the implementation of the server.
  Impl ServerImplementation BACKTICK:json:"impl"BACKTICK
}
`,
			declSources: []string{`package mypkg
// ServerImplementation is a remote service with a URL.
type ServerImplementation interface{
  GetURL() string
}
`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Server",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "src/myfile.go",
					Fields: []refgen.Field{
						{
							Name:        "impl",
							Description: "The implementation of the server.",
							Type:        "[Server Implementation](#server-implementation)",
						},
						{
							Name:        "name",
							Description: "The name of the server.",
							Type:        "string",
						},
					},
					YAMLExample: `name: "string"
impl: # [...]
`,
				},
				refgen.PackageInfo{
					DeclName:    "ServerImplementation",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Server Implementation",
					Description: "A remote service with a URL.",
					SourcePath:  "src/myfile0.go",
					Fields:      nil,
					YAMLExample: "",
				},
			},
		},
		{
			description: "embedded struct",
			declInfo: refgen.PackageInfo{
				DeclName:    "MyResource",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package mypkg

import types "github.com/gravitational/teleport/src"

// MyResource is a resource declared for testing.
type MyResource struct{
  // Alias is another name to call the resource.
  Alias string BACKTICKjson:"alias"BACKTICK
  types.Metadata
}
`,
			declSources: []string{
				`package types

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Active indicates whether the resource is currently in use.
    Active bool BACKTICKjson:"active"BACKTICK
}`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "MyResource",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "My Resource",
					Description: "A resource declared for testing.",
					SourcePath:  "src/myfile.go",
					Fields: []refgen.Field{
						{
							Name:        "active",
							Description: "Indicates whether the resource is currently in use.",
							Type:        "Boolean",
						},
						{
							Name:        "alias",
							Description: "Another name to call the resource.",
							Type:        "string",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
					YAMLExample: `alias: "string"
name: "string"
active: true
`,
				},
			},
		},
		{
			description: "embedded struct with struct field",
			declInfo: refgen.PackageInfo{
				DeclName:    "MyResource",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package mypkg

import types "github.com/gravitational/teleport/src"

// MyResource is a resource declared for testing.
type MyResource struct{
  // Alias is another name to call the resource.
  Alias string BACKTICKjson:"alias"BACKTICK
  types.Header
}
`,
			declSources: []string{
				`package types
type Header struct {
    // Metadata is the resource metadata
    Metadata Metadata BACKTICKjson:"metadata"BACKTICK
}

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Active indicates whether the resource is currently in use.
    Active bool BACKTICKjson:"active"BACKTICK
}`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "MyResource",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "My Resource",
					Description: "A resource declared for testing.",
					SourcePath:  "src/myfile.go",
					Fields: []refgen.Field{
						{
							Name:        "alias",
							Description: "Another name to call the resource.",
							Type:        "string",
						},
						{
							Name:        "metadata",
							Description: "The resource metadata",
							Type:        "[Metadata](#metadata)",
						},
					},
					YAMLExample: `alias: "string"
metadata: # [...]
`,
				},
				refgen.PackageInfo{
					DeclName:    "Metadata",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Metadata",
					SourcePath:  "src/myfile0.go",
					Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
					Fields: []refgen.Field{
						{
							Name:        "active",
							Description: "Indicates whether the resource is currently in use.",
							Type:        "Boolean",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
					YAMLExample: `name: "string"
active: true
`,
				},
			},
		},
		{
			description: "embedded struct with base in the same package",
			declInfo: refgen.PackageInfo{
				DeclName:    "MyResource",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package mypkg
// MyResource is a resource declared for testing.
type MyResource struct{
  // Alias is another name to call the resource.
  Alias string BACKTICKjson:"alias"BACKTICK
  Metadata
}
`,
			declSources: []string{
				`package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Active indicates whether the resource is currently in use.
    Active bool BACKTICKjson:"active"BACKTICK
}`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "MyResource",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "My Resource",
					Description: "A resource declared for testing.",
					SourcePath:  "src/myfile.go",
					Fields: []refgen.Field{
						{
							Name:        "active",
							Description: "Indicates whether the resource is currently in use.",
							Type:        "Boolean",
						},
						{
							Name:        "alias",
							Description: "Another name to call the resource.",
							Type:        "string",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
					YAMLExample: `alias: "string"
name: "string"
active: true
`,
				},
			},
		},
		{
			description: "struct with two embedded structs",
			declInfo: refgen.PackageInfo{
				DeclName:    "MyResource",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package mypkg

import moretypes "github.com/gravitational/teleport/src"
import types "github.com/gravitational/teleport/src"

// MyResource is a resource declared for testing.
type MyResource struct{
  // Alias is another name to call the resource.
  Alias string BACKTICKjson:"alias"BACKTICK
  types.Metadata
  moretypes.ActivityStatus
}
`,
			declSources: []string{
				`package types

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
}`,
				`package moretypes

// ActivityStatus indicates the status of a resource
type ActivityStatus struct{
    // Active indicates whether the resource is currently in use.
    Active bool BACKTICKjson:"active"BACKTICK
}`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "MyResource",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "My Resource",
					Description: "A resource declared for testing.",
					SourcePath:  "src/myfile.go",
					Fields: []refgen.Field{
						{
							Name:        "active",
							Description: "Indicates whether the resource is currently in use.",
							Type:        "Boolean",
						},
						{
							Name:        "alias",
							Description: "Another name to call the resource.",
							Type:        "string",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
					YAMLExample: `alias: "string"
name: "string"
active: true
`,
				},
			},
		},
		{
			description: "embedded struct with an embedded struct",
			declInfo: refgen.PackageInfo{
				DeclName:    "MyResource",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package mypkg

import types "github.com/gravitational/teleport/src"

// MyResource is a resource declared for testing.
type MyResource struct{
  // Alias is another name to call the resource.
  Alias string BACKTICKjson:"alias"BACKTICK
  types.Metadata
}
`,
			declSources: []string{
				`package types

import moretypes "github.com/gravitational/teleport/src"

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    moretypes.ActivityStatus
}`,
				`package moretypes

// ActivityStatus indicates the status of a resource
type ActivityStatus struct{
    // Active indicates whether the resource is currently in use.
    Active bool BACKTICKjson:"active"BACKTICK
}`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "MyResource",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "My Resource",
					Description: "A resource declared for testing.",
					SourcePath:  "src/myfile.go",
					Fields: []refgen.Field{
						{
							Name:        "active",
							Description: "Indicates whether the resource is currently in use.",
							Type:        "Boolean",
						},
						{
							Name:        "alias",
							Description: "Another name to call the resource.",
							Type:        "string",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
					YAMLExample: `alias: "string"
name: "string"
active: true
`,
				},
			},
		},
		{
			description: "ignored fields with non-YAML-comptabible types",
			declInfo: refgen.PackageInfo{
				DeclName:    "Metadata",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    XXX_NoUnkeyedLiteral struct{} BACKTICKjson:"-"BACKTICK
    XXX_unrecognized     []byte   BACKTICKjson:"-"BACKTICK

}
`,
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Metadata",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `name: "string"
`,
					Fields: []refgen.Field{
						refgen.Field{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "non-embedded custom field type declared in the same package as the containing struct",
			declInfo: refgen.PackageInfo{
				DeclName:    "DatabaseServerV3",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package typestest

// DatabaseServerV3 represents a database access server.
type DatabaseServerV3 struct {
	// Kind is the database server resource kind.
	Kind string BACKTICKprotobuf:"bytes,1,opt,name=Kind,proto3" json:"kind"BACKTICK
	// Metadata is the database server metadata.
	Metadata Metadata BACKTICKprotobuf:"bytes,4,opt,name=Metadata,proto3" json:"metadata"BACKTICK
}
`,
			declSources: []string{
				`package typestest

// Metadata is resource metadata
type Metadata struct {
	// Name is an object name
	Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
	// Description is object description
	Description string BACKTICKprotobuf:"bytes,3,opt,name=Description,proto3" json:"description,omitempty"BACKTICK
}`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "DatabaseServerV3",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Database Server V3",
					Description: "Represents a database access server.",
					SourcePath:  "src/myfile.go",
					Fields: []refgen.Field{
						refgen.Field{
							Name:        "kind",
							Description: "The database server resource kind.",
							Type:        "string",
						},
						refgen.Field{
							Name:        "metadata",
							Description: "The database server metadata.",
							Type:        "[Metadata](#metadata)",
						},
					},
					YAMLExample: `kind: "string"
metadata: # [...]
`,
				},
				refgen.PackageInfo{
					DeclName:    "Metadata",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Metadata",
					Description: "Resource metadata",
					SourcePath:  "src/myfile0.go",
					Fields: []refgen.Field{
						{
							Name:        "description",
							Description: "Object description",
							Type:        "string",
						},
						{
							Name:        "name",
							Description: "An object name",
							Type:        "string",
						},
					},
					YAMLExample: `name: "string"
description: "string"
`,
				},
			},
		},
		{
			description: "pointer field",
			declInfo: refgen.PackageInfo{
				DeclName:    "DatabaseServerV3",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package typestest

// DatabaseServerV3 represents a database access server.
type DatabaseServerV3 struct {
	// Metadata is the database server metadata.
	Metadata *Metadata BACKTICKprotobuf:"bytes,4,opt,name=Metadata,proto3" json:"metadata"BACKTICK
}
`,
			declSources: []string{
				`package typestest

// Metadata is resource metadata
type Metadata struct {
	// Name is an object name
	Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
}`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "DatabaseServerV3",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Database Server V3",
					Description: "Represents a database access server.",
					SourcePath:  "src/myfile.go",
					Fields: []refgen.Field{
						refgen.Field{
							Name:        "metadata",
							Description: "The database server metadata.",
							Type:        "[Metadata](#metadata)",
						},
					},
					YAMLExample: `metadata: # [...]
`,
				},
				refgen.PackageInfo{
					DeclName:    "Metadata",
					PackagePath: "github.com/gravitational/teleport/src",
				}: refgen.ReferenceEntry{
					SectionName: "Metadata",
					Description: "Resource metadata",
					SourcePath:  "src/myfile0.go",
					Fields: []refgen.Field{
						{
							Name:        "name",
							Description: "An object name",
							Type:        "string",
						},
					},
					YAMLExample: `name: "string"
`,
				},
			},
		},
		{
			description: "map of strings to an undeclared field",
			declInfo: refgen.PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "Server",
			},
			source: `
package mypkg

// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the server.
    Name string BACKTICKjson:"name"BACKTICK
    // LabelMaps includes a map of strings to labels.
    LabelMaps []map[string]types.Label BACKTICKjson:"label_maps"BACKTICK
}
`,
			declSources: []string{`package types
// ServerSpecV1 includes aspects of a proxied server.
type ServerSpecV1 struct {
    // The address of the server.
    Address string BACKTICKjson:"address"BACKTICK
}`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Server",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `name: "string"
label_maps: 
  - 
    "string": # See description
    "string": # See description
    "string": # See description
  - 
    "string": # See description
    "string": # See description
    "string": # See description
  - 
    "string": # See description
    "string": # See description
    "string": # See description
`,
					Fields: []refgen.Field{
						refgen.Field{
							Name:        "label_maps",
							Description: "Includes a map of strings to labels.",
							Type:        "[]map[string]",
						},
						refgen.Field{
							Name:        "name",
							Description: "The name of the server.",
							Type:        "string"},
					},
				},
			},
		},
		{
			description: "type parameter",
			declInfo: refgen.PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "Resource",
			},
			source: `package mypkg
// Resource is a resource.
type Resource struct {
  // The name of the resource.
  Name string BACKTICKjson:"name"BACKTICK
}
`,
			declSources: []string{
				`package mypkg
// streamFunc is a wrapper that converts a closure into a stream.
type streamFunc[T any] struct {
	fn        func() (T, error)
	doneFuncs []func()
	item      T
	err       error
}

func (stream *streamFunc[T]) Next() bool {
	stream.item, stream.err = stream.fn()
	return stream.err == nil
}
`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					PackagePath: "github.com/gravitational/teleport/src",
					DeclName:    "Resource",
				}: refgen.ReferenceEntry{
					SectionName: "Resource",
					Description: "A resource.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `name: "string"
`,
					Fields: []refgen.Field{
						refgen.Field{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "field type not declared in a loaded package",
			declInfo: refgen.PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "Resource",
			},
			source: `package mypkg

import "time"

// Resource is a resource.
type Resource struct {
  // The name of the resource.
  Name string BACKTICKjson:"name"BACKTICK
  // How much time must elapse before the resource expires.
  Expiry time.Time BACKTICKjson:"expiry"BACKTICK
}
`,
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					PackagePath: "github.com/gravitational/teleport/src",
					DeclName:    "Resource",
				}: refgen.ReferenceEntry{
					SectionName: "Resource",
					Description: "A resource.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `name: "string"
expiry: # See description
`,
					Fields: []refgen.Field{
						refgen.Field{
							Name:        "expiry",
							Description: "How much time must elapse before the resource expires.",
							Type:        "",
						},
						refgen.Field{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
				},
			},
			declSources: []string{},
		},
		{
			description: "byte slice",
			declInfo: refgen.PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "Metadata",
			},
			source: `
package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // PrivateKey is the private key of the resource.
    PrivateKey []byte BACKTICKjson:"private_key"BACKTICK
}
`,
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Metadata",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `name: "string"
private_key: BASE64_STRING
`,
					Fields: []refgen.Field{
						refgen.Field{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						refgen.Field{
							Name:        "private_key",
							Description: "The private key of the resource.",
							Type:        "base64-encoded string",
						},
					},
				},
			},
		},
		{
			description: "named import in embedded struct field",
			declInfo: refgen.PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "Server",
			},
			source: `
package mypkg

import types "github.com/gravitational/teleport/src"

// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Spec contains information about the server.
    Spec types.ServerSpecV1 BACKTICKjson:"spec"BACKTICK
}
`,
			declSources: []string{`package types

import alias "github.com/gravitational/teleport/src"

// ServerSpecV1 includes aspects of a proxied server.
type ServerSpecV1 struct {
  alias.ServerSpec
}`,
				`package otherpkg

type ServerSpec struct {
    // The address of the server.
    Address string BACKTICKjson:"address"BACKTICK
}`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Server",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `name: "string"
spec: # [...]
`,
					Fields: []refgen.Field{
						refgen.Field{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						refgen.Field{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "[Server Spec V1](#server-spec-v1)",
						},
					},
				},
				refgen.PackageInfo{
					DeclName:    "ServerSpecV1",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Server Spec V1",
					Description: "Includes aspects of a proxied server.",
					SourcePath:  "src/myfile0.go",
					YAMLExample: `address: "string"
`,
					Fields: []refgen.Field{
						refgen.Field{
							Name:        "address",
							Description: "The address of the server.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "named import in named struct field",
			declInfo: refgen.PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "Server",
			},
			source: `
package mypkg

import types "github.com/gravitational/teleport/src"

// Server includes information about a server registered with Teleport.
type Server struct {
    // Spec contains information about the server.
    Spec types.ServerSpecV1 BACKTICKjson:"spec"BACKTICK
}
`,
			declSources: []string{`package types
import alias "github.com/gravitational/teleport/src"

// ServerSpecV1 includes aspects of a proxied server.
type ServerSpecV1 struct {
  // Address information.
  Info alias.AddressInfo BACKTICKjson:"info"BACKTICK
}`,

				`package otherpkg
// AddressInfo provides information about an address.
type AddressInfo struct {
    // The address of the server.
    Address string BACKTICKjson:"address"BACKTICK
}`,
			},
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "AddressInfo",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Address Info",
					Description: "Provides information about an address.",
					SourcePath:  "src/myfile1.go",
					Fields: []refgen.Field{
						{
							Name:        "address",
							Description: "The address of the server.",
							Type:        "string",
						},
					},
					YAMLExample: "address: \"string\"\n",
				},
				refgen.PackageInfo{
					DeclName:    "Server",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `spec: # [...]
`,
					Fields: []refgen.Field{
						refgen.Field{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "[Server Spec V1](#server-spec-v1)"},
					},
				},
				refgen.PackageInfo{
					DeclName:    "ServerSpecV1",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Server Spec V1",
					Description: "Includes aspects of a proxied server.",
					SourcePath:  "src/myfile0.go",
					YAMLExample: `info: # [...]
`,
					Fields: []refgen.Field{
						refgen.Field{
							Name:        "info",
							Description: "Address information.",
							Type:        "[Address Info](#address-info)",
						},
					},
				},
			},
		},
		{
			description: "scalar fields with two unexported fields",
			declInfo: refgen.PackageInfo{
				DeclName:    "Metadata",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

import "protoimpl"

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Description is the resource's description.
    Description string BACKTICKprotobuf:"bytes,3,opt,name=Description,proto3" json:"description,omitempty"BACKTICK
    state protoimpl.MessageState BACKTICKprotogen:"open.v1"BACKTICK
    unknownFields protoimpl.UnknownFields
    sizeCache     protoimpl.SizeCache
}
`,
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Metadata",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `name: "string"
description: "string"
`,
					Fields: []refgen.Field{
						refgen.Field{
							Name:        "description",
							Description: "The resource's description.",
							Type:        "string",
						},
						refgen.Field{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "curly braces in descriptions",
			declInfo: refgen.PackageInfo{
				DeclName:    "Metadata",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

// Metadata describes information about a {dynamic resource}. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the {name of the resource}.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
}
`,
			expected: map[refgen.PackageInfo]refgen.ReferenceEntry{
				refgen.PackageInfo{
					DeclName:    "Metadata",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a `{dynamic resource}`. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "src/myfile.go",
					YAMLExample: `name: "string"
`,
					Fields: []refgen.Field{
						refgen.Field{
							Name:        "name",
							Description: "The `{name of the resource}`.",
							Type:        "string",
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			tmp := t.TempDir()
			if err := os.Mkdir(filepath.Join(tmp, "src"), 0777); err != nil {
				t.Fatal(err)
			}

			// Make a map of filenames to content
			sources := make(map[string][]byte)
			sources[filepath.Join(tmp, "src", "myfile.go")] = []byte(replaceBackticks(tc.source))
			for i, s := range tc.declSources {
				sources[filepath.Join(tmp, "src", "myfile"+strconv.Itoa(i)+".go")] = []byte(replaceBackticks(s))
			}

			// Write the source content to a temporary directory
			for n, v := range sources {
				f, err := os.Create(n)
				if err != nil {
					t.Fatal(err)
				}

				if _, err := f.Write(v); err != nil {
					t.Fatal(err)
				}
			}

			sourceData, err := NewSourceData("github.com/gravitational/teleport", tmp)
			if err != nil {
				t.Fatal(err)
			}

			// Remove the temporary directory from package paths
			// since we can't know it in advance in test cases.
			declsWithoutTmp := make(map[refgen.PackageInfo]refgen.DeclarationInfo)
			for k, d := range sourceData.TypeDecls {
				k.PackagePath = strings.ReplaceAll(k.PackagePath, filepath.Base(tmp)+"/", "")
				d.PackageName = strings.ReplaceAll(k.PackagePath, filepath.Base(tmp)+"/", "")
				declsWithoutTmp[k] = d
			}

			di, ok := declsWithoutTmp[tc.declInfo]
			if !ok {
				t.Fatalf("expected data for %v.%v not found in the source", tc.declInfo.PackagePath, tc.declInfo.DeclName)
			}

			r, err := ReferenceDataFromDeclaration("github.com/gravitational/teleport", di, declsWithoutTmp, camelCaseExceptions)
			if tc.errorSubstring == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.errorSubstring)
			}

			assert.Equal(t, tc.expected, r)
		})
	}
}

func TestNamedImports(t *testing.T) {
	cases := []struct {
		description string
		input       string
		expected    map[string]string
	}{
		{
			description: "single-line format",
			input: `package mypkg
import alias "otherpkg"
`,
			expected: map[string]string{
				"alias": "otherpkg",
			},
		},
		{
			description: "multi-line format",
			input: `package mypkg
import (
    alias "first"
    alias2 "second"
)
`,
			expected: map[string]string{
				"alias":  "first",
				"alias2": "second",
			},
		},
		{
			description: "multi-segment package path",
			input: `package mypkg
import alias "my/multi/segment/package"
`,
			expected: map[string]string{
				"alias": "my/multi/segment/package",
			},
		},
		{
			description: "no explicit name",
			input: `package mypkg
import "my/multi/segment/pkg"
`,
			expected: map[string]string{
				"pkg": "my/multi/segment/pkg",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset,
				"myfile.go",
				c.input,
				parser.ParseComments,
			)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, refgen.NamedImports(f))
		})
	}
}

func TestMakeFieldTableInfo(t *testing.T) {
	camelCaseExceptions := []string{
		"IdP",
	}

	cases := []struct {
		description string
		input       []refgen.RawField
		expected    []refgen.Field
	}{
		{
			description: "angle brackets in GoDoc",
			input: []refgen.RawField{
				refgen.RawField{
					PackageName: "mypkg",
					Doc:         `An ID, e.g., "<myid>"`,
					Kind:        refgen.YAMLString{},
					Name:        "ObjectID",
					FieldName:   "object_id",
					Tags:        `json:"object_id"`,
				},
			},
			expected: []refgen.Field{
				{
					Name:        "object_id",
					Description: `An ID, e.g., "\<myid\>"`,
					Type:        "string",
				},
			},
		},
		{
			description: "pipe in field description",
			input: []refgen.RawField{
				{
					PackageName: "mypkg",
					Doc:         "Specifies the locking mode (strict|best_effort) to be applied with the role.",
					Kind: refgen.YAMLCustomType{
						Name: "LockingMode",
						DeclarationInfo: refgen.PackageInfo{
							DeclName:    "LockingMode",
							PackagePath: "mypkg",
						},
					},
					Name:      "LockingMode",
					FieldName: "locking_mode",
					Tags:      "json:\"locking_mode\"",
				},
			},
			expected: []refgen.Field{
				{
					Name:        "locking_mode",
					Description: `Specifies the locking mode (strict\|best_effort) to be applied with the role.`,
					Type:        "[Locking Mode](#locking-mode)",
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			f, err := refgen.MakeFieldTableInfo(c.input, camelCaseExceptions)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, f)
		})
	}
}

func TestGetJSONTag(t *testing.T) {
	cases := []struct {
		description string
		input       string
		expected    string
	}{
		{
			description: "one well-formed struct tag",
			input:       `json:"my_tag"`,
			expected:    "my_tag",
		},
		{
			description: "multiple well-formed struct tags",
			input:       `json:"json_tag" yaml:"yaml_tag" other:"other-tag"`,
			expected:    "json_tag",
		},
		{
			description: "omitempty option in tag value",
			input:       `json:"json_tag,omitempty" yaml:"yaml_tag" other:"other-tag"`,
			expected:    "json_tag",
		},
		{
			description: "No JSON tag",
			input:       `other:"other-tag"`,
			expected:    "",
		},
		{
			description: "Empty JSON tag with the omitempty option",
			input:       `json:",omitempty" other:"other-tag"`,
			expected:    "",
		},
		{
			description: "Ignored JSON field",
			input:       `json:"-" other:"other-tag"`,
			expected:    "-",
		},
		{
			description: "empty JSON tag",
			input:       `json:"" yaml:"yaml_tag" other:"other-tag"`,
			expected:    "",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			g := refgen.GetTag(c.input, "json")
			assert.Equal(t, c.expected, g)
		})
	}
}

// TestAllFieldsDeclEmbeddedPointerCrossPackageSameName tests the pattern where a
// wrapper struct embeds a pointer to a same-named type from a different package
// (e.g. api/types/discoveryconfig.IntegrationDiscoveredSummary embedding
// *discoveryconfigv1.IntegrationDiscoveredSummary).
func TestAllFieldsDeclEmbeddedPointerCrossPackageSameName(t *testing.T) {
	fset := token.NewFileSet()

	protoPkg := "github.com/gravitational/teleport/proto"
	protoFile, err := parser.ParseFile(fset, "proto.go", `package proto

type Foo struct {
	Name string
}
`, parser.ParseComments)
	require.NoError(t, err)

	wrapperPkg := "github.com/gravitational/teleport/wrapper"
	wrapperFile, err := parser.ParseFile(fset, "wrapper.go", `package wrapper

import proto "github.com/gravitational/teleport/proto"

type Foo struct {
	*proto.Foo
}
`, parser.ParseComments)
	require.NoError(t, err)

	allDecls := map[refgen.PackageInfo]refgen.DeclarationInfo{
		{DeclName: "Foo", PackagePath: protoPkg}: {
			Decl:        protoFile.Decls[0],
			FilePath:    "proto/proto.go",
			PackageName: protoPkg,
		},
		{DeclName: "Foo", PackagePath: wrapperPkg}: {
			Decl:         wrapperFile.Decls[1], // skip import decl
			FilePath:     "wrapper/wrapper.go",
			PackageName:  wrapperPkg,
			NamedImports: refgen.NamedImports(wrapperFile),
		},
	}

	wrapperDecl := allDecls[refgen.PackageInfo{DeclName: "Foo", PackagePath: wrapperPkg}]
	rs, err := refgen.TypeForDecl(wrapperDecl, allDecls, "json")
	require.NoError(t, err)

	_, err = refgen.AllFieldsForDecl(wrapperDecl, rs.Fields, allDecls, "json")
	require.NoError(t, err)
}

func TestPrintableDescription(t *testing.T) {
	cases := []struct {
		description string
		input       string
		name        string
		expected    string
	}{
		{
			description: "short description",
			input:       "A",
			name:        "MyDecl",
			expected:    "A",
		},
		{
			description: "no description",
			input:       "",
			name:        "MyDecl",
			expected:    "",
		},
		{
			description: "GoDoc consists only of declaration name",
			input:       "MyDecl",
			name:        "MyDecl",
			expected:    "",
		},
		{
			description: "description containing name",
			input:       "MyDecl is a declaration that we will describe in the docs.",
			name:        "MyDecl",
			expected:    "A declaration that we will describe in the docs.",
		},
		{
			description: "description containing name and \"are\"",
			input:       "MyDecls are things that we will describe in the docs.",
			name:        "MyDecls",
			expected:    "Things that we will describe in the docs.",
		},

		{
			description: "description with no name",
			input:       "Declaration that we will describe in the docs.",
			name:        "MyDecl",
			expected:    "Declaration that we will describe in the docs.",
		},
		{
			description: "description beginning with name and non-is verb",
			input:       "MyDecl performs an action.",
			name:        "MyDecl",
			expected:    "Performs an action.",
		},
		{
			description: "curly brace pair and identifier name",
			input:       "MyDecl performs an action, such as {updating, deleting}",
			name:        "MyDecl",
			expected:    "Performs an action, such as `{updating, deleting}`",
		},
		{
			description: "curly brace pair and no identifier name",
			input:       "Performs an action, such as {updating, deleting}",
			name:        "MyDecl",
			expected:    "Performs an action, such as `{updating, deleting}`",
		},
		{
			description: "curly brace pair with existing backticks",
			input:       "Performs an action, such as `{updating, deleting}`",
			name:        "MyDecl",
			expected:    "Performs an action, such as `{updating, deleting}`",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.Equal(t, c.expected, refgen.PrintableDescription(c.input, c.name, ""))
		})
	}
}

func TestMakeYAMLExample(t *testing.T) {
	cases := []struct {
		description string
		input       []refgen.RawField
		expected    string
	}{
		{
			description: "all scalars",
			input: []refgen.RawField{
				refgen.RawField{
					Doc:  "myInt is an int",
					Kind: refgen.YAMLNumber{},
					Name: "myInt",
					Tags: `json:"my_int"`,
				},
				refgen.RawField{
					Doc:  "myBool is a Boolean",
					Kind: refgen.YAMLBool{},
					Name: "myBool",
					Tags: `json:"my_bool"`,
				},
				refgen.RawField{
					Doc:  "myString is a string",
					Kind: refgen.YAMLString{},
					Tags: `json:"my_string"`,
				},
			},
			expected: `my_int: 1
my_bool: true
my_string: "string"
`,
		},
		{
			description: "sequence of sequence of strings",
			input: []refgen.RawField{
				refgen.RawField{
					Name:      "mySeq",
					FieldName: "my_seq",
					Doc:       "mySeq is a sequence of sequences of strings",
					Tags:      `json:"my_seq"`,
					Kind: refgen.YAMLSequence{
						ElementKind: refgen.YAMLSequence{
							ElementKind: refgen.YAMLString{},
						},
					},
				},
			},
			expected: `my_seq: 
  - 
    - "string"
    - "string"
    - "string"
  - 
    - "string"
    - "string"
    - "string"
  - 
    - "string"
    - "string"
    - "string"
`,
		},
		{
			description: "maps of numbers to strings",
			input: []refgen.RawField{
				refgen.RawField{
					Name:      "myMap",
					FieldName: "my_map",
					Doc:       "myMap is a map of ints to strings",
					Tags:      `json:"my_map"`,
					Kind: refgen.YAMLMapping{
						KeyKind:   refgen.YAMLNumber{},
						ValueKind: refgen.YAMLString{},
					},
				},
			},
			expected: `my_map: 
  1: "string"
  1: "string"
  1: "string"
`,
		},
		{
			description: "sequence of maps of strings to Booleans",
			input: []refgen.RawField{
				refgen.RawField{
					Name:      "mySeq",
					FieldName: "my_seq",
					Doc:       "mySeq is a complex type",
					Tags:      `json:"my_seq"`,
					Kind: refgen.YAMLSequence{
						ElementKind: refgen.YAMLMapping{
							KeyKind:   refgen.YAMLString{},
							ValueKind: refgen.YAMLBool{},
						},
					},
				},
			},
			expected: `my_seq: 
  - 
    "string": true
    "string": true
    "string": true
  - 
    "string": true
    "string": true
    "string": true
  - 
    "string": true
    "string": true
    "string": true
`,
		},
		{
			description: "sequences of custom types",
			input: []refgen.RawField{
				refgen.RawField{
					Name:      "labels",
					FieldName: "labels",
					Doc:       "labels is a list of labels",
					Tags:      `json:"labels"`,
					Kind: refgen.YAMLSequence{
						ElementKind: refgen.YAMLCustomType{
							Name: "label",
							DeclarationInfo: refgen.PackageInfo{
								DeclName:    "label",
								PackagePath: "mypkg",
							},
						},
					},
				},
			},
			expected: `labels: 
  - # [...]
  - # [...]
  - # [...]
`,
		},
		{
			description: "maps of strings to custom types",
			input: []refgen.RawField{
				refgen.RawField{
					Name:      "labels",
					FieldName: "labels",
					Doc:       "labels is a map of strings to labels",
					Tags:      `json:"labels"`,
					Kind: refgen.YAMLMapping{
						KeyKind: refgen.YAMLString{},
						ValueKind: refgen.YAMLCustomType{
							Name: "label",
							DeclarationInfo: refgen.PackageInfo{
								DeclName:    "label",
								PackagePath: "mypkg",
							},
						},
					},
				},
			},
			expected: `labels: 
  "string": # [...]
  "string": # [...]
  "string": # [...]
`,
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			e, err := refgen.MakeYAMLExample(c.input, "json")
			assert.NoError(t, err)
			assert.Equal(t, c.expected, e)
		})
	}
}

func TestSplitCamelCase(t *testing.T) {
	camelCaseExceptions := []string{
		"ElastiCache",
		"IdP",
		"MySQL",
		"SAML",
	}

	cases := []struct {
		description string
		original    string
		expected    string
	}{
		{
			description: "camel-case name",
			original:    "ServerSpec",
			expected:    "Server Spec",
		},
		{
			description: "entire camel-case name excepted",
			original:    "ElastiCache",
			expected:    "ElastiCache",
		},
		{
			description: "camel-case name with three words",
			original:    "MyExcellentResource",
			expected:    "My Excellent Resource",
		},
		{
			description: "camel-case name with version",
			original:    "ServerSpecV2",
			expected:    "Server Spec V2",
		},
		{
			description: "abbreviation",
			original:    "SAMLConnector",
			expected:    "SAML Connector",
		},
		{
			description: "idp",
			original:    "IdPSAMLOptions",
			expected:    "IdP SAML Options",
		},
		{
			description: "excepted word with abbreviation",
			original:    "MySQLOptions",
			expected:    "MySQL Options",
		},
		{
			description: "one abbreviation",
			original:    "AWS",
			expected:    "AWS",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.Equal(t, c.expected, refgen.SplitCamelCase(c.original, camelCaseExceptions))
		})
	}
}
