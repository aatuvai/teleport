# Configuration reference generator

The configuration reference generator is a Go program that produces a comprehensive reference guide for each section in the `teleport.yaml` configuration file.

## Usage

From the root of your `gravitational/teleport` clone:

```
$ make gen-config-docs
```

## How it works

The configuration reference generator works by:

1. Identifying Go types that represent to the fields of each 
   configuration section struct identified in `lib/config/fileconf.go`.
2. Retrieving reference information about Teleport configuration sections 
   and their fields using a combination of Go comments and type information.

## Configuration

The generator uses a YAML configuration file with the following fields.

### Main config

- `source` (string): path to fileconf.go, which is used to generate the configuration reference.

- `destination` (string): the directory path in which to place reference pages.

- `sections` (array of configuration section objects): The configuration sections to represent in the reference docs.

- `examples_directory` (string): path to the directory where example YAML files are located.

- `camel_case_exceptions` (array of strings): a list of strings that should be exempt from conversion to title case in the reference section titles.

- `title_word_replacements` (array of strings): a list of known abbreviations to their replacements in section titles (e.g. "App" to "Application")

### Section configuration

In the `sections` field of the configuration file, each configuration section
object has the following structure:

- `type`: The name of the type declaration that represents the configuration section, e.g., `JamfService`.
- `introduction`: Introduction paragraph(s) to add to the template in place of the default text.

### Example

```yaml
source: "../../../../lib/config/fileconf.go"
destination: "../../../../docs/pages/reference/deployment/config"
examples_directory: "./section_examples"

sections:
  - type: Global
  - type: AccessGraph
  - type: Apps
  - type: Auth
  - type: Databases
  - type: DebugService
  - type: Discovery
  - type: JamfService
  - type: Kube
  - type: Metrics
  - type: Okta
  - type: PluginService
  - type: Proxy
  - type: Relay
    introduction: |
      The Relay Service is available in Teleport v18.3.0 and later.

      Represents the `relay_service` section of the configuration file.
  - type: SSH
  - type: TracingService
  - type: WindowsDesktopService

camel_case_exceptions:
  - AWS
  - AlloyDB
  - CORS
  - DocumentDB
  - EKS
  - ElastiCache
  - GCP
  - GitHub
  - GitLab
  - HTTP
  - IAM
  - ID
  - IdP
  - MFA
  - MemoryDB
  - MySQL
  - OIDC
  - OpenSearch
  - RDS
  - SAML
  - SQL
  - SSO
  - TLS
  - TPM

title_word_replacements:
  - App: Application
  - Db: Database
  - Ssh: SSH
  - Teleport: Instance-wide settings
```
