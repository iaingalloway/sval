package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func Execute(version string) error {
	root := newRootCmd(version)
	return root.Execute()
}

func newRootCmd(version string) *cobra.Command {
	var showVersion bool

	root := &cobra.Command{
		Use:           "sval",
		Short:         "Sval schema validator",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), version)
				return err
			}
			_ = cmd.Help()
			return errors.New("")
		},
	}

	initVersionFlag(root.Flags(), &showVersion)
	root.AddCommand(NewValidateCmd())
	return root
}

func initVersionFlag(flags *pflag.FlagSet, showVersion *bool) {
	flags.BoolVar(showVersion, "version", false, "print version")
}
