// Package main is the entry point for the mark-guard CLI
package main

import (
	"fmt"
	"os"

	"github.com/ElshadHu/mark-guard/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
}
