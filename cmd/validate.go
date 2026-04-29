package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/cobra"

	"sval/internal/config"
	"sval/internal/validator"
)

// ErrSilent signals that the command failed but has already written its own
// diagnostics. main.go uses errors.Is to suppress the default error print.
var ErrSilent = errors.New("")

func NewValidateCmd() *cobra.Command {
	var (
		schemaPath       string
		configPath       string
		configFromVSCode bool
		jsonOutput       bool
		failFast         bool
	)

	cmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate files against JSON schemas",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if schemaPath != "" && (configPath != "" || configFromVSCode) {
				return fmt.Errorf("--schema cannot be combined with --config or --config-from-vscode")
			}

			if schemaPath != "" {
				if len(args) != 1 {
					return fmt.Errorf("validate requires exactly one file argument when --schema is used")
				}
				return runSingleFile(cmd, args[0], schemaPath, jsonOutput)
			}

			cfg, cfgDir, err := resolveConfig(configPath, configFromVSCode)
			if err != nil {
				return err
			}
			return runUsingConfig(cmd, cfg, cfgDir, jsonOutput, failFast)
		},
	}

	cmd.Flags().StringVar(&schemaPath, "schema", "", "path to JSON schema file (single-file mode)")
	cmd.Flags().StringVar(&configPath, "config", "", "path to sval config file")
	cmd.Flags().BoolVar(&configFromVSCode, "config-from-vscode", false, "load schema rules from .vscode/settings.json")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output results as JSON")
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "stop after the first validation failure")

	return cmd
}

// runSingleFile validates one file against one explicitly-supplied schema.
func runSingleFile(cmd *cobra.Command, filePath, schemaPath string, jsonOutput bool) error {
	if validateFile(cmd, filePath, schemaPath, jsonOutput) {
		return ErrSilent
	}
	if !jsonOutput {
		fmt.Fprintln(cmd.OutOrStdout(), "Validated 1 file. All files are valid.")
	}
	return nil
}

// resolveConfig loads a Config from --config, --config-from-vscode, or auto-discovery.
// Returns the config and the directory it was loaded from (for resolving relative paths).
func resolveConfig(configFlag string, fromVSCode bool) (*config.Config, string, error) {
	if configFlag != "" {
		abs, err := filepath.Abs(configFlag)
		if err != nil {
			return nil, "", fmt.Errorf("resolve config path: %w", err)
		}
		cfg, err := config.Load(abs)
		if err != nil {
			return nil, "", fmt.Errorf("load config: %w", err)
		}
		return cfg, filepath.Dir(abs), nil
	}

	if fromVSCode {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, "", fmt.Errorf("get working directory: %w", err)
		}
		settingsPath := filepath.Join(cwd, ".vscode", "settings.json")
		cfg, err := config.FromVSCode(settingsPath)
		if err != nil {
			return nil, "", fmt.Errorf("load vscode config: %w", err)
		}
		// FromVSCode resolves rule paths to absolute itself; cwd is returned
		// as the base for resolving any relative ignore patterns.
		return cfg, cwd, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("get working directory: %w", err)
	}
	cfgPath, cfg, err := config.Discover(cwd)
	if err != nil {
		return nil, "", err
	}
	if cfg == nil {
		return nil, "", fmt.Errorf("no config file found in %s; use --config to specify one, or --schema for single-file validation", cwd)
	}
	return cfg, filepath.Dir(cfgPath), nil
}

// runUsingConfig expands each rule's glob pattern, filters ignored files, and validates.
func runUsingConfig(cmd *cobra.Command, cfg *config.Config, cfgDir string, jsonOutput bool, failFast bool) error {
	seen := make(map[string]struct{})
	validated := 0
	skipped := 0
	anyInvalid := false

	for _, rule := range cfg.Rules {
		pattern := resolveRel(cfgDir, rule.Pattern)
		schemaPath := resolveRel(cfgDir, rule.Schema)

		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			return fmt.Errorf("invalid glob %q: %w", rule.Pattern, err)
		}

		for _, path := range matches {
			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			if _, dup := seen[abs]; dup {
				continue
			}
			seen[abs] = struct{}{}

			if isIgnoredByConfig(abs, cfg.Ignore, cfgDir) {
				skipped++
				continue
			}

			if validator.DetectFileType(abs) == validator.FileTypeUnknown {
				skipped++
				continue
			}

			validated++
			if validateFile(cmd, abs, schemaPath, jsonOutput) {
				if failFast {
					return ErrSilent
				}
				anyInvalid = true
			}
		}
	}

	if anyInvalid {
		return ErrSilent
	}
	if !jsonOutput {
		fmt.Fprintf(cmd.OutOrStdout(), "Validated %d file(s), skipped %d. All files are valid.\n", validated, skipped)
	}
	return nil
}

func isIgnoredByConfig(absPath string, patterns []string, cfgDir string) bool {
	for _, pattern := range patterns {
		ok, err := doublestar.Match(resolveRel(cfgDir, pattern), absPath)
		if err == nil && ok {
			return true
		}
	}
	return false
}

// validateFile validates a single file against a schema and writes its result
// to the appropriate stream. Returns true if the file failed validation or
// could not be validated due to a system error.
func validateFile(cmd *cobra.Command, filePath, schemaPath string, jsonOutput bool) bool {
	if jsonOutput {
		result, err := validator.ValidatePathResult(filePath, schemaPath)
		if err != nil {
			// json.Marshal of a map[string]string cannot fail.
			out, _ := json.Marshal(map[string]string{"error": err.Error()})
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return true
		}
		// json.Marshal of validator.Result cannot fail for the values we produce.
		out, _ := json.Marshal(result)
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return !result.Valid
	}

	if err := validator.ValidatePath(filePath, schemaPath); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), err)
		return true
	}
	return false
}

// resolveRel returns p unchanged if it is absolute, otherwise joined onto base.
func resolveRel(base, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}
