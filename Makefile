.PHONY: embed build test

embed:
	mkdir -p mcp-servers/bridge/embedded
	cp skills/code-routing-full.md mcp-servers/bridge/embedded/SKILL.md
	cp skills/code-routing-slim.md mcp-servers/bridge/embedded/code-routing-slim.md
	cp hooks/subagent-code-routing.sh mcp-servers/bridge/embedded/subagent-code-routing.sh

build: embed
	cd mcp-servers/bridge && go build -o oh-my-bridge .

test: embed
	cd mcp-servers/bridge && go test -count=1 -race -timeout 120s ./...
