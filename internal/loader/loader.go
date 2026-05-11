package loader

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type Document struct {
	Data      any
	Positions map[string]Position
}

type Position struct {
	Line   int
	Column int
}

var pointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

func (d *Document) PointerPosition(ptr string) (Position, bool) {
	if d == nil || len(d.Positions) == 0 {
		return Position{}, false
	}
	if ptr == "" {
		ptr = "/"
	}
	pos, ok := d.Positions[ptr]
	return pos, ok
}

func LoadFrontmatter(path string) (*Document, error) {
	docs, err := LoadFrontmatterDocuments(path)
	if err != nil {
		return nil, err
	}
	if len(docs) == 1 {
		if docs[0] == nil {
			return &Document{}, nil
		}
		return docs[0], nil
	}
	return &Document{Data: documentsDataSlice(docs)}, nil
}

func LoadFrontmatterDocuments(path string) ([]*Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	sections, err := extractFrontmatterSections(content)
	if err != nil {
		if errors.Is(err, errFrontmatterNotFound) {
			return []*Document{{}}, nil
		}
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}
	if len(sections) == 0 {
		return []*Document{{}}, nil
	}

	allDocs := make([]*Document, 0, len(sections))

	docIndex := 1
	for _, section := range sections {
		if len(bytes.TrimSpace(section.Data)) == 0 {
			continue
		}
		docs, err := parseFrontmatterSectionWithIndex(section, docIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
		}
		allDocs = append(allDocs, docs...)
		docIndex += len(docs)
	}

	if len(allDocs) == 0 {
		return []*Document{{}}, nil
	}
	return allDocs, nil
}

func LoadYAML(path string) (*Document, error) {
	docs, err := LoadYAMLDocuments(path)
	if err != nil {
		return nil, err
	}
	if len(docs) == 1 {
		if docs[0] == nil {
			return &Document{}, nil
		}
		return docs[0], nil
	}
	return &Document{Data: documentsDataSlice(docs)}, nil
}

func LoadYAMLDocuments(path string) ([]*Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return []*Document{{}}, nil
	}

	docs, err := parseYAMLDocuments(content, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}
	return docs, nil
}

func LoadJSON(path string) (*Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return &Document{}, nil
	}

	doc, err := parseJSONDocument(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse json: %w", err)
	}
	return doc, nil
}

func LoadTOML(path string) (*Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return &Document{}, nil
	}

	doc, err := parseTOMLDocument(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse toml: %w", err)
	}
	return doc, nil
}

var errFrontmatterNotFound = errors.New("frontmatter not found")

type frontmatterSection struct {
	Data      []byte
	StartLine int
	Marker    string // close delimiter to match
	Format    string // "yaml", "toml", or "json"
}

func extractFrontmatterSections(content []byte) ([]frontmatterSection, error) {
	reader := bufio.NewReader(bytes.NewReader(content))
	lineNum := 0
	started := false
	sections := make([]frontmatterSection, 0, 1)

	for {
		lineBytes, err := reader.ReadBytes('\n')
		if len(lineBytes) == 0 && err == io.EOF {
			break
		}

		lineNum++
		trimmed := strings.TrimSpace(string(bytes.TrimRight(lineBytes, "\r\n")))
		if lineNum == 1 {
			trimmed = strings.TrimPrefix(trimmed, "\ufeff")
		}

		if !started {
			if trimmed == "" {
				if err == io.EOF {
					break
				}
				continue
			}
			marker, format, ok := detectFrontmatterStart(trimmed)
			if !ok {
				return nil, errFrontmatterNotFound
			}
			started = true
			section, linesRead, readErr := readFrontmatterSection(reader, marker, lineNum+1)
			if readErr != nil {
				return nil, readErr
			}
			section.Marker = marker
			section.Format = format
			sections = append(sections, section)
			lineNum += linesRead
			if err == io.EOF {
				break
			}
			continue
		}

		if trimmed == "" {
			if err == io.EOF {
				break
			}
			continue
		}

		marker, format, ok := detectFrontmatterStart(trimmed)
		if !ok {
			break
		}
		section, linesRead, readErr := readFrontmatterSection(reader, marker, lineNum+1)
		if readErr != nil {
			return nil, readErr
		}
		section.Marker = marker
		section.Format = format
		sections = append(sections, section)
		lineNum += linesRead
		if err == io.EOF {
			break
		}
	}

	if len(sections) == 0 {
		return nil, errFrontmatterNotFound
	}
	return sections, nil
}

func readFrontmatterSection(reader *bufio.Reader, closeMarker string, startLine int) (frontmatterSection, int, error) {
	var b strings.Builder
	linesRead := 0
	for {
		lineBytes, err := reader.ReadBytes('\n')
		if len(lineBytes) == 0 && err == io.EOF {
			break
		}

		linesRead++
		trimmed := strings.TrimSpace(string(bytes.TrimRight(lineBytes, "\r\n")))
		if matchesCloseMarker(trimmed, closeMarker) {
			return frontmatterSection{Data: []byte(b.String()), StartLine: startLine}, linesRead, nil
		}

		b.Write(lineBytes)

		if err == io.EOF {
			break
		}
		if err != nil {
			return frontmatterSection{}, linesRead, err
		}
	}
	return frontmatterSection{}, linesRead, fmt.Errorf("unterminated frontmatter")
}

func detectFrontmatterStart(trimmedLine string) (marker string, format string, ok bool) {
	lower := strings.ToLower(strings.TrimSpace(trimmedLine))
	if lower == "+++" {
		return "+++", "toml", true
	}
	if strings.HasPrefix(lower, "---") {
		tag := strings.TrimPrefix(lower, "---")
		switch tag {
		case "toml":
			return "---", "toml", true
		case "json":
			return "---", "json", true
		default:
			// "---", "---yaml", or any unrecognised tag → YAML
			return "---", "yaml", true
		}
	}
	return "", "", false
}

func matchesCloseMarker(trimmedLine string, closeMarker string) bool {
	trimmed := strings.TrimSpace(trimmedLine)
	if closeMarker == "+++" {
		return trimmed == "+++"
	}
	return strings.ToLower(trimmed) == closeMarker
}

func parseFrontmatterSectionWithIndex(section frontmatterSection, docIndex int) ([]*Document, error) {
	if section.Format == "toml" {
		doc, err := parseTOMLDocument(section.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse toml (doc %d): %w", docIndex, err)
		}
		return []*Document{doc}, nil
	}
	// yaml and json (JSON is valid YAML; both get position tracking)
	docs, err := parseYAMLDocumentsWithIndex(section.Data, section.StartLine, docIndex)
	if err != nil {
		return nil, err
	}
	return docs, nil
}

func parseYAMLDocumentsWithIndex(content []byte, startLine int, docIndex int) ([]*Document, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	docs := make([]*Document, 0, 1)
	idx := docIndex
	for {
		var root yaml.Node
		if err := decoder.Decode(&root); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to parse yaml (doc %d): %w", idx, err)
		}

		var data any
		if err := root.Decode(&data); err != nil {
			return nil, fmt.Errorf("failed to parse yaml (doc %d): %w", idx, err)
		}

		doc := &Document{Data: data}
		if len(root.Content) > 0 {
			doc.Positions = make(map[string]Position)
			collectPositions(root.Content[0], "/", startLine-1, doc.Positions)
		}
		docs = append(docs, doc)
		idx++
	}
	if len(docs) == 0 {
		return []*Document{{}}, nil
	}
	return docs, nil
}

func parseYAMLDocuments(content []byte, startLine int) ([]*Document, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	docs := make([]*Document, 0, 1)
	docIdx := 1
	for {
		var root yaml.Node
		if err := decoder.Decode(&root); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to parse yaml (doc %d): %w", docIdx, err)
		}

		var data any
		if err := root.Decode(&data); err != nil {
			return nil, fmt.Errorf("failed to parse yaml (doc %d): %w", docIdx, err)
		}

		doc := &Document{Data: data}
		if len(root.Content) > 0 {
			doc.Positions = make(map[string]Position)
			collectPositions(root.Content[0], "/", startLine-1, doc.Positions)
		}
		docs = append(docs, doc)
		docIdx++
	}
	if len(docs) == 0 {
		return []*Document{{}}, nil
	}
	return docs, nil
}

func parseJSONDocument(content []byte) (*Document, error) {
	var data any
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}
	return &Document{Data: data}, nil
}

func parseTOMLDocument(content []byte) (*Document, error) {
	var data any
	if err := toml.Unmarshal(content, &data); err != nil {
		return nil, err
	}
	return &Document{Data: data}, nil
}

func documentsDataSlice(docs []*Document) []any {
	items := make([]any, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			items = append(items, nil)
			continue
		}
		items = append(items, doc.Data)
	}
	return items
}

func collectPositions(node *yaml.Node, ptr string, offset int, positions map[string]Position) {
	if node == nil {
		return
	}
	positions[ptr] = Position{Line: offset + node.Line, Column: node.Column}

	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			token := escapePointerToken(keyNode.Value)
			childPtr := joinPointer(ptr, token)
			collectPositions(valNode, childPtr, offset, positions)
		}
	case yaml.SequenceNode:
		for idx, child := range node.Content {
			childPtr := joinPointer(ptr, strconv.Itoa(idx))
			collectPositions(child, childPtr, offset, positions)
		}
	}
}

func escapePointerToken(token string) string {
	return pointerEscaper.Replace(token)
}

func joinPointer(base, token string) string {
	if base == "/" {
		return "/" + token
	}
	return base + "/" + token
}
