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
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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
	titleWordReplacements := []map[string]string{
		{
			"App": "Application",
		},
	}

	cases := []struct {
		description string
		source      string
		expected    map[PackageInfo]ReferenceEntry
		// Go source fixtures that the test uses for named type fields.
		declSources []string
		// Substring to expect in a resulting error message
		errorSubstring string
		declInfo       PackageInfo
	}{
		{
			description: "scalar fields with one field ignored",
			declInfo: PackageInfo{
				DeclName:    "CachePolicy",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

// CachePolicy is used to control local cache. Every cache policy in Teleport has custom parameters.
type CachePolicy struct {
    // Type is for cache type sqlite or in-memory.
    Type string BACKTICKyaml:"type"BACKTICK
    // EnabledFlag is whether the cache is enabled.
    EnabledFlag string BACKTICKyaml:"-"BACKTICK
    // TTL sets maximum TTL for the cached values.
    TTL string BACKTICKyaml:"ttl,omitempty"BACKTICK
    // MaxBackoff sets the maximum backoff on error.
    MaxBackoff uint BACKTICKyaml:"max_backoff"BACKTICK
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "CachePolicy",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Cache Policy",
					Description: "Used to control local cache. Every cache policy in Teleport has custom parameters.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "CachePolicy",
					YAMLExample: `type: "string"
ttl: "string"
max_backoff: 1
`,
					Fields: []Field{
						Field{
							Name:        "max_backoff",
							Description: "Sets the maximum backoff on error.",
							Type:        "number",
						},
						Field{
							Name:        "ttl",
							Description: "Sets maximum TTL for the cached values.",
							Type:        "string",
						},
						Field{
							Name:        "type",
							Description: "For cache type sqlite or in-memory.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "sequences of scalars",
			declInfo: PackageInfo{
				DeclName:    "Log",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

// Log configures teleport logging. Every log structure in Teleport defines output levels.
type Log struct {
    // Outputs is a list of logging outputs.
    Outputs []string BACKTICKyaml:"outputs"BACKTICK
    // SeverityLevels is a list of severity levels.
    SeverityLevels []int BACKTICKyaml:"severity_levels"BACKTICK
    // Flags is a list of extra flags.
    Flags []bool BACKTICKyaml:"flags"BACKTICK
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Log",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Log",
					Description: "Configures teleport logging. Every log structure in Teleport defines output levels.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Log",
					YAMLExample: `outputs: 
  - "string"
  - "string"
  - "string"
severity_levels: 
  - 1
  - 1
  - 1
flags: 
  - true
  - true
  - true
`,
					Fields: []Field{
						Field{
							Name:        "flags",
							Description: "A list of extra flags.",
							Type:        "[]Boolean",
						},
						Field{
							Name:        "outputs",
							Description: "A list of logging outputs.",
							Type:        "[]string",
						},
						Field{
							Name:        "severity_levels",
							Description: "A list of severity levels.",
							Type:        "[]number",
						},
					},
				},
			},
		},
		{
			description: "a map of strings to sequences",
			declInfo: PackageInfo{
				DeclName:    "PluginService",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

// PluginService represents the configuration for the plugin service. Every plugin has map parameters.
type PluginService struct {
  // Plugins is a map of matchers for enabled plugin resources.
  Plugins map[string][]string BACKTICKyaml:"plugins"BACKTICK
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "PluginService",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Plugin Service",
					Description: "Represents the configuration for the plugin service. Every plugin has map parameters.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "PluginService",
					YAMLExample: `plugins: 
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
					Fields: []Field{
						Field{
							Name:        "plugins",
							Description: "A map of matchers for enabled plugin resources.",
							Type:        "map[string][]string",
						},
					},
				},
			},
		},
		{
			description: "an undeclared custom type field",
			declInfo: PackageInfo{
				DeclName:    "Auth",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

import "types"

// Auth includes information about auth service registered with Teleport.
type Auth struct {
    // ListenAddress is the listen address of the service.
    ListenAddress string BACKTICKyaml:"listen_addr"BACKTICK
    // Authentication contains authentication config.
    Authentication types.AuthenticationConfig BACKTICKyaml:"authentication"BACKTICK
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Auth",
					PackagePath: "github.com/gravitational/teleport/src",
				}: ReferenceEntry{
					SectionName: "Auth",
					Description: "Includes information about auth service registered with Teleport.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Auth",
					Fields: []Field{
						{
							Name:        "authentication",
							Description: "Contains authentication config.",
							Type:        "",
						},
						{
							Name:        "listen_addr",
							Description: "The listen address of the service.",
							Type:        "string",
						},
					},
					YAMLExample: "listen_addr: \"string\"\nauthentication: # See description\n",
				},
			},
		},
		{
			description: "named scalar type",
			declInfo: PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "Auth",
			},
			source: `package mypkg

import types "github.com/gravitational/teleport/src"

// Auth includes information about auth service registered with Teleport.
type Auth struct {
    // ListenAddress is the listen address of the service.
    ListenAddress string BACKTICKyaml:"listen_addr"BACKTICK
    // Authentication contains authentication config.
    Authentication types.AuthenticationConfig BACKTICKyaml:"authentication"BACKTICK
    // ClusterName specifies cluster name for the auth.
    ClusterName ClusterName BACKTICKyaml:"cluster_name"BACKTICK
}
`,
			declSources: []string{
				`package mypkg

// ClusterName is a named string that we'll process downstream
type ClusterName string
`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Auth",
					PackagePath: "github.com/gravitational/teleport/src",
				}: ReferenceEntry{
					SectionName: "Auth",
					Description: "Includes information about auth service registered with Teleport.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Auth",
					Fields: []Field{
						{
							Name:        "authentication",
							Description: "Contains authentication config.",
							Type:        "",
						},
						{
							Name:        "cluster_name",
							Description: "Specifies cluster name for the auth.",
							Type:        "[Cluster Name](#cluster-name)",
						},
						{
							Name:        "listen_addr",
							Description: "The listen address of the service.",
							Type:        "string",
						},
					},
					YAMLExample: "listen_addr: \"string\"\nauthentication: # See description\ncluster_name: # [...]\n",
				},
				PackageInfo{
					DeclName:    "ClusterName",
					PackagePath: "github.com/gravitational/teleport/src",
				}: ReferenceEntry{
					SectionName: "Cluster Name",
					Description: "A named string that we'll process downstream",
					SourcePath:  "github.com/gravitational/teleport/src/myfile0.go",
					YAMLName:    "ClusterName",
					Fields:      nil,
					YAMLExample: "",
				},
			},
		},
		{
			description: "custom type fields with a custom JSON unmarshaller",
			declInfo: PackageInfo{
				DeclName:    "Auth",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

import types "github.com/gravitational/teleport/src"

// Auth includes information about auth service registered with Teleport.
type Auth struct {
    // ListenAddress is the listen address of the service.
    ListenAddress string BACKTICKyaml:"listen_addr"BACKTICK
    // Authentication contains authentication config.
    Authentication types.AuthenticationConfig BACKTICKyaml:"authentication"BACKTICK
}
`,
			declSources: []string{
				`package mypkg

func (s *Auth) UnmarshalJSON (b []byte) error {
  return nil
}
`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Auth",
					PackagePath: "github.com/gravitational/teleport/src",
				}: ReferenceEntry{
					SectionName: "Auth",
					Description: "Includes information about auth service registered with Teleport.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Auth",
					Fields: []Field{
						{
							Name:        "authentication",
							Description: "Contains authentication config.",
							Type:        "",
						},
						{
							Name:        "listen_addr",
							Description: "The listen address of the service.",
							Type:        "string",
						},
					},
					YAMLExample: "listen_addr: \"string\"\nauthentication: # See description\n",
				},
			},
		},
		{
			description: "custom type with custom YAML unmarshaller",
			declInfo: PackageInfo{
				DeclName:    "Log",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

import types "github.com/gravitational/teleport/src"

// Log includes information about log service registered with Teleport.
type Log struct {
    // Output is the log output destination.
    Output string BACKTICKyaml:"output"BACKTICK
    // Format contains formatting options.
    Format types.LogFormat BACKTICKyaml:"format"BACKTICK
}
`,
			declSources: []string{
				`package mypkg

func (a *Log) UnmarshalYAML(value *yaml.Node) error {
  return nil
}
`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Log",
					PackagePath: "github.com/gravitational/teleport/src",
				}: ReferenceEntry{
					SectionName: "Log",
					Description: "Includes information about log service registered with Teleport.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Log",
					Fields: []Field{
						{
							Name:        "format",
							Description: "Contains formatting options.",
							Type:        "",
						},
						{
							Name:        "output",
							Description: "The log output destination.",
							Type:        "string",
						},
					},
					YAMLExample: "output: \"string\"\nformat: # See description\n",
				},
			},
		},
		{
			description: "a custom type field declared in a second source file",
			declInfo: PackageInfo{
				DeclName:    "Auth",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

import types "github.com/gravitational/teleport/src"

// Auth includes information about auth service registered with Teleport.
type Auth struct {
    // ListenAddress is the listen address of the service.
    ListenAddress string BACKTICKyaml:"listen_addr"BACKTICK
    // Authentication contains authentication config.
    Authentication types.AuthenticationConfig BACKTICKyaml:"authentication"BACKTICK
}
`,
			declSources: []string{`package types
// AuthenticationConfig includes aspects of auth preferences.
type AuthenticationConfig struct {
    // The type of the auth preference.
    Type string BACKTICKyaml:"type"BACKTICK
    // Timeout is session max duration.
    Timeout int BACKTICKyaml:"timeout"BACKTICK
    // IsLocal tells if local auth is enabled.
    IsLocal bool BACKTICKyaml:"is_local"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Auth",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Auth",
					Description: "Includes information about auth service registered with Teleport.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Auth",
					YAMLExample: `listen_addr: "string"
authentication: # [...]
`,
					Fields: []Field{
						Field{
							Name:        "authentication",
							Description: "Contains authentication config.",
							Type:        "[Authentication Config](#authentication-config)",
						},
						Field{
							Name:        "listen_addr",
							Description: "The listen address of the service.",
							Type:        "string",
						},
					},
				},
				PackageInfo{
					DeclName:    "AuthenticationConfig",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Authentication Config",
					Description: "Includes aspects of auth preferences.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile0.go",
					YAMLName:    "AuthenticationConfig",
					YAMLExample: `type: "string"
timeout: 1
is_local: true
`,
					Fields: []Field{
						Field{
							Name:        "is_local",
							Description: "Tells if local auth is enabled.",
							Type:        "Boolean",
						},
						Field{
							Name:        "timeout",
							Description: "Session max duration.",
							Type:        "number",
						},
						Field{
							Name:        "type",
							Description: "The type of the auth preference.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "composite field type with named scalar type",
			declInfo: PackageInfo{
				DeclName:    "Auth",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

import types "github.com/gravitational/teleport/src"

// Auth includes information about auth service registered with Teleport.
type Auth struct {
    // Authentication contains authentication config.
    Authentication types.AuthenticationConfig BACKTICKyaml:"authentication"BACKTICK
    // TokenMaps includes a map of strings to tokens.
    TokenMaps []map[string]types.StaticToken BACKTICKyaml:"token_maps"BACKTICK
}
`,
			declSources: []string{`package types
// AuthenticationConfig includes aspects of auth preferences.
type AuthenticationConfig struct {
    // The type of the auth preference.
    Type string BACKTICKyaml:"type"BACKTICK
}`,
				`package types

// StaticToken is a custom type that we unmarshal in a non-default way.
type StaticToken string
`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Auth",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Auth",
					Description: "Includes information about auth service registered with Teleport.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Auth",
					YAMLExample: `authentication: # [...]
token_maps: 
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
					Fields: []Field{
						Field{
							Name:        "authentication",
							Description: "Contains authentication config.",
							Type:        "[Authentication Config](#authentication-config)",
						},
						Field{
							Name:        "token_maps",
							Description: "Includes a map of strings to tokens.",
							Type:        "[]map[string][Static Token](#static-token)",
						},
					},
				},
				PackageInfo{
					DeclName:    "AuthenticationConfig",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Authentication Config",
					Description: "Includes aspects of auth preferences.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile0.go",
					YAMLName:    "AuthenticationConfig",
					YAMLExample: `type: "string"
`,
					Fields: []Field{
						Field{
							Name:        "type",
							Description: "The type of the auth preference.",
							Type:        "string",
						},
					},
				},
				PackageInfo{
					DeclName:    "StaticToken",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Static Token",
					Description: "A custom type that we unmarshal in a non-default way.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile1.go",
					YAMLName:    "StaticToken",
					Fields:      nil,
					YAMLExample: "",
				},
			},
		},
		{
			description: "struct type with an interface field",
			declInfo: PackageInfo{
				DeclName:    "JoinParams",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

// JoinParams includes information about Simplified Node Joining.
type JoinParams struct {
  // TokenName is the name of the token.
  TokenName string BACKTICKyaml:"token_name"BACKTICK
  // Method is the join method.
  Method JoinMethod BACKTICKyaml:"method"BACKTICK
}
`,
			declSources: []string{`package mypkg
// JoinMethod is a joining method with name.
type JoinMethod interface{
  GetMethodName() string
}
`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "JoinParams",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Join Params",
					Description: "Includes information about Simplified Node Joining.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "JoinParams",
					Fields: []Field{
						{
							Name:        "method",
							Description: "The join method.",
							Type:        "[Join Method](#join-method)",
						},
						{
							Name:        "token_name",
							Description: "The name of the token.",
							Type:        "string",
						},
					},
					YAMLExample: `token_name: "string"
method: # [...]
`,
				},
				PackageInfo{
					DeclName:    "JoinMethod",
					PackagePath: "github.com/gravitational/teleport/src",
				}: ReferenceEntry{
					SectionName: "Join Method",
					Description: "A joining method with name.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile0.go",
					YAMLName:    "JoinMethod",
					Fields:      nil,
					YAMLExample: "",
				},
			},
		},
		{
			description: "embedded struct",
			declInfo: PackageInfo{
				DeclName:    "SSH",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package mypkg

import types "github.com/gravitational/teleport/src"

// SSH is a SSH service config.
type SSH struct{
  // Namespace is the default namespace.
  Namespace string BACKTICKyaml:"namespace"BACKTICK
  types.Service
}
`,
			declSources: []string{
				`package types

// Service describes information about a common configurations.
type Service struct {
    // ListenAddress is the listen address.
    ListenAddress string BACKTICKyaml:"listen_addr"BACKTICK
    // Enabled indicates whether the service is enabled.
    Enabled bool BACKTICKyaml:"enabled"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "SSH",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "SSH",
					Description: "A SSH service config.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "SSH",
					Fields: []Field{
						{
							Name:        "enabled",
							Description: "Indicates whether the service is enabled.",
							Type:        "Boolean",
						},
						{
							Name:        "listen_addr",
							Description: "The listen address.",
							Type:        "string",
						},
						{
							Name:        "namespace",
							Description: "The default namespace.",
							Type:        "string",
						},
					},
					YAMLExample: `namespace: "string"
listen_addr: "string"
enabled: true
`,
				},
			},
		},
		{
			description: "embedded struct with struct field",
			declInfo: PackageInfo{
				DeclName:    "Proxy",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package mypkg

import types "github.com/gravitational/teleport/src"

// Proxy is a proxy configuration.
type Proxy struct{
  // WebAddr is the web UI listen address.
  WebAddr string BACKTICKyaml:"web_listen_addr"BACKTICK
  types.SSH
}
`,
			declSources: []string{
				`package types
type SSH struct {
    // Command is command details
    Command Command BACKTICKyaml:"commands"BACKTICK
}

// Command is command details.
type Command struct {
    // Name is the name of command.
    Name string BACKTICKyaml:"name"BACKTICK
    // Period is query period.
    Period bool BACKTICKyaml:"period"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Proxy",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Proxy",
					Description: "A proxy configuration.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Proxy",
					Fields: []Field{
						{
							Name:        "commands",
							Description: "Command details",
							Type:        "[Command](#command)",
						},
						{
							Name:        "web_listen_addr",
							Description: "The web UI listen address.",
							Type:        "string",
						},
					},
					YAMLExample: `web_listen_addr: "string"
commands: # [...]
`,
				},
				PackageInfo{
					DeclName:    "Command",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Command",
					SourcePath:  "github.com/gravitational/teleport/src/myfile0.go",
					Description: "Command details.",
					YAMLName:    "Command",
					Fields: []Field{
						{
							Name:        "name",
							Description: "The name of command.",
							Type:        "string",
						},
						{
							Name:        "period",
							Description: "Query period.",
							Type:        "Boolean",
						},
					},
					YAMLExample: `name: "string"
period: true
`,
				},
			},
		},
		{
			description: "embedded struct with base in the same package",
			declInfo: PackageInfo{
				DeclName:    "SSH",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package mypkg
// SSH is a SSH service config.
type SSH struct{
  // Namespace is the default namespace.
  Namespace string BACKTICKyaml:"namespace"BACKTICK
  Service
}
`,
			declSources: []string{
				`package mypkg

// Service describes information about a common configurations.
type Service struct {
    // ListenAddress is the listen address.
    ListenAddress string BACKTICKyaml:"listen_addr"BACKTICK
    // Enabled indicates whether the service is enabled.
    Enabled bool BACKTICKyaml:"enabled"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "SSH",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "SSH",
					Description: "A SSH service config.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "SSH",
					Fields: []Field{
						{
							Name:        "enabled",
							Description: "Indicates whether the service is enabled.",
							Type:        "Boolean",
						},
						{
							Name:        "listen_addr",
							Description: "The listen address.",
							Type:        "string",
						},
						{
							Name:        "namespace",
							Description: "The default namespace.",
							Type:        "string",
						},
					},
					YAMLExample: `namespace: "string"
listen_addr: "string"
enabled: true
`,
				},
			},
		},
		{
			description: "struct with two embedded structs",
			declInfo: PackageInfo{
				DeclName:    "SSH",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package mypkg

import moretypes "github.com/gravitational/teleport/src"
import types "github.com/gravitational/teleport/src"

// SSH is a SSH service config.
type SSH struct{
  // Namespace is the default namespace.
  Namespace string BACKTICKyaml:"namespace"BACKTICK
  types.Service
  moretypes.LegacyLog
}
`,
			declSources: []string{
				`package types

// Service describes information about a common configurations.
type Service struct {
    // ListenAddress is the listen address.
    ListenAddress string BACKTICKyaml:"listen_addr"BACKTICK
}`,
				`package moretypes

// LegacyLog represents logged events
type LegacyLog struct{
    // Active indicates whether the log is currently active.
    Active bool BACKTICKyaml:"active"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "SSH",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "SSH",
					Description: "A SSH service config.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "SSH",
					Fields: []Field{
						{
							Name:        "active",
							Description: "Indicates whether the log is currently active.",
							Type:        "Boolean",
						},
						{
							Name:        "listen_addr",
							Description: "The listen address.",
							Type:        "string",
						},
						{
							Name:        "namespace",
							Description: "The default namespace.",
							Type:        "string",
						},
					},
					YAMLExample: `namespace: "string"
listen_addr: "string"
active: true
`,
				},
			},
		},
		{
			description: "embedded struct with an embedded struct",
			declInfo: PackageInfo{
				DeclName:    "SSH",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package mypkg

import types "github.com/gravitational/teleport/src"

// SSH is a SSH service config.
type SSH struct{
  // Namespace is the default namespace.
  Namespace string BACKTICKyaml:"namespace"BACKTICK
  types.Service
}
`,
			declSources: []string{
				`package types

import moretypes "github.com/gravitational/teleport/src"

// Service describes information about a common configurations.
type Service struct {
    // ListenAddress is the listen address.
    ListenAddress string BACKTICKyaml:"listen_addr"BACKTICK
    moretypes.LegacyLog
}`,
				`package moretypes

// LegacyLog represents logged events
type LegacyLog struct{
    // Active indicates whether the log is currently active.
    Active bool BACKTICKyaml:"active"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "SSH",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "SSH",
					Description: "A SSH service config.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "SSH",
					Fields: []Field{
						{
							Name:        "active",
							Description: "Indicates whether the log is currently active.",
							Type:        "Boolean",
						},
						{
							Name:        "listen_addr",
							Description: "The listen address.",
							Type:        "string",
						},
						{
							Name:        "namespace",
							Description: "The default namespace.",
							Type:        "string",
						},
					},
					YAMLExample: `namespace: "string"
listen_addr: "string"
active: true
`,
				},
			},
		},
		{
			description: "ignored fields with non-YAML-comptabible types",
			declInfo: PackageInfo{
				DeclName:    "CachePolicy",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

// CachePolicy is used to control local cache. Every cache policy in Teleport has custom parameters.
type CachePolicy struct {
    // Type is for cache type sqlite or in-memory.
    Type string BACKTICKyaml:"type"BACKTICK
    XXX_NoUnkeyedLiteral struct{} BACKTICKyaml:"-"BACKTICK
    XXX_unrecognized     []byte   BACKTICKyaml:"-"BACKTICK
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "CachePolicy",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Cache Policy",
					Description: "Used to control local cache. Every cache policy in Teleport has custom parameters.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "CachePolicy",
					YAMLExample: `type: "string"
`,
					Fields: []Field{
						Field{
							Name:        "type",
							Description: "For cache type sqlite or in-memory.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "non-embedded custom field type declared in the same package as the containing struct",
			declInfo: PackageInfo{
				DeclName:    "Databases",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package typestest

// Databases represents database configuration.
type Databases struct {
	// Protocol is the database protocol.
	Protocol string BACKTICKyaml:"protocol"BACKTICK
	// TLS is database TLS connection settings.
	TLS DatabaseTLS BACKTICKyaml:"tls"BACKTICK
}
`,
			declSources: []string{
				`package typestest

// DatabaseTLS is TLS settings
type DatabaseTLS struct {
	// ServerName is the host name of LDAP.
	ServerName string BACKTICKyaml:"server_name"BACKTICK
	// CACertFile is ca path of LDAP.
	CACertFile string BACKTICKyaml:"ca_cert_file,omitempty"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Databases",
					PackagePath: "github.com/gravitational/teleport/src",
				}: ReferenceEntry{
					SectionName: "Databases",
					Description: "Represents database configuration.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Databases",
					Fields: []Field{
						Field{
							Name:        "protocol",
							Description: "The database protocol.",
							Type:        "string",
						},
						Field{
							Name:        "tls",
							Description: "Database TLS connection settings.",
							Type:        "[DatabaseTLS](#databasetls)",
						},
					},
					YAMLExample: `protocol: "string"
tls: # [...]
`,
				},
				PackageInfo{
					DeclName:    "DatabaseTLS",
					PackagePath: "github.com/gravitational/teleport/src",
				}: ReferenceEntry{
					SectionName: "DatabaseTLS",
					Description: "TLS settings",
					SourcePath:  "github.com/gravitational/teleport/src/myfile0.go",
					YAMLName:    "DatabaseTLS",
					Fields: []Field{
						{
							Name:        "ca_cert_file",
							Description: "Ca path of LDAP.",
							Type:        "string",
						},
						{
							Name:        "server_name",
							Description: "The host name of LDAP.",
							Type:        "string",
						},
					},
					YAMLExample: `server_name: "string"
ca_cert_file: "string"
`,
				},
			},
		},
		{
			description: "pointer field",
			declInfo: PackageInfo{
				DeclName:    "Databases",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `package typestest

// Databases represents database configuration.
type Databases struct {
	// TLS is database TLS connection settings.
	TLS *DatabaseTLS BACKTICKyaml:"tls"BACKTICK
}
`,
			declSources: []string{
				`package typestest

// DatabaseTLS is TLS settings
type DatabaseTLS struct {
	// ServerName is the host name of LDAP.
	ServerName string BACKTICKyaml:"server_name"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Databases",
					PackagePath: "github.com/gravitational/teleport/src",
				}: ReferenceEntry{
					SectionName: "Databases",
					Description: "Represents database configuration.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Databases",
					Fields: []Field{
						Field{
							Name:        "tls",
							Description: "Database TLS connection settings.",
							Type:        "[DatabaseTLS](#databasetls)",
						},
					},
					YAMLExample: `tls: # [...]
`,
				},
				PackageInfo{
					DeclName:    "DatabaseTLS",
					PackagePath: "github.com/gravitational/teleport/src",
				}: ReferenceEntry{
					SectionName: "DatabaseTLS",
					Description: "TLS settings",
					SourcePath:  "github.com/gravitational/teleport/src/myfile0.go",
					YAMLName:    "DatabaseTLS",
					Fields: []Field{
						{
							Name:        "server_name",
							Description: "The host name of LDAP.",
							Type:        "string",
						},
					},
					YAMLExample: `server_name: "string"
`,
				},
			},
		},
		{
			description: "map of strings to an undeclared field",
			declInfo: PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "Auth",
			},
			source: `
package mypkg

import types "github.com/gravitational/teleport/src"

// Auth includes information about auth service registered with Teleport.
type Auth struct {
    // ListenAddress is the listen address of the service.
    ListenAddress string BACKTICKyaml:"listen_addr"BACKTICK
    // TokenMaps includes a map of strings to tokens.
    TokenMaps []map[string]types.StaticToken BACKTICKyaml:"token_maps"BACKTICK
}
`,
			declSources: []string{`package types
// ServerSpecV1 includes aspects of a proxied server.
type ServerSpecV1 struct {
    // The address of the server.
    Address string BACKTICKyaml:"address"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Auth",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Auth",
					Description: "Includes information about auth service registered with Teleport.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Auth",
					YAMLExample: `listen_addr: "string"
token_maps: 
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
					Fields: []Field{
						Field{
							Name:        "listen_addr",
							Description: "The listen address of the service.",
							Type:        "string",
						},
						Field{
							Name:        "token_maps",
							Description: "Includes a map of strings to tokens.",
							Type:        "[]map[string]",
						},
					},
				},
			},
		},
		{
			description: "type parameter",
			declInfo: PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "Global",
			},
			source: `package mypkg
// Global configuration.
type Global struct {
  // NodeName is the node name.
  NodeName string BACKTICKyaml:"nodename"BACKTICK
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
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					PackagePath: "github.com/gravitational/teleport/src",
					DeclName:    "Global",
				}: ReferenceEntry{
					SectionName: "Global",
					Description: "Configuration.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Global",
					YAMLExample: `nodename: "string"
`,
					Fields: []Field{
						Field{
							Name:        "nodename",
							Description: "The node name.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "field type not declared in a loaded package",
			declInfo: PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "Global",
			},
			source: `package mypkg

import "time"

// Global configuration.
type Global struct {
  // NodeName is the node name.
  NodeName string BACKTICKyaml:"nodename"BACKTICK
  // Expiry is expiration time.
  Expiry time.Time BACKTICKyaml:"expiry"BACKTICK
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					PackagePath: "github.com/gravitational/teleport/src",
					DeclName:    "Global",
				}: ReferenceEntry{
					SectionName: "Global",
					Description: "Configuration.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Global",
					YAMLExample: `nodename: "string"
expiry: # See description
`,
					Fields: []Field{
						Field{
							Name:        "expiry",
							Description: "Expiration time.",
							Type:        "",
						},
						Field{
							Name:        "nodename",
							Description: "The node name.",
							Type:        "string",
						},
					},
				},
			},
			declSources: []string{},
		},
		{
			description: "byte slice",
			declInfo: PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "BoundKeypairParams",
			},
			source: `
package mypkg

// BoundKeypairParams contains parameters for joining.
type BoundKeypairParams struct {
    // RegistrationSecretValue is registration secret.
    RegistrationSecretValue string BACKTICKyaml:"registration_secret_value"BACKTICK
    // StaticPrivateKey is private key.
    StaticPrivateKey []byte BACKTICKyaml:"static_private_key"BACKTICK
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "BoundKeypairParams",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Bound Keypair Params",
					Description: "Contains parameters for joining.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "BoundKeypairParams",
					YAMLExample: `registration_secret_value: "string"
static_private_key: BASE64_STRING
`,
					Fields: []Field{
						Field{
							Name:        "registration_secret_value",
							Description: "Registration secret.",
							Type:        "string",
						},
						Field{
							Name:        "static_private_key",
							Description: "Private key.",
							Type:        "base64-encoded string",
						},
					},
				},
			},
		},
		{
			description: "named import in embedded struct field",
			declInfo: PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "Auth",
			},
			source: `
package mypkg

import types "github.com/gravitational/teleport/src"

// Auth includes information about auth service.
type Auth struct {
    // Spec contains SSH options.
    Spec types.SSH BACKTICKyaml:"spec"BACKTICK
}
`,
			declSources: []string{`package types

import alias "github.com/gravitational/teleport/src"

// SSH is SSH configuration.
type SSH struct {
  alias.Service
}`,
				`package otherpkg

type Service struct {
    // The listen address.
    Address string BACKTICKyaml:"address"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Auth",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Auth",
					Description: "Includes information about auth service.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Auth",
					YAMLExample: `spec: # [...]
`,
					Fields: []Field{
						Field{
							Name:        "spec",
							Description: "Contains SSH options.",
							Type:        "[SSH](#ssh)",
						},
					},
				},
				PackageInfo{
					DeclName:    "SSH",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "SSH",
					Description: "SSH configuration.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile0.go",
					YAMLName:    "SSH",
					YAMLExample: `address: "string"
`,
					Fields: []Field{
						Field{
							Name:        "address",
							Description: "The listen address.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "named import in named struct field",
			declInfo: PackageInfo{
				PackagePath: "github.com/gravitational/teleport/src",
				DeclName:    "Auth",
			},
			source: `
package mypkg

import types "github.com/gravitational/teleport/src"

// Auth includes information about auth service.
type Auth struct {
    // Spec contains SSH options.
    Spec types.SSH BACKTICKyaml:"spec"BACKTICK
}
`,
			declSources: []string{`package types
import alias "github.com/gravitational/teleport/src"

// SSH is SSH configuration.
type SSH struct {
  // Info is address info.
  Info alias.AddressInfo BACKTICKyaml:"info"BACKTICK
}`,

				`package otherpkg
// AddressInfo provides information about an address.
type AddressInfo struct {
    // The address of the server.
    Address string BACKTICKyaml:"address"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "AddressInfo",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Address Info",
					Description: "Provides information about an address.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile1.go",
					YAMLName:    "AddressInfo",
					Fields: []Field{
						{
							Name:        "address",
							Description: "The address of the server.",
							Type:        "string",
						},
					},
					YAMLExample: "address: \"string\"\n",
				},
				PackageInfo{
					DeclName:    "Auth",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Auth",
					Description: "Includes information about auth service.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "Auth",
					YAMLExample: `spec: # [...]
`,
					Fields: []Field{
						Field{
							Name:        "spec",
							Description: "Contains SSH options.",
							Type:        "[SSH](#ssh)"},
					},
				},
				PackageInfo{
					DeclName:    "SSH",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "SSH",
					Description: "SSH configuration.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile0.go",
					YAMLName:    "SSH",
					YAMLExample: `info: # [...]
`,
					Fields: []Field{
						Field{
							Name:        "info",
							Description: "Address info.",
							Type:        "[Address Info](#address-info)",
						},
					},
				},
			},
		},
		{
			description: "scalar fields with two unexported fields",
			declInfo: PackageInfo{
				DeclName:    "CachePolicy",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

import "protoimpl"

// CachePolicy is used to control local cache. Every cache policy in Teleport has custom parameters.
type CachePolicy struct {
    // Type is for cache type.
    Type string BACKTICKyaml:"type"BACKTICK
    // TTL sets maximum TTL for the cached values.
    TTL string BACKTICKyaml:"ttl,omitempty"BACKTICK
    state protoimpl.MessageState BACKTICKyaml:"-"BACKTICK
    unknownFields protoimpl.UnknownFields
    sizeCache     protoimpl.SizeCache
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "CachePolicy",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Cache Policy",
					Description: "Used to control local cache. Every cache policy in Teleport has custom parameters.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "CachePolicy",
					YAMLExample: `type: "string"
ttl: "string"
`,
					Fields: []Field{
						Field{
							Name:        "ttl",
							Description: "Sets maximum TTL for the cached values.",
							Type:        "string",
						},
						Field{
							Name:        "type",
							Description: "For cache type.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "curly braces in descriptions",
			declInfo: PackageInfo{
				DeclName:    "CachePolicy",
				PackagePath: "github.com/gravitational/teleport/src",
			},
			source: `
package mypkg

// CachePolicy is used to control {local cache}. Every cache policy in Teleport has custom parameters.
type CachePolicy struct {
    // Type is the {cache type}.
    Type string BACKTICKyaml:"type"BACKTICK
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "CachePolicy",
					PackagePath: "github.com/gravitational/teleport/src",
				}: {
					SectionName: "Cache Policy",
					Description: "Used to control `{local cache}`. Every cache policy in Teleport has custom parameters.",
					SourcePath:  "github.com/gravitational/teleport/src/myfile.go",
					YAMLName:    "CachePolicy",
					YAMLExample: `type: "string"
`,
					Fields: []Field{
						Field{
							Name:        "type",
							Description: "The `{cache type}`.",
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

			sourceData, err := NewSourceData("github.com/gravitational/teleport", filepath.Join(tmp, "src", "myfile.go"))
			if err != nil {
				t.Fatal(err)
			}

			for i := range tc.declSources {
				path := filepath.Join(tmp, "src", "myfile"+strconv.Itoa(i)+".go")
				sd, err := NewSourceData("github.com/gravitational/teleport", path)
				if err != nil {
					t.Fatal(err)
				}
				for k, v := range sd.TypeDecls {
					sourceData.TypeDecls[k] = v
				}
			}

			// Used to adjust package paths in test cases to remove the
			// temporary directory since it can't be known in advance.
			cleanPath := func(p string) string {
				if index := strings.Index(p, "/src"); index != -1 {
					return "github.com/gravitational/teleport" + p[index:]
				}
				return p
			}
			// Remove the temporary directory from package paths
			// since we can't know it in advance in test cases.
			declsWithoutTmp := make(map[PackageInfo]DeclarationInfo)
			for k, d := range sourceData.TypeDecls {
				k.PackagePath = cleanPath(k.PackagePath)
				d.PackageName = cleanPath(d.PackageName)
				d.FilePath = cleanPath(d.FilePath)
				declsWithoutTmp[k] = d
			}

			di, ok := declsWithoutTmp[tc.declInfo]
			if !ok {
				t.Fatalf("expected data for %v.%v not found in the source", tc.declInfo.PackagePath, tc.declInfo.DeclName)
			}

			r, err := ReferenceDataFromDeclaration("github.com/gravitational/teleport", di, declsWithoutTmp, camelCaseExceptions, titleWordReplacements)
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
			assert.Equal(t, c.expected, NamedImports(f))
		})
	}
}

func TestMakeFieldTableInfo(t *testing.T) {
	camelCaseExceptions := []string{
		"IdP",
	}

	cases := []struct {
		description string
		input       []rawField
		expected    []Field
	}{
		{
			description: "angle brackets in GoDoc",
			input: []rawField{
				rawField{
					packageName: "mypkg",
					doc:         `An ID, e.g., "<myid>"`,
					kind:        yamlString{},
					name:        "ObjectID",
					yamlName:    "object_id",
					tags:        `yaml:"object_id"`,
				},
			},
			expected: []Field{
				{
					Name:        "object_id",
					Description: `An ID, e.g., "\<myid\>"`,
					Type:        "string",
				},
			},
		},
		{
			description: "pipe in field description",
			input: []rawField{
				{
					packageName: "mypkg",
					doc:         "Specifies the locking mode (strict|best_effort) to be applied with the role.",
					kind: yamlCustomType{
						name: "LockingMode",
						declarationInfo: PackageInfo{
							DeclName:    "LockingMode",
							PackagePath: "mypkg",
						},
					},
					name:     "LockingMode",
					yamlName: "locking_mode",
					tags:     "yaml:\"locking_mode\"",
				},
			},
			expected: []Field{
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
			f, err := makeFieldTableInfo(c.input, camelCaseExceptions)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, f)
		})
	}
}

func TestGetYAMLTag(t *testing.T) {
	cases := []struct {
		description string
		input       string
		expected    string
	}{
		{
			description: "one well-formed struct tag",
			input:       `yaml:"my_tag"`,
			expected:    "my_tag",
		},
		{
			description: "multiple well-formed struct tags",
			input:       `json:"json_tag" yaml:"yaml_tag" other:"other-tag"`,
			expected:    "yaml_tag",
		},
		{
			description: "omitempty option in tag value",
			input:       `yaml:"yaml_tag,omitempty" other:"other-tag"`,
			expected:    "yaml_tag",
		},
		{
			description: "No YAML tag",
			input:       `other:"other-tag"`,
			expected:    "",
		},
		{
			description: "Empty YAML tag with the omitempty option",
			input:       `yaml:",omitempty" other:"other-tag"`,
			expected:    "",
		},
		{
			description: "Ignored YAML field",
			input:       `yaml:"-" other:"other-tag"`,
			expected:    "-",
		},
		{
			description: "empty YAML tag",
			input:       `yaml:"" other:"other-tag"`,
			expected:    "",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			g := getYAMLTag(c.input)
			assert.Equal(t, c.expected, g)
		})
	}
}

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

	allDecls := map[PackageInfo]DeclarationInfo{
		{DeclName: "Foo", PackagePath: protoPkg}: {
			Decl:        protoFile.Decls[0],
			FilePath:    "proto/proto.go",
			PackageName: protoPkg,
		},
		{DeclName: "Foo", PackagePath: wrapperPkg}: {
			Decl:         wrapperFile.Decls[1], // skip import decl
			FilePath:     "wrapper/wrapper.go",
			PackageName:  wrapperPkg,
			NamedImports: NamedImports(wrapperFile),
		},
	}

	wrapperDecl := allDecls[PackageInfo{DeclName: "Foo", PackagePath: wrapperPkg}]
	rs, err := typeForDecl(wrapperDecl, allDecls)
	require.NoError(t, err)

	_, err = allFieldsForDecl(wrapperDecl, rs.fields, allDecls)
	require.NoError(t, err)
}

func TestGetSectionYAMLName(t *testing.T) {
	fset := token.NewFileSet()
	cleanedCode := replaceBackticks(string(`package mypkg
type FileConfig struct {
	Apps Apps BACKTICKyaml:"app_service,omitempty"BACKTICK
	SSH  SSH  BACKTICKyaml:"ssh_service,omitempty"BACKTICK
}
`))
	f, err := parser.ParseFile(fset, "mock.go", cleanedCode, parser.ParseComments)
	require.NoError(t, err)

	allDecls := map[PackageInfo]DeclarationInfo{
		{DeclName: "FileConfig", PackagePath: "github.com/gravitational/teleport/src"}: {
			Decl:        f.Decls[0],
			FilePath:    "src/mock.go",
			PackageName: "github.com/gravitational/teleport/src",
		},
		{DeclName: "Apps", PackagePath: "github.com/gravitational/teleport/src"}: {
			Decl:        f.Decls[0],
			FilePath:    "src/mock.go",
			PackageName: "github.com/gravitational/teleport/src",
		},
		{DeclName: "SSH", PackagePath: "github.com/gravitational/teleport/src"}: {
			Decl:        f.Decls[0],
			FilePath:    "src/mock.go",
			PackageName: "github.com/gravitational/teleport/src",
		},
	}

	yamlName, _ := getSectionYAMLName(allDecls, "Apps")
	assert.Equal(t, "app_service", yamlName)
	yamlName, _ = getSectionYAMLName(allDecls, "SSH")
	assert.Equal(t, "ssh_service", yamlName)
	yamlName, _ = getSectionYAMLName(allDecls, "UndeclaredType")
	assert.Equal(t, "UndeclaredType", yamlName)
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
			assert.Equal(t, c.expected, printableDescription(c.input, c.name, ""))
		})
	}
}

func TestMakeYAMLExample(t *testing.T) {
	cases := []struct {
		description string
		input       []rawField
		expected    string
	}{
		{
			description: "all scalars",
			input: []rawField{
				rawField{
					doc:  "myInt is an int",
					kind: yamlNumber{},
					name: "myInt",
					tags: `yaml:"my_int"`,
				},
				rawField{
					doc:  "myBool is a Boolean",
					kind: yamlBool{},
					name: "myBool",
					tags: `yaml:"my_bool"`,
				},
				rawField{
					doc:  "myString is a string",
					kind: yamlString{},
					tags: `yaml:"my_string"`,
				},
			},
			expected: `my_int: 1
my_bool: true
my_string: "string"
`,
		},
		{
			description: "sequence of sequence of strings",
			input: []rawField{
				rawField{
					name: "mySeq",
					doc:  "mySeq is a sequence of sequences of strings",
					tags: `yaml:"my_seq"`,
					kind: yamlSequence{
						elementKind: yamlSequence{
							elementKind: yamlString{},
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
			input: []rawField{
				rawField{
					name: "myMap",
					doc:  "myMap is a map of ints to strings",
					tags: `yaml:"my_map"`,
					kind: yamlMapping{
						keyKind:   yamlNumber{},
						valueKind: yamlString{},
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
			input: []rawField{
				rawField{
					name: "mySeq",
					doc:  "mySeq is a complex type",
					tags: `yaml:"my_seq"`,
					kind: yamlSequence{
						elementKind: yamlMapping{
							keyKind:   yamlString{},
							valueKind: yamlBool{},
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
			input: []rawField{
				rawField{
					name: "labels",
					doc:  "labels is a list of labels",
					tags: `yaml:"labels"`,
					kind: yamlSequence{
						elementKind: yamlCustomType{
							name: "label",
							declarationInfo: PackageInfo{
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
			input: []rawField{
				rawField{
					name: "labels",
					doc:  "labels is a map of strings to labels",
					tags: `yaml:"labels"`,
					kind: yamlMapping{
						keyKind: yamlString{},
						valueKind: yamlCustomType{
							name: "label",
							declarationInfo: PackageInfo{
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
			e, err := makeYAMLExample(c.input)
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
			assert.Equal(t, c.expected, splitCamelCase(c.original, camelCaseExceptions))
		})
	}
}
