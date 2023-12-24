// Format Terraform code for use in documentation
// optional, requires terraform installed
//go:generate terraform fmt -recursive ../examples/
// Generate documentation.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-dir ..

package tools

import (
	// Documentation generation
	//nolint:go list
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)

// vscode complains that
//   `import "<...>/tfplugindocs" is a program, not an importable package`
// actually, it is a gopls bug in build tags support
// - see https://github.com/golang/go/issues/29202
