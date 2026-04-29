package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// verbosity controls how much output the command emits.
type verbosity int

const (
	verbosityQuiet verbosity = iota
	verbositySummary
	verbosityDefault
	verbosityVerbose
	verbosityDiag
)

func parseVerbosity(s string) (verbosity, error) {
	switch s {
	case "quiet":
		return verbosityQuiet, nil
	case "summary":
		return verbositySummary, nil
	case "default":
		return verbosityDefault, nil
	case "verbose":
		return verbosityVerbose, nil
	case "diag", "diagnostic":
		return verbosityDiag, nil
	default:
		return 0, fmt.Errorf("invalid --verbosity %q (want quiet|summary|default|verbose|diag)", s)
	}
}

func NewValidateCmd() *cobra.Command {
	var (
		schemaPath       string
		configPath       string
		configFromVSCode bool
		jsonOutput       bool
		failFast         bool

		verbosityFlag string
		quietFlag     bool
		summaryFlag   bool
		verboseFlag   bool
		diagFlag      bool
		diagAliasFlag bool
	)

	cmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate files against JSON schemas",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if schemaPath != "" && (configPath != "" || configFromVSCode) {
				return fmt.Errorf("--schema cannot be combined with --config or --config-from-vscode")
			}

			level, err := resolveVerbosity(verbosityFlag, quietFlag, summaryFlag, verboseFlag, diagFlag || diagAliasFlag)
			if err != nil {
				return err
			}
			if jsonOutput && (verbosityFlag != "" || quietFlag || summaryFlag || verboseFlag || diagFlag || diagAliasFlag) {
				return fmt.Errorf("--json cannot be combined with --verbosity / --quiet / --summary / --verbose / --diag")
			}

			rep := newReporter(cmd.OutOrStdout(), cmd.ErrOrStderr(), level, jsonOutput)

			if schemaPath != "" {
				if len(args) != 1 {
					return fmt.Errorf("validate requires exactly one file argument when --schema is used")
				}
				return runSingleFile(rep, args[0], schemaPath)
			}

			cfg, cfgDir, cfgPath, err := resolveConfig(configPath, configFromVSCode)
			if err != nil {
				return err
			}
			rep.diag("config: %s", cfgPath)
			return runUsingConfig(rep, cfg, cfgDir, failFast)
		},
	}

	cmd.Flags().StringVar(&schemaPath, "schema", "", "path to JSON schema file (single-file mode)")
	cmd.Flags().StringVar(&configPath, "config", "", "path to sval config file")
	cmd.Flags().BoolVar(&configFromVSCode, "config-from-vscode", false, "load schema rules from .vscode/settings.json")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "output results as JSON")
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "stop after the first validation failure")

	cmd.Flags().StringVar(&verbosityFlag, "verbosity", "", "output level: quiet|summary|default|verbose|diag")
	cmd.Flags().BoolVarP(&quietFlag, "quiet", "q", false, "no output; rely on exit code (shortcut for --verbosity quiet)")
	cmd.Flags().BoolVar(&summaryFlag, "summary", false, "only print the aggregate summary line (shortcut for --verbosity summary)")
	cmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "also print OK lines and per-rule expansion (shortcut for --verbosity verbose)")
	cmd.Flags().BoolVar(&diagFlag, "diag", false, "verbose plus diagnostic info (shortcut for --verbosity diag)")
	cmd.Flags().BoolVar(&diagAliasFlag, "diagnostic", false, "alias for --diag")

	return cmd
}

// resolveVerbosity collapses the explicit --verbosity flag and the shortcut
// bool flags into a single level. At most one shortcut may be set, and a
// shortcut combined with --verbosity is rejected.
func resolveVerbosity(s string, quiet, summary, verbose, diag bool) (verbosity, error) {
	shortcuts := 0
	for _, b := range []bool{quiet, summary, verbose, diag} {
		if b {
			shortcuts++
		}
	}
	if shortcuts > 1 {
		return 0, fmt.Errorf("at most one of --quiet / --summary / --verbose / --diag may be set")
	}
	if s != "" && shortcuts > 0 {
		return 0, fmt.Errorf("--verbosity cannot be combined with --quiet / --summary / --verbose / --diag")
	}
	if s != "" {
		return parseVerbosity(s)
	}
	switch {
	case quiet:
		return verbosityQuiet, nil
	case summary:
		return verbositySummary, nil
	case verbose:
		return verbosityVerbose, nil
	case diag:
		return verbosityDiag, nil
	default:
		return verbosityDefault, nil
	}
}

// runSingleFile validates one file against one explicitly-supplied schema.
func runSingleFile(rep *reporter, filePath, schemaPath string) error {
	failed := validateFile(rep, filePath, schemaPath)
	if failed {
		rep.summary(0, 0, 1)
		return ErrSilent
	}
	rep.fileOK(filePath)
	rep.singleFileSuccess()
	return nil
}

// resolveConfig loads a Config from --config, --config-from-vscode, or auto-discovery.
// Returns the config, the directory it was loaded from (for resolving relative
// paths), and the path of the config file/source itself (for diagnostics).
func resolveConfig(configFlag string, fromVSCode bool) (*config.Config, string, string, error) {
	if configFlag != "" {
		abs, err := filepath.Abs(configFlag)
		if err != nil {
			return nil, "", "", fmt.Errorf("resolve config path: %w", err)
		}
		cfg, err := config.Load(abs)
		if err != nil {
			return nil, "", "", fmt.Errorf("load config: %w", err)
		}
		return cfg, filepath.Dir(abs), abs, nil
	}

	if fromVSCode {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, "", "", fmt.Errorf("get working directory: %w", err)
		}
		settingsPath := filepath.Join(cwd, ".vscode", "settings.json")
		cfg, err := config.FromVSCode(settingsPath)
		if err != nil {
			return nil, "", "", fmt.Errorf("load vscode config: %w", err)
		}
		// FromVSCode resolves rule paths to absolute itself; cwd is returned
		// as the base for resolving any relative ignore patterns.
		return cfg, cwd, settingsPath, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", "", fmt.Errorf("get working directory: %w", err)
	}
	cfgPath, cfg, err := config.Discover(cwd)
	if err != nil {
		return nil, "", "", err
	}
	if cfg == nil {
		return nil, "", "", fmt.Errorf("no config file found in %s; use --config to specify one, or --schema for single-file validation", cwd)
	}
	return cfg, filepath.Dir(cfgPath), cfgPath, nil
}

// runUsingConfig expands each rule's glob pattern, filters ignored files, and validates.
func runUsingConfig(rep *reporter, cfg *config.Config, cfgDir string, failFast bool) error {
	seen := make(map[string]struct{})
	validated := 0
	skipped := 0
	failed := 0

	for _, rule := range cfg.Rules {
		pattern := resolveRel(cfgDir, rule.Pattern)
		schemaPath := resolveRel(cfgDir, rule.Schema)

		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			return fmt.Errorf("invalid glob %q: %w", rule.Pattern, err)
		}
		rep.ruleExpanded(rule.Pattern, len(matches))

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
				rep.diag("ignored: %s", abs)
				continue
			}

			if validator.DetectFileType(abs) == validator.FileTypeUnknown {
				skipped++
				rep.diag("unknown file type, skipping: %s", abs)
				continue
			}

			validated++
			if validateFile(rep, abs, schemaPath) {
				failed++
				if failFast {
					rep.summary(validated, skipped, failed)
					return ErrSilent
				}
			} else {
				rep.fileOK(abs)
			}
		}
	}

	rep.summary(validated, skipped, failed)
	if failed > 0 {
		return ErrSilent
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

// validateFile validates a single file against a schema. Returns true if the
// file failed validation or could not be validated due to a system error.
func validateFile(rep *reporter, filePath, schemaPath string) bool {
	if rep.json {
		result, err := validator.ValidatePathResult(filePath, schemaPath)
		if err != nil {
			rep.jsonError(err)
			return true
		}
		rep.jsonResult(result)
		return !result.Valid
	}

	if err := validator.ValidatePath(filePath, schemaPath); err != nil {
		rep.fileError(err)
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

// reporter routes per-file and aggregate output to stdout / stderr based on
// the configured verbosity level. In JSON mode, only jsonResult/jsonError emit;
// all other methods are no-ops so the surrounding control flow stays uniform.
type reporter struct {
	out   io.Writer
	err   io.Writer
	level verbosity
	json  bool
}

func newReporter(out, errW io.Writer, level verbosity, jsonOutput bool) *reporter {
	return &reporter{out: out, err: errW, level: level, json: jsonOutput}
}

// fileOK announces a successfully-validated file. Verbose+ only.
func (r *reporter) fileOK(path string) {
	if r.json || r.level < verbosityVerbose {
		return
	}
	fmt.Fprintf(r.out, "OK: %s\n", path)
}

// fileError prints a per-file validation error. Default+ only (suppressed by
// quiet and summary).
func (r *reporter) fileError(err error) {
	if r.json || r.level < verbosityDefault {
		return
	}
	fmt.Fprintln(r.err, err)
}

// ruleExpanded reports how many files a glob pattern matched. Verbose+ only.
func (r *reporter) ruleExpanded(pattern string, n int) {
	if r.json || r.level < verbosityVerbose {
		return
	}
	fmt.Fprintf(r.err, "pattern %q matched %d file(s)\n", pattern, n)
}

// diag emits a diagnostic message. Diag level only.
func (r *reporter) diag(format string, args ...any) {
	if r.json || r.level < verbosityDiag {
		return
	}
	fmt.Fprintf(r.err, "diag: "+format+"\n", args...)
}

// summary emits the aggregate count line. Suppressed below summary level. At
// default level it is also suppressed when there are failures (per-file errors
// already convey the result); at summary+ it is always emitted because per-file
// errors may be silenced.
func (r *reporter) summary(validated, skipped, failed int) {
	if r.json || r.level < verbositySummary {
		return
	}
	if failed > 0 && r.level == verbosityDefault {
		return
	}
	if failed > 0 {
		fmt.Fprintf(r.out, "Validated %d file(s), skipped %d, failed %d.\n", validated, skipped, failed)
		return
	}
	fmt.Fprintf(r.out, "Validated %d file(s), skipped %d. All files are valid.\n", validated, skipped)
}

// singleFileSuccess emits the single-file success line. Summary+ only.
func (r *reporter) singleFileSuccess() {
	if r.json || r.level < verbositySummary {
		return
	}
	fmt.Fprintln(r.out, "Validated 1 file. All files are valid.")
}

// jsonResult writes a single validator result as one NDJSON line.
func (r *reporter) jsonResult(result any) {
	// json.Marshal of validator.Result cannot fail for the values we produce.
	out, _ := json.Marshal(result)
	fmt.Fprintln(r.out, string(out))
}

// jsonError writes a system error as a single-line JSON object.
func (r *reporter) jsonError(err error) {
	// json.Marshal of a map[string]string cannot fail.
	out, _ := json.Marshal(map[string]string{"error": err.Error()})
	fmt.Fprintln(r.out, string(out))
}
