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

func NewValidateCmd() *cobra.Command {
	var schemaPath string
	var configPath string
	var configFromVSCode bool
	var jsonOutput bool

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
				return runSchemaMode(cmd, args[0], schemaPath, jsonOutput)
			}

			cfg, cfgDir, err := resolveConfig(configPath, configFromVSCode)
			if err != nil {
				return err
			}
			return runConfigMode(cmd, cfg, cfgDir, jsonOutput)
		},
	}

	cmd.Flags().StringVar(&schemaPath, "schema", "", "path to JSON schema file (single-file mode)")
	cmd.Flags().StringVar(&configPath, "config", "", "path to sval config file")
	cmd.Flags().BoolVar(&configFromVSCode, "config-from-vscode", false, "load schema rules from .vscode/settings.json")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output results as JSON")

	return cmd
}

// runSchemaMode is the original single-file path: validate one file against one schema.
func runSchemaMode(cmd *cobra.Command, filePath, schemaPath string, jsonOutput bool) error {
	if !jsonOutput {
		if err := validator.ValidatePath(filePath, schemaPath); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Validated 1 file. All files are valid.\n")
		return nil
	}

	result, err := validator.ValidatePathResult(filePath, schemaPath)
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
		// FromVSCode already resolves all paths to absolute, so cfgDir is unused
		// but set to cwd for any ignore patterns.
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

// runConfigMode expands each rule's glob pattern, filters ignored files, and validates.
func runConfigMode(cmd *cobra.Command, cfg *config.Config, cfgDir string, jsonOutput bool) error {
	seen := make(map[string]struct{})
	anyInvalid := false

	for _, rule := range cfg.Rules {
		pattern := rule.Pattern
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(cfgDir, pattern)
		}
		schemaPath := rule.Schema
		if !filepath.IsAbs(schemaPath) {
			schemaPath = filepath.Join(cfgDir, schemaPath)
		}

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
				continue
			}

			if validator.DetectFileType(abs) == validator.FileTypeUnknown {
				continue
			}

			if jsonOutput {
				result, err := validator.ValidatePathResult(abs, schemaPath)
				if err != nil {
					out, _ := json.Marshal(map[string]string{"error": err.Error()})
					fmt.Fprintln(cmd.OutOrStdout(), string(out))
					anyInvalid = true
					continue
				}
				out, _ := json.Marshal(result)
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				if !result.Valid {
					anyInvalid = true
				}
			} else {
				if err := validator.ValidatePath(abs, schemaPath); err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), err)
					anyInvalid = true
				}
			}
		}
	}

	if anyInvalid {
		return errors.New("")
	}
	if !jsonOutput {
		fmt.Fprintf(cmd.OutOrStdout(), "Validated %d file(s). All files are valid.\n", len(seen))
	}
	return nil
}

func isIgnoredByConfig(absPath string, patterns []string, cfgDir string) bool {
	for _, pattern := range patterns {
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(cfgDir, pattern)
		}
		ok, err := doublestar.Match(pattern, absPath)
		if err == nil && ok {
			return true
		}
	}
	return false
}
