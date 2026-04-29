package main

import (
	"errors"
	"fmt"
	"os"

	"sval/cmd"
)

var version = "dev"

func main() {
	err := cmd.Execute(version)
	if err == nil {
		return
	}
	if errors.Is(err, cmd.ErrSilent) {
		// Validation failure: per-file diagnostics already printed.
		os.Exit(1)
	}
	// Usage, configuration, or system error.
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}
