package tui

import (
	"fmt"
	"sort"
	"strings"
)

func parseTags(input string) ([]string, error) {
	if strings.TrimSpace(input) == "" {
		return nil, nil
	}
	raw := strings.Split(input, ",")
	tags := make([]string, 0, len(raw))
	for _, tag := range raw {
		t := strings.TrimSpace(tag)
		if t == "" {
			return nil, fmt.Errorf("tags cannot contain empty values")
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func parseAttributes(input string) (map[string]string, error) {
	result := map[string]string{}
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid attribute %q (expected key=value)", line)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("attribute key missing in %q", line)
		}
		if value == "" {
			return nil, fmt.Errorf("attribute %q has empty value", key)
		}
		result[key] = value
	}
	return result, nil
}

func formatAttributeLines(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, attrs[k]))
	}
	return strings.Join(lines, "\n")
}
