package agents

import (
	"fmt"
	"sort"
)

type jobFieldKind int

const (
	jobStringField jobFieldKind = iota
	jobBoolField
	jobStringListField
	jobObjectField
	jobUnsupportedField
)

type jobFieldSchema struct {
	kind   jobFieldKind
	fields map[string]jobFieldSchema
}

var agentJobFrontmatterSchema = map[string]jobFieldSchema{
	"id":      {kind: jobStringField},
	"enabled": {kind: jobBoolField},
	"schedule": {
		kind: jobObjectField,
		fields: map[string]jobFieldSchema{
			"cron":     {kind: jobStringField},
			"every":    {kind: jobStringField},
			"timezone": {kind: jobStringField},
		},
	},
	"agent": {
		kind: jobObjectField,
		fields: map[string]jobFieldSchema{
			"command":           {kind: jobStringField},
			"args":              {kind: jobStringListField},
			"timeout":           {kind: jobStringField},
			"completion_signal": {kind: jobStringField},
		},
	},
	"workspace": {
		kind: jobObjectField,
		fields: map[string]jobFieldSchema{
			"repo":         {kind: jobStringField},
			"base":         {kind: jobStringField},
			"strategy":     {kind: jobStringField},
			"branch":       {kind: jobStringField},
			"dirty_policy": {kind: jobStringField},
		},
	},
	"sandbox": {
		kind: jobObjectField,
		fields: map[string]jobFieldSchema{
			"type":    {kind: jobStringField},
			"image":   {kind: jobStringField},
			"network": {kind: jobStringField},
			"env":     {kind: jobStringListField},
		},
	},
	"verify": {
		kind: jobObjectField,
		fields: map[string]jobFieldSchema{
			"commands": {kind: jobStringListField},
			"timeout":  {kind: jobStringField},
		},
	},
	"output": {
		kind: jobObjectField,
		fields: map[string]jobFieldSchema{
			"commit":         {kind: jobBoolField},
			"commit_message": {kind: jobStringField},
			"pr":             {kind: jobBoolField},
		},
	},
	"concurrency": {kind: jobUnsupportedField},
}

func validateJobFrontmatterShape(data map[string]any) error {
	var issues []ValidationIssue
	validateJobObject("", data, agentJobFrontmatterSchema, &issues)
	if len(issues) == 0 {
		return nil
	}
	return ValidationError{Issues: issues}
}

func validateJobObject(prefix string, data map[string]any, schema map[string]jobFieldSchema, issues *[]ValidationIssue) {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		field := key
		if prefix != "" {
			field = prefix + "." + key
		}
		rule, supported := schema[key]
		if !supported {
			*issues = append(*issues, ValidationIssue{Field: field, Message: "is not a supported agent job field"})
			continue
		}
		validateJobField(field, data[key], rule, issues)
	}
}

func validateJobField(field string, value any, schema jobFieldSchema, issues *[]ValidationIssue) {
	switch schema.kind {
	case jobStringField:
		if _, ok := value.(string); !ok {
			addJobTypeIssue(field, "a string", value, issues)
		}
	case jobBoolField:
		if _, ok := value.(bool); !ok {
			addJobTypeIssue(field, "a boolean", value, issues)
		}
	case jobStringListField:
		items, ok := value.([]any)
		if !ok {
			addJobTypeIssue(field, "a list of strings", value, issues)
			return
		}
		for index, item := range items {
			if _, ok := item.(string); !ok {
				addJobTypeIssue(fmt.Sprintf("%s[%d]", field, index), "a string", item, issues)
			}
		}
	case jobObjectField:
		object, ok := value.(map[string]any)
		if !ok {
			addJobTypeIssue(field, "a mapping", value, issues)
			return
		}
		validateJobObject(field, object, schema.fields, issues)
	case jobUnsupportedField:
		*issues = append(*issues, ValidationIssue{Field: field, Message: "is reserved and not enforced by the local runner"})
	}
}

func addJobTypeIssue(field string, expected string, value any, issues *[]ValidationIssue) {
	*issues = append(*issues, ValidationIssue{
		Field:   field,
		Message: fmt.Sprintf("must be %s, got %s", expected, jobValueType(value)),
	})
}

func jobValueType(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case int, int64, uint64, float64:
		return "number"
	case []any:
		return "list"
	case map[string]any:
		return "mapping"
	default:
		return fmt.Sprintf("%T", value)
	}
}
