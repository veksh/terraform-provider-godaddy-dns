// go:build tools

package tools

/*
import (
	// Documentation generation
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)
*/

// vscode complains that
//   `import "<...>/tfplugindocs" is a program, not an importable package`
// actually, it is a gopls bug in build tags support
// - see https://github.com/golang/go/issues/29202

// also it brings a lot of dependencies into the project, so lets drop it
// better get it from https://github.com/hashicorp/terraform-plugin-docs/
