package main

import (
	"fmt"
	"io"
)

const usage = `GoChain is a small educational blockchain.

Usage:
  gochain <command>

Commands:
  help    Show this help message
`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 1
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, usage)
}
