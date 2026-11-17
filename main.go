package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printUsage()
		return fmt.Errorf("missing subcommand")
	}
	switch os.Args[1] {
	case "serve":
		return runServe(os.Args[2:])
	case "version":
		printVersion()
		return nil
	default:
		printUsage()
		return fmt.Errorf("%q is not a valid subcommand", os.Args[1])
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  sakura-secrets-socket serve -socket <path>")
	fmt.Fprintln(os.Stderr, "  sakura-secrets-socket version")
}
