package main

import _ "embed"

//go:embed embedded/SKILL.md
var embeddedSkillMD []byte

//go:embed embedded/code-routing-slim.md
var embeddedSlimMD []byte

//go:embed embedded/subagent-code-routing.sh
var embeddedHookSH []byte
