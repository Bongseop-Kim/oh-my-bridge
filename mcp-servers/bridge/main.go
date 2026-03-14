package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// config 서브커맨드 분기 — MCP 서버 기동 전에 처리
	if len(os.Args) > 1 && os.Args[1] == "config" {
		var err error
		cfg, err = loadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "oh-my-bridge: config load error: %v\n", err)
			os.Exit(1)
		}
		availableCLIs = detectCLIs(cfg)
		runConfigCommand(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println(serverVersion)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		runDoctor()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "stats" {
		runStats()
		return
	}

	// MCP 서버 모드 (기존 동작)
	var err error
	cfg, err = loadConfig()
	if err != nil {
		log.Fatalf("oh-my-bridge: %v", err)
	}
	availableCLIs = detectCLIs(cfg)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delegate",
		Description: "Delegate a code generation task to the best available AI model.",
	}, delegateTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "status",
		Description: "Return current config routes, model definitions, and CLI availability.",
	}, statusTool)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
