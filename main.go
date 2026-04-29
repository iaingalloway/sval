package main

import (
	"errors"
	"fmt"
	"os"

	"sval/cmd"
)

var version = "dev"

func main() {
	if err := cmd.Execute(version); err != nil {
		if !errors.Is(err, cmd.ErrSilent) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
