package main

import (
	"fmt"
	"os"

	"github.com/c-w-xiaohei/gptx/internal/cli"
)

func main() {
	cmd := cli.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
