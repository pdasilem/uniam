package cli

import _ "embed"

const (
	openCodeInstructionConfigRef = "./uniam-instructions.md"
	openCodeInstructionsFileName = "uniam-instructions.md"
	openCodePluginFileName       = "uniam.js"
)

//go:embed instructions/opencode/uniam-instructions.md
var openCodeInstructionsContent []byte

//go:embed plugins/opencode/uniam.js
var openCodePluginContent []byte
