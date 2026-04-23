package cli

import _ "embed"

const (
	openCodeInstructionsFileName = "uniam-instructions.md"
	openCodePluginFileName       = "uniam.js"
)

//go:embed plugins/opencode/uniam.js
var openCodePluginContent []byte
