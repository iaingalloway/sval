package validator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/qri-io/jsonschema"

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

func ValidatePath(ctx context.Context, filePath string, schemaPath string) error {
	schema, err := loadSchema(schemaPath)
	if err != nil {
		return fmt.Errorf("load schema: %v", err)
	}
	return ValidateFile(ctx, filePath, schema)
}

// ValidatePathResult validates filePath against the schema at schemaPath and
// returns a structured result. System errors (schema load failure, file read
// failure) are returned as a non-nil error with a nil result.
func ValidatePathResult(ctx context.Context, filePath string, schemaPath string) (*ValidationResult, error) {
	schema, err := loadSchema(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("load schema: %v", err)
	}
	return validateFileResult(ctx, filePath, schema)
}

func ValidateFile(ctx context.Context, path string, schema *jsonschema.Schema) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}

	result, err := validateFileResult(ctx, path, schema)
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

func validateFileResult(ctx context.Context, path string, schema *jsonschema.Schema) (*ValidationResult, error) {
	docs, err := loadDocumentsForValidation(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}

	var allErrors []ValidationError
	for idx, doc := range docs {
		if doc == nil {
			doc = &loader.Document{}
		}

		jsonBytes, err := json.Marshal(doc.Data)
		if err != nil {
			return nil, fmt.Errorf("%s (doc %d): cannot marshal data: %v", path, idx+1, err)
		}

		validationErrs, err := schema.ValidateBytes(ctx, jsonBytes)
		if err != nil {
			return nil, fmt.Errorf("%s (doc %d): schema validation failed: %v", path, idx+1, err)
		}

		allErrors = append(allErrors, buildValidationErrors(doc, validationErrs)...)
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

func loadSchema(path string) (*jsonschema.Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	schema := &jsonschema.Schema{}
	if err := json.Unmarshal(data, schema); err != nil {
		return nil, fmt.Errorf("parse schema: %v", err)
	}
	return schema, nil
}

func buildValidationErrors(doc *loader.Document, errs []jsonschema.KeyError) []ValidationError {
	result := make([]ValidationError, 0, len(errs))
	for _, ve := range errs {
		line, column := pointerPosition(doc, ve.PropertyPath)
		result = append(result, ValidationError{
			Field:   ve.PropertyPath,
			Line:    line,
			Column:  column,
			Message: ve.Error(),
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
