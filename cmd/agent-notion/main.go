// Command agent-notion is the Notion CLI for humans and LLMs.
package main

import "github.com/shhac/agent-notion/internal/cli"

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cli.Run(version)
}
