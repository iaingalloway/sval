package validator

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"sval/internal/loader"
)

type FileType int

const (
	FileTypeUnknown FileType = iota
	FileTypeMarkdown
	FileTypeYAML
	FileTypeJSON
	FileTypeTOML
)

// ValidationError holds a single schema validation error with source position.
type ValidationError struct {
	Field   string `json:"field,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Message string `json:"message"`
}

// ValidationResult holds the structured outcome of validating a file.
type ValidationResult struct {
	File   string            `json:"file"`
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

func ValidatePath(filePath string, schemaPath string) error {
	schema, err := loadSchema(schemaPath)
	if err != nil {
		return fmt.Errorf("load schema: %v", err)
	}
	return ValidateFile(filePath, schema)
}

// ValidatePathResult validates filePath against the schema at schemaPath and
// returns a structured result. System errors (schema load failure, file read
// failure) are returned as a non-nil error with a nil result.
func ValidatePathResult(filePath string, schemaPath string) (*ValidationResult, error) {
	schema, err := loadSchema(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("load schema: %v", err)
	}
	return validateFileResult(filePath, schema)
}

func ValidateFile(path string, schema *jsonschema.Schema) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}

	result, err := validateFileResult(path, schema)
	if err != nil {
		return err
	}

	if !result.Valid {
		var messages []string
		for _, ve := range result.Errors {
			if ve.Column > 0 {
				messages = append(messages, fmt.Sprintf("%s:%d:%d: %s", path, ve.Line, ve.Column, ve.Message))
			} else {
				messages = append(messages, fmt.Sprintf("%s:%d: %s", path, ve.Line, ve.Message))
			}
		}
		return fmt.Errorf("%s", strings.Join(messages, "\n"))
	}
	return nil
}

func validateFileResult(path string, schema *jsonschema.Schema) (*ValidationResult, error) {
	docs, err := loadDocumentsForValidation(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}

	var allErrors []ValidationError
	for _, doc := range docs {
		if doc == nil {
			doc = &loader.Document{}
		}

		errs := buildValidationErrors(doc, schema.Validate(doc.Data))
		allErrors = append(allErrors, errs...)
	}

	return &ValidationResult{
		File:   path,
		Valid:  len(allErrors) == 0,
		Errors: allErrors,
	}, nil
}

func loadDocumentsForValidation(path string) ([]*loader.Document, error) {
	switch DetectFileType(path) {
	case FileTypeMarkdown:
		return loader.LoadFrontmatterDocuments(path)
	case FileTypeYAML:
		return loader.LoadYAMLDocuments(path)
	case FileTypeJSON:
		doc, err := loader.LoadJSON(path)
		if err != nil {
			return nil, err
		}
		return []*loader.Document{doc}, nil
	case FileTypeTOML:
		doc, err := loader.LoadTOML(path)
		if err != nil {
			return nil, err
		}
		return []*loader.Document{doc}, nil
	default:
		return nil, fmt.Errorf("unsupported file type for %s", path)
	}
}

func DetectFileType(path string) FileType {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md":
		return FileTypeMarkdown
	case ".yaml", ".yml":
		return FileTypeYAML
	case ".json":
		return FileTypeJSON
	case ".toml":
		return FileTypeTOML
	default:
		return FileTypeUnknown
	}
}

type httpURLLoader http.Client

func (l *httpURLLoader) Load(url string) (any, error) {
	client := (*http.Client)(l)
	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%s returned status code %d", url, resp.StatusCode)
	}
	defer resp.Body.Close()
	return jsonschema.UnmarshalJSON(resp.Body)
}

func newCompiler() *jsonschema.Compiler {
	httpLoader := (*httpURLLoader)(&http.Client{Timeout: 15 * time.Second})
	c := jsonschema.NewCompiler()
	c.UseLoader(jsonschema.SchemeURLLoader{
		"file":  jsonschema.FileLoader{},
		"http":  httpLoader,
		"https": httpLoader,
	})
	return c
}

func loadSchema(path string) (*jsonschema.Schema, error) {
	c := newCompiler()
	loc := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		loc = "file://" + abs
	}
	schema, err := c.Compile(loc)
	if err != nil {
		// Surface "no such file" errors directly; wrap compile errors with context.
		return nil, fmt.Errorf("parse schema: %v", err)
	}
	return schema, nil
}

func buildValidationErrors(doc *loader.Document, err error) []ValidationError {
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []ValidationError{{Message: err.Error()}}
	}

	basic := ve.BasicOutput()
	var result []ValidationError
	for _, unit := range basic.Errors {
		if unit.Error == nil {
			continue
		}
		ptr := unit.InstanceLocation
		if ptr == "" {
			ptr = "/"
		}
		line, column := pointerPosition(doc, ptr)
		result = append(result, ValidationError{
			Field:   ptr,
			Line:    line,
			Column:  column,
			Message: unit.Error.String(),
		})
	}
	return result
}

func pointerPosition(doc *loader.Document, pointer string) (int, int) {
	if doc == nil {
		return 0, 0
	}
	pos, ok := doc.PointerPosition(pointer)
	if !ok {
		return 0, 0
	}
	return pos.Line, pos.Column
}
