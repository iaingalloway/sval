package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/iaingalloway/sval/cmd"
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
	// Configuration or system error.
	if msg := err.Error(); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
	}
	os.Exit(2)
}
