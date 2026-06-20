package okf

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const ConfigFile = "openknowledge.toml"

type Config struct {
	Bundle         BundleMetadata
	PublishExclude map[string]struct{}
}

func ReadConfig(root string) (Config, error) {
	content, err := os.ReadFile(filepath.Join(root, ConfigFile))
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	return parseConfig(string(content))
}

func parseConfig(content string) (Config, error) {
	config := Config{PublishExclude: map[string]struct{}{}}
	var entries []BundleEntry
	scanner := bufio.NewScanner(strings.NewReader(content))
	section := ""
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(stripConfigComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return Config{}, fmt.Errorf("%s:%d expected key = value", ConfigFile, lineNumber)
		}
		key = strings.TrimSpace(key)
		rawValue = strings.TrimSpace(rawValue)

		switch section {
		case "bundle":
			if err := parseBundleConfigValue(&config.Bundle, key, rawValue, lineNumber); err != nil {
				return Config{}, err
			}
		case "bundle.entries":
			value, err := parseConfigStringValue(rawValue)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d %w", ConfigFile, lineNumber, err)
			}
			entries = append(entries, BundleEntry{Name: key, Path: value})
		case "publish":
			if key != "exclude" {
				continue
			}
			values, err := parseConfigStringArray(rawValue)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d %w", ConfigFile, lineNumber, err)
			}
			for _, value := range values {
				normalized, err := normalizeConfigPath(value)
				if err != nil {
					return Config{}, fmt.Errorf("%s:%d publish.exclude %q: %w", ConfigFile, lineNumber, value, err)
				}
				config.PublishExclude[normalized] = struct{}{}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Config{}, err
	}

	config.Bundle.Entries = entries
	metadata, err := normalizeBundleMetadata(config.Bundle)
	if err != nil {
		return Config{}, err
	}
	config.Bundle = metadata
	return config, nil
}

func parseBundleConfigValue(metadata *BundleMetadata, key string, rawValue string, lineNumber int) error {
	switch key {
	case "name":
		value, err := parseConfigStringValue(rawValue)
		if err != nil {
			return fmt.Errorf("%s:%d %w", ConfigFile, lineNumber, err)
		}
		metadata.Name = value
	case "title":
		value, err := parseConfigStringValue(rawValue)
		if err != nil {
			return fmt.Errorf("%s:%d %w", ConfigFile, lineNumber, err)
		}
		metadata.Title = value
	case "purpose":
		value, err := parseConfigStringValue(rawValue)
		if err != nil {
			return fmt.Errorf("%s:%d %w", ConfigFile, lineNumber, err)
		}
		metadata.Purpose = value
	case "tags":
		values, err := parseConfigStringArray(rawValue)
		if err != nil {
			return fmt.Errorf("%s:%d %w", ConfigFile, lineNumber, err)
		}
		metadata.Tags = values
	}
	return nil
}

func parseConfigStringValue(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("expected a quoted string value")
	}
	if strings.HasPrefix(value, `"`) {
		parsed, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("invalid quoted string")
		}
		return parsed, nil
	}
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
		return value[1 : len(value)-1], nil
	}
	return "", fmt.Errorf("expected a quoted string value")
}

func parseConfigStringArray(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, fmt.Errorf("expected an array of quoted strings")
	}
	body := strings.TrimSpace(value[1 : len(value)-1])
	if body == "" {
		return nil, nil
	}

	var values []string
	for len(body) > 0 {
		body = strings.TrimSpace(body)
		item, rest, err := parseConfigArrayItem(body)
		if err != nil {
			return nil, err
		}
		values = append(values, item)
		body = strings.TrimSpace(rest)
		if body == "" {
			break
		}
		if !strings.HasPrefix(body, ",") {
			return nil, fmt.Errorf("expected comma between array values")
		}
		body = strings.TrimSpace(strings.TrimPrefix(body, ","))
	}
	return values, nil
}

func parseConfigArrayItem(value string) (string, string, error) {
	if strings.HasPrefix(value, `"`) {
		for index := 1; index < len(value); index++ {
			if value[index] != '"' {
				continue
			}
			segment := value[:index+1]
			if parsed, err := strconv.Unquote(segment); err == nil {
				return parsed, value[index+1:], nil
			}
		}
		return "", "", fmt.Errorf("invalid quoted string")
	}
	if strings.HasPrefix(value, "'") {
		end := strings.Index(value[1:], "'")
		if end < 0 {
			return "", "", fmt.Errorf("invalid quoted string")
		}
		end += 1
		return value[1:end], value[end+1:], nil
	}
	return "", "", fmt.Errorf("expected a quoted string value")
}

func stripConfigComment(line string) string {
	quote := rune(0)
	escaped := false
	for index, char := range line {
		if escaped {
			escaped = false
			continue
		}
		if quote == '"' && char == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '"' || char == '\'' {
			quote = char
			continue
		}
		if char == '#' {
			return line[:index]
		}
	}
	return line
}

func normalizeConfigPath(value string) (string, error) {
	clean := strings.TrimSpace(value)
	if clean == "" || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("must be a relative bundle path")
	}
	clean = path.Clean(strings.ReplaceAll(clean, "\\", "/"))
	clean = strings.TrimPrefix(clean, "./")
	if clean == "." || clean == "" || hasParentPathSegment(clean) {
		return "", fmt.Errorf("must stay inside the bundle")
	}
	return clean, nil
}

func hasParentPathSegment(value string) bool {
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}
