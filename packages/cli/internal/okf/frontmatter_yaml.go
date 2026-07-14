package okf

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const maxFrontmatterYAMLDepth = 100

var yamlErrorLinePattern = regexp.MustCompile(`line ([0-9]+):`)

func parseYAMLFrontmatter(rawLines []string, startLine int) (map[string]string, map[string]struct{}, map[string]any, []frontmatterWarning, error) {
	values := map[string]string{}
	keys := map[string]struct{}{}
	data := map[string]any{}
	var warnings []frontmatterWarning

	block := strings.Join(rawLines, "\n")
	if strings.TrimSpace(block) == "" {
		return values, keys, data, warnings, nil
	}

	var document yaml.Node
	if err := yaml.Unmarshal([]byte(block), &document); err != nil {
		return values, keys, data, warnings, frontmatterYAMLError(err, startLine)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		line := startLine
		if len(document.Content) > 0 && document.Content[0].Line > 0 {
			line = startLine + document.Content[0].Line - 1
		}
		return values, keys, data, warnings, frontmatterParseError{
			line:    line,
			message: "frontmatter must be a YAML mapping",
		}
	}

	root := document.Content[0]
	for index := 0; index < len(root.Content); index += 2 {
		keyNode := root.Content[index]
		valueNode := root.Content[index+1]
		key, err := frontmatterYAMLKey(keyNode, startLine)
		if err != nil {
			return values, keys, data, warnings, err
		}
		if _, exists := keys[key]; exists {
			warnings = append(warnings, frontmatterWarning{
				line:    startLine + keyNode.Line - 1,
				message: fmt.Sprintf("frontmatter key %q is repeated; later value wins", key),
			})
		}

		decoded, err := decodeFrontmatterYAMLNode(valueNode, startLine, 0, map[*yaml.Node]bool{}, &warnings)
		if err != nil {
			return values, keys, data, warnings, err
		}
		keys[key] = struct{}{}
		data[key] = decoded
		values[key] = frontmatterCompatibilityValue(decoded)
	}

	return values, keys, data, warnings, nil
}

func frontmatterYAMLKey(node *yaml.Node, startLine int) (string, error) {
	if node.Kind != yaml.ScalarNode || node.ShortTag() != "!!str" {
		return "", frontmatterParseError{
			line:    startLine + node.Line - 1,
			message: "frontmatter mapping keys must be strings",
		}
	}
	if strings.TrimSpace(node.Value) == "" {
		return "", frontmatterParseError{
			line:    startLine + node.Line - 1,
			message: "frontmatter key is empty",
		}
	}
	return node.Value, nil
}

func decodeFrontmatterYAMLNode(node *yaml.Node, startLine int, depth int, active map[*yaml.Node]bool, warnings *[]frontmatterWarning) (any, error) {
	if depth > maxFrontmatterYAMLDepth {
		return nil, frontmatterParseError{
			line:    startLine + node.Line - 1,
			message: fmt.Sprintf("frontmatter YAML nesting exceeds %d levels", maxFrontmatterYAMLDepth),
		}
	}
	if active[node] {
		return nil, frontmatterParseError{
			line:    startLine + node.Line - 1,
			message: "frontmatter YAML aliases must not form a cycle",
		}
	}
	active[node] = true
	defer delete(active, node)

	switch node.Kind {
	case yaml.ScalarNode:
		if node.ShortTag() == "!!timestamp" {
			return node.Value, nil
		}
		var value any
		if err := node.Decode(&value); err != nil {
			return nil, frontmatterYAMLError(err, startLine)
		}
		return value, nil
	case yaml.SequenceNode:
		values := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			value, err := decodeFrontmatterYAMLNode(child, startLine, depth+1, active, warnings)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	case yaml.MappingNode:
		values := make(map[string]any, len(node.Content)/2)
		seen := make(map[string]struct{}, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			keyNode := node.Content[index]
			key, err := frontmatterYAMLKey(keyNode, startLine)
			if err != nil {
				return nil, err
			}
			if _, exists := seen[key]; exists {
				*warnings = append(*warnings, frontmatterWarning{
					line:    startLine + keyNode.Line - 1,
					message: fmt.Sprintf("frontmatter key %q is repeated; later value wins", key),
				})
			}
			value, err := decodeFrontmatterYAMLNode(node.Content[index+1], startLine, depth+1, active, warnings)
			if err != nil {
				return nil, err
			}
			seen[key] = struct{}{}
			values[key] = value
		}
		return values, nil
	case yaml.AliasNode:
		if node.Alias == nil {
			return nil, frontmatterParseError{
				line:    startLine + node.Line - 1,
				message: "frontmatter YAML alias has no target",
			}
		}
		return decodeFrontmatterYAMLNode(node.Alias, startLine, depth+1, active, warnings)
	default:
		return nil, frontmatterParseError{
			line:    startLine + node.Line - 1,
			message: "frontmatter contains an unsupported YAML node",
		}
	}
}

func frontmatterCompatibilityValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			switch item.(type) {
			case map[string]any, []any:
				return ""
			}
			parts = append(parts, fmt.Sprint(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func frontmatterYAMLError(err error, startLine int) error {
	line := startLine
	if matches := yamlErrorLinePattern.FindStringSubmatch(err.Error()); len(matches) == 2 {
		if relative, parseErr := strconv.Atoi(matches[1]); parseErr == nil && relative > 0 {
			line = startLine + relative - 1
		}
	}
	return frontmatterParseError{
		line:    line,
		message: "frontmatter YAML is invalid: " + err.Error(),
	}
}
