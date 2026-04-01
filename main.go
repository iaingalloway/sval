package main

import (
	"fmt"
	"os"

	"sval/cmd"
)

var version = "dev"

func main() {
	if err := cmd.Execute(version); err != nil {
		if err.Error() != "" {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
