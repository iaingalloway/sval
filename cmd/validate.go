package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"sval/internal/validator"
)

func NewValidateCmd() *cobra.Command {
	var schemaPath string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate a file against a JSON schema",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if !jsonOutput {
				return validator.ValidatePath(ctx, args[0], schemaPath)
			}

			result, err := validator.ValidatePathResult(ctx, args[0], schemaPath)
			if err != nil {
				out, _ := json.Marshal(map[string]string{"error": err.Error()})
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return errors.New("")
			}

			out, _ := json.Marshal(result)
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			if !result.Valid {
				return errors.New("")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&schemaPath, "schema", "", "path to JSON schema file")
	_ = cmd.MarkFlagRequired("schema")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output results as JSON")

	return cmd
}
